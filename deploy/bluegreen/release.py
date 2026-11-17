#!/usr/bin/env python3
"""Blue-green releases using the existing Compose configuration and Nginx."""

import argparse
import copy
import datetime
import fcntl
import json
import os
from pathlib import Path
import pwd
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.request


IMAGE = "registry.cn-guangzhou.aliyuncs.com/quickpay-sit/sub2api:last"
UPSTREAM = "oapi_sub2api"


class ReleaseError(Exception):
    def __init__(self, message, stderr=""):
        super().__init__(message)
        self.stderr = stderr


def run(args, input_text=None, environment=None):
    result = subprocess.run(args, input=input_text, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, universal_newlines=True, env=environment)
    if result.returncode:
        # Compose's diagnostic output can contain expanded environment secrets.
        raise ReleaseError("{} failed (exit {}); inspect it on the server".format(
            " ".join(args[:2]), result.returncode), result.stderr)
    return result.stdout


def write_atomic(path, text, mode=0o600):
    path = Path(path)
    temporary = path.with_name(path.name + ".tmp")
    fd = os.open(str(temporary), os.O_WRONLY | os.O_CREAT | os.O_TRUNC, mode)
    with os.fdopen(fd, "w") as stream:
        stream.write(text)
        stream.flush()
        os.fsync(stream.fileno())
    os.chmod(str(temporary), mode)
    os.replace(str(temporary), str(path))


def save_json(path, value):
    write_atomic(path, json.dumps(value, indent=2) + "\n")


def inspect(name):
    return json.loads(run(["docker", "inspect", name]))[0]


def upstream_text(port):
    return ("# Managed by deploy/bluegreen/release.py\n"
            "upstream " + UPSTREAM + " {\n"
            "    server 127.0.0.1:" + str(port) + ";\n"
            "    keepalive 32;\n"
            "}\n")


def request_json(url, expected):
    try:
        response = urllib.request.urlopen(url, timeout=5)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        if response.code != expected:
            raise ReleaseError("Unexpected HTTP status at " + url)
        if "application/json" not in response.headers.get("Content-Type", ""):
            raise ReleaseError("Expected JSON, not an SPA page, at " + url)
        value = json.loads(response.read(1024 * 1024))
        if not isinstance(value, dict):
            raise ReleaseError("Expected a JSON object at " + url)
        return value


def smoke(base):
    health = request_json(base + "/health", 200)
    if health.get("status") != "ok":
        raise ReleaseError("Application health is not ok")
    denied = request_json(base + "/api/v1/admin/usage/image-holds?page_size=1", 401)
    if denied.get("code") != "UNAUTHORIZED":
        raise ReleaseError("Admin route did not return the expected auth error")


def compose_document(source, service, container, port, image):
    app = copy.deepcopy(source["services"][service])
    for key in ("build", "depends_on", "profiles", "develop"):
        app.pop(key, None)
    app.update(image=image, pull_policy="never", container_name=container,
               ports=[{"target": 8080, "published": str(port),
                       "host_ip": "127.0.0.1", "protocol": "tcp"}],
               stop_grace_period="60s")
    # Both slots mount the same data. SELinux's private :Z relabel is unsuitable.
    for volume in app.get("volumes", []):
        if volume.get("type") == "bind" and volume.get("bind", {}).get("selinux") == "Z":
            volume["bind"]["selinux"] = "z"
    result = {"name": source["name"], "services": {container: app}}
    for kind in ("networks", "volumes"):
        if kind in source:
            result[kind] = {
                key: {"external": True, "name": value.get("name", key)}
                for key, value in source[kind].items()
            }
    return result


def check_identity(source_app, active):
    desired = source_app.get("environment", {})
    actual = dict(item.split("=", 1) for item in active["Config"]["Env"] if "=" in item)
    keys = ("JWT_SECRET", "TOTP_ENCRYPTION_KEY", "DATABASE_HOST", "DATABASE_PORT",
            "DATABASE_USER", "DATABASE_PASSWORD", "DATABASE_DBNAME",
            "REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD", "REDIS_DB")
    for key in keys:
        if str(desired.get(key) or "") != actual.get(key, ""):
            raise ReleaseError(key + " differs from the active container; resolve before release")
    # The setup persists generated keys in the shared /app/data when env is empty.
    destinations = {item["Destination"]: item["Source"] for item in active["Mounts"]}
    for volume in source_app.get("volumes", []):
        if volume.get("target") == "/app/data":
            if volume.get("type") != "bind" or destinations.get("/app/data") != volume["source"]:
                raise ReleaseError("Both slots must share the existing /app/data bind mount")
            break
    else:
        raise ReleaseError("Existing /app/data bind mount is required")


def check_background_jobs(container):
    sql = ("SELECT (SELECT count(*) FROM scheduled_test_plans WHERE enabled),"
           "(SELECT count(*) FROM channel_monitors WHERE enabled);")
    command = ('export PGPASSWORD="$DATABASE_PASSWORD"; exec psql -X -v ON_ERROR_STOP=1 '
               '-h "$DATABASE_HOST" -p "${DATABASE_PORT:-5432}" '
               '-U "$DATABASE_USER" -d "$DATABASE_DBNAME" -At')
    counts = run(["docker", "exec", "-i", container, "sh", "-c", command], sql).strip()
    if counts != "0|0":
        raise ReleaseError("Pause scheduled model tests and channel monitors before overlap; "
                           "these runners do not yet have cross-instance exclusion")


class Deployment:
    def __init__(self, directory):
        self.directory = Path(directory).resolve()
        self.root = self.directory / ".bluegreen"
        self.root.mkdir(mode=0o700, exist_ok=True)
        self.state_path = self.root / "state.json"
        self.config_path = self.root / "config.json"

    def lock(self):
        self.lock_stream = open(str(self.root / "release.lock"), "a")
        try:
            fcntl.flock(self.lock_stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise ReleaseError("Another blue-green operation is running")

    def load(self):
        self.config = json.loads(self.config_path.read_text())
        self.state = json.loads(self.state_path.read_text())

    def source(self):
        return json.loads(run(["docker", "compose", "--project-directory",
                               self.config["source_directory"],
                               "-f", self.config["compose_file"], "config", "--format", "json"]))

    def nginx(self, *arguments):
        container = self.config.get("nginx_container")
        prefix = ["docker", "exec", container] if container else []
        return run(prefix + ["nginx"] + list(arguments))

    def workers(self):
        container = self.config.get("nginx_container")
        command = (["docker", "top", container, "-eo", "pid,args"]
                   if container else ["ps", "-eo", "pid,args"])
        identities = []
        for line in run(command).splitlines():
            if "nginx: worker process" in line:
                pid = int(line.split()[0])
                try:
                    stat = Path("/proc/{}/stat".format(pid)).read_text()
                    identities.append([pid, stat.rsplit(")", 1)[1].split()[19]])
                except FileNotFoundError:
                    pass
        if not identities:
            raise ReleaseError("No Nginx worker processes found")
        return identities

    @staticmethod
    def workers_alive(identities):
        for pid, started in identities:
            try:
                stat = Path("/proc/{}/stat".format(pid)).read_text()
                if stat.rsplit(")", 1)[1].split()[19] == started:
                    return True
            except FileNotFoundError:
                pass
        return False

    def wait_healthy(self, slot, expected_image, timeout=180):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            details = inspect(self.config[slot + "_container"])
            status = details["State"]
            if details["Image"] != expected_image or not status.get("Running"):
                raise ReleaseError("Candidate container is not running the expected image")
            if status.get("Health", {}).get("Status") == "healthy":
                try:
                    smoke("http://127.0.0.1:" + str(self.config[slot + "_port"]))
                    return
                except (OSError, ValueError, ReleaseError):
                    pass
            time.sleep(2)
        raise ReleaseError("Candidate health checks timed out; current route was preserved")

    def wait_reloaded(self, previous):
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            if any(worker not in previous for worker in self.workers()):
                return
            time.sleep(0.25)
        raise ReleaseError("Nginx did not start a new worker generation")

    def image_tasks_processing(self):
        source = self.source()
        app = source["services"][self.config["service"]]
        host = app.get("environment", {}).get("REDIS_HOST", "")
        redis_service = source["services"].get(host)
        if not redis_service:
            raise ReleaseError("Cannot check async image tasks: Redis must be a Compose service")
        container = redis_service.get("container_name", source["name"] + "-" + host + "-1")
        database = str(app.get("environment", {}).get("REDIS_DB") or "0")
        prefix = ["docker", "exec", "-e", "REDISCLI_AUTH", container,
                  "redis-cli", "-n", database, "--raw"]
        environment = dict(os.environ)
        environment["REDISCLI_AUTH"] = app.get("environment", {}).get("REDIS_PASSWORD") or ""
        # Use the app's auth, not the Redis container's independent CLI default.
        # Only the variable name is passed in argv.
        def query(arguments):
            nonlocal prefix
            try:
                return run(prefix + arguments, environment=environment)
            except ReleaseError as error:
                if "without any password configured" not in error.stderr:
                    raise
                prefix = ["docker", "exec", container, "env", "-u", "REDISCLI_AUTH",
                          "redis-cli", "-n", database, "--raw"]
                return run(prefix + arguments, environment=environment)

        keys = query(["--scan", "--pattern", "image_task:*"]).splitlines()
        if len(keys) > 1000:
            raise ReleaseError("Too many async image tasks to prove drain; old app retained")
        for key in keys:
            value = query(["GET", key])
            if value.strip() and json.loads(value).get("status") == "processing":
                return True
        return False

    def start_slot(self, slot, image):
        source = self.source()
        name = self.config[slot + "_container"]
        document = compose_document(source, self.config["service"], name,
                                    self.config[slot + "_port"], image)
        path = self.root / (slot + ".compose.json")
        save_json(path, document)
        run(["docker", "compose", "--project-directory", str(self.directory),
             "-f", str(path), "up", "-d", "--no-deps", name])
        self.wait_healthy(slot, image)

    def backup(self):
        stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S-%f")
        target = self.directory / "back" / ("bluegreen-" + stamp)
        target.mkdir(mode=0o700, parents=True)
        for source in (Path(self.config["compose_file"]), self.config_path,
                       self.state_path, Path(self.config["upstream_file"])):
            if source.exists():
                shutil.copy2(str(source), str(target / source.name))
        return target

    def verify_route(self, slot):
        expected = upstream_text(self.config[slot + "_port"])
        if Path(self.config["upstream_file"]).read_text() != expected:
            raise ReleaseError("Nginx upstream file differs from release state")
        self.nginx("-t")
        smoke(self.config["probe_url"])

    def init(self, args):
        if self.config_path.exists():
            raise ReleaseError("Already initialized; use status or deploy")
        self.config = {
            "compose_file": str((self.directory / args.compose_file).resolve()),
            "source_directory": str(Path(args.source_directory or self.directory).resolve()),
            "service": args.service, "blue_container": args.blue_container or args.service,
            "green_container": args.service + "-green",
            "blue_port": args.blue_port, "green_port": args.green_port,
            "upstream_file": str(Path(args.upstream_file).resolve()),
            "nginx_container": args.nginx_container, "probe_url": args.probe_url.rstrip("/"),
        }
        if args.blue_port == args.green_port:
            raise ReleaseError("Blue and green ports must differ")
        self.nginx("-t")
        if Path(self.config["upstream_file"]).read_text() != upstream_text(args.blue_port):
            raise ReleaseError("Install the initial blue upstream file before init")
        source = self.source()
        original = source["services"][args.service].get("container_name", args.service)
        details = inspect(original)
        check_identity(source["services"][args.service], details)
        check_background_jobs(original)
        if self.config["blue_container"] != original:
            self.start_slot("blue", details["Image"])
        else:
            self.wait_healthy("blue", details["Image"])
        smoke(self.config["probe_url"])
        self.state = {"active": "blue", "previous": None, "pending": None,
                      "images": {"blue": details["Image"]}}
        save_json(self.state_path, self.state)
        save_json(self.config_path, self.config)
        print("Initialized blue route; database and Redis were not recreated")

    def switch(self, target):
        old = self.state["active"]
        old_workers = self.workers()
        self.backup()
        # Persist the recovery journal BEFORE mutating the route.
        self.state["switching"] = {"from": old, "to": target, "workers": old_workers}
        save_json(self.state_path, self.state)
        path = Path(self.config["upstream_file"])
        previous = path.read_text()
        try:
            write_atomic(path, upstream_text(self.config[target + "_port"]), 0o644)
            self.nginx("-t")
            self.nginx("-s", "reload")
            self.wait_reloaded(old_workers)
            smoke(self.config["probe_url"])
        except Exception:
            # Do not stop either app: an accepted reload may still be draining.
            write_atomic(path, previous, 0o644)
            self.nginx("-t")
            self.nginx("-s", "reload")
            self.state["switching"]["restore_requested"] = True
            save_json(self.state_path, self.state)
            raise ReleaseError("Switch failed; old route restored. Both apps retained; run recover")
        self.state.update(active=target, previous=old,
                          pending={"slot": old, "workers": old_workers})
        self.state.pop("switching")
        save_json(self.state_path, self.state)
        print("Nginx switched to " + target)

    def finish(self, timeout):
        if self.state.get("switching"):
            raise ReleaseError("Interrupted switch: run recover before any other mutation")
        pending = self.state.get("pending")
        if not pending:
            return
        deadline = time.monotonic() + timeout
        while self.workers_alive(pending["workers"]):
            if time.monotonic() >= deadline:
                print("Old Nginx workers still draining; old app retained. Run finish later.")
                return
            time.sleep(2)
        slot = pending["slot"]
        # A user bypassing Nginx must not have their requests cut off either.
        connections = run(["ss", "-Htn", "state", "established",
                           "( sport = :{} )".format(self.config[slot + "_port"])])
        if connections.strip():
            print("Old app still has direct TCP connections; retained. Run finish later.")
            return
        if self.image_tasks_processing():
            print("Async image tasks still processing; old app retained. Run finish later.")
            return
        self.verify_route(self.state["active"])
        run(["docker", "stop", "--time", "60", self.config[slot + "_container"]])
        self.state["pending"] = None
        save_json(self.state_path, self.state)
        print("Old app stopped after Nginx workers and app connections drained")

    def deploy(self, timeout):
        self.finish(0)
        if self.state.get("pending"):
            raise ReleaseError("Previous release is still draining; idle slot cannot be reused")
        current = self.state["active"]
        self.verify_route(current)
        source = self.source()
        details = inspect(self.config[current + "_container"])
        check_identity(source["services"][self.config["service"]], details)
        check_background_jobs(self.config[current + "_container"])
        self.backup()
        run(["docker", "pull", IMAGE])
        image = inspect(IMAGE)["Id"]
        target = "green" if current == "blue" else "blue"
        self.start_slot(target, image)
        self.state["images"][target] = image
        save_json(self.state_path, self.state)
        self.switch(target)
        self.finish(timeout)

    def rollback(self, timeout):
        if self.state.get("switching"):
            raise ReleaseError("Run recover first")
        target = self.state.get("previous")
        if not target:
            raise ReleaseError("No previous release")
        self.verify_route(self.state["active"])
        check_background_jobs(self.config[self.state["active"] + "_container"])
        image = self.state["images"][target]
        if not inspect(self.config[target + "_container"])["State"]["Running"]:
            run(["docker", "start", self.config[target + "_container"]])
        self.wait_healthy(target, image)
        self.switch(target)
        self.finish(timeout)

    def recover(self):
        journal = self.state.get("switching")
        if not journal:
            print("No interrupted switch")
            return
        old = journal["from"]
        self.wait_healthy(old, self.state["images"][old])
        # Keep every pre-recovery worker's upstream alive until they exit.
        workers = self.workers() + journal["workers"]
        write_atomic(self.config["upstream_file"], upstream_text(self.config[old + "_port"]), 0o644)
        self.nginx("-t")
        self.nginx("-s", "reload")
        self.wait_reloaded(workers)
        smoke(self.config["probe_url"])
        self.state.update(active=old, previous=journal["to"],
                          pending={"slot": journal["to"], "workers": workers})
        self.state.pop("switching")
        save_json(self.state_path, self.state)
        print("Restored old route; run finish to drain the candidate")


def main():
    if "DOCKER_CONFIG" not in os.environ and os.environ.get("SUDO_USER"):
        home = Path(pwd.getpwnam(os.environ["SUDO_USER"]).pw_dir)
        if (home / ".docker" / "config.json").is_file():
            os.environ["DOCKER_CONFIG"] = str(home / ".docker")
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", default=".")
    sub = parser.add_subparsers(dest="action")
    init = sub.add_parser("init")
    init.add_argument("--compose-file", default="docker-compose.yml")
    init.add_argument("--source-directory")
    init.add_argument("--service", default="sub2api-foreign")
    init.add_argument("--blue-container")
    init.add_argument("--blue-port", type=int, default=8081)
    init.add_argument("--green-port", type=int, default=8082)
    init.add_argument("--upstream-file", required=True)
    init.add_argument("--nginx-container")
    init.add_argument("--probe-url", default="https://oapi.yundata.ai")
    for command in ("deploy", "finish", "rollback"):
        command_parser = sub.add_parser(command)
        command_parser.add_argument("--drain-seconds", type=int, default=120)
    sub.add_parser("status")
    sub.add_parser("recover")
    args = parser.parse_args()
    if not args.action:
        parser.error("a command is required")
    deployment = Deployment(args.directory)
    deployment.lock()
    if args.action == "init":
        deployment.init(args)
        return
    deployment.load()
    if args.action == "status":
        print(json.dumps(deployment.state, indent=2))
    elif args.action == "recover":
        deployment.recover()
    else:
        getattr(deployment, args.action)(args.drain_seconds)


if __name__ == "__main__":
    try:
        main()
    except (ReleaseError, OSError, ValueError, KeyError) as error:
        print("Release stopped: " + str(error), file=sys.stderr)
        sys.exit(1)

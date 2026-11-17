#!/usr/bin/env python3
"""Non-billable live check for the isolated SIT blue-green validation stack."""

import http.server
import json
from pathlib import Path
import socketserver
import subprocess
import sys
import threading
import time
import urllib.request


def main():
    root = Path(sys.argv[1]).resolve()
    config = json.loads((root / ".bluegreen/config.json").read_text())
    if (config["blue_container"] != "sub2api-bg-blue"
            or config["probe_url"] != "http://127.0.0.1:18088"):
        raise RuntimeError("This test is restricted to the isolated SIT validation stack")
    command = ["python3", str(root / "release.py"), "--directory", str(root)]
    accepted, done, stop = threading.Event(), threading.Event(), threading.Event()
    results, health = [], []

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            accepted.set()
            deadline = time.time() + 240
            while ("18081;" in Path(config["upstream_file"]).read_text()
                   and time.time() < deadline):
                time.sleep(0.1)
            time.sleep(12)
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"drained":true}')

        def log_message(self, *args):
            pass

    class Server(socketserver.ThreadingMixIn, http.server.HTTPServer):
        daemon_threads = True

    server = Server(("127.0.0.1", 18090), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()

    def slow():
        try:
            with urllib.request.urlopen(config["probe_url"] + "/__bluegreen_drain_test",
                                        timeout=240) as response:
                results.append((response.status, response.read().decode()))
        finally:
            done.set()

    def monitor():
        while not stop.is_set():
            try:
                with urllib.request.urlopen(config["probe_url"] + "/health", timeout=3) as response:
                    health.append(response.status == 200
                                  and json.loads(response.read()).get("status") == "ok")
            except Exception:
                health.append(False)
            stop.wait(0.2)

    def release(*args):
        subprocess.check_call(command + list(args))

    threading.Thread(target=slow, daemon=True).start()
    assert accepted.wait(10)
    threading.Thread(target=monitor, daemon=True).start()
    try:
        release("deploy", "--drain-seconds", "0")
        state = json.loads((root / ".bluegreen/state.json").read_text())
        assert state["active"] == "green" and state["pending"]["slot"] == "blue"
        assert not done.is_set(), "Long request completed before retention was checked"
        running = subprocess.check_output(["docker", "inspect", "sub2api-bg-blue",
                                           "--format", "{{.State.Running}}"]).decode().strip()
        assert running == "true"
        print("PASS: old app retained while request remained active", flush=True)
        assert done.wait(45)
        assert results == [(200, '{"drained":true}')], results
        print("PASS: existing long request completed with HTTP 200", flush=True)
        release("finish", "--drain-seconds", "20")
        state = json.loads((root / ".bluegreen/state.json").read_text())
        assert state["pending"] is None, state
        release("rollback", "--drain-seconds", "20")
        state = json.loads((root / ".bluegreen/state.json").read_text())
        assert state["active"] == "blue" and state["pending"] is None, state
        print("PASS: rollback restored blue and drained green", flush=True)
        assert health and all(health)
    finally:
        stop.set()
        server.shutdown()
        server.server_close()
        print("HEALTH_CHECKS: {} FAILURES: {}".format(
            len(health), sum(not entry for entry in health)), flush=True)
        print("LONG_REQUEST: " + repr(results), flush=True)


if __name__ == "__main__":
    main()

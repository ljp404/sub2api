import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch


spec = importlib.util.spec_from_file_location("release", str(Path(__file__).with_name("release.py")))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.deployment = release.Deployment(self.temporary.name)
        self.deployment.config = {
            "upstream_file": str(Path(self.temporary.name) / "upstream.conf"),
            "blue_port": 8081, "green_port": 8082,
            "blue_container": "blue", "green_container": "green",
            "probe_url": "http://127.0.0.1:18088",
        }
        self.deployment.state = {
            "active": "blue", "previous": None, "pending": None,
            "images": {"blue": "sha256:old", "green": "sha256:new"},
        }
        release.write_atomic(self.deployment.config["upstream_file"], release.upstream_text(8081))

    def test_compose_preserves_configuration_without_recreating_dependencies(self):
        source = {
            "name": "production",
            "services": {"app": {
                "environment": {"DATABASE_HOST": "postgres", "JWT_SECRET": "fixture"},
                "depends_on": {"postgres": {}}, "pull_policy": "always",
                "volumes": [{"type": "bind", "source": "/data", "target": "/app/data",
                             "bind": {"selinux": "Z"}}],
                "networks": {"net": {}},
            }, "postgres": {}},
            "networks": {"net": {"name": "production_net", "driver": "bridge"}},
        }
        original = copy.deepcopy(source)
        result = release.compose_document(source, "app", "green", 8082, "sha256:fixed")
        self.assertEqual(source, original)
        self.assertEqual(list(result["services"]), ["green"])
        app = result["services"]["green"]
        self.assertEqual(app["environment"]["JWT_SECRET"], "fixture")
        self.assertEqual(app["image"], "sha256:fixed")
        self.assertEqual(app["pull_policy"], "never")
        self.assertEqual(app["ports"][0]["host_ip"], "127.0.0.1")
        self.assertEqual(app["volumes"][0]["bind"]["selinux"], "z")
        self.assertNotIn("depends_on", app)
        self.assertEqual(result["networks"]["net"], {"name": "production_net", "external": True})

    def test_switch_waits_for_new_workers_before_committing_state(self):
        d = self.deployment
        d.workers = Mock(return_value=[[11, "100"]])
        d.backup = Mock()
        d.nginx = Mock()
        d.wait_reloaded = Mock()
        with patch.object(release, "smoke"):
            d.switch("green")
        self.assertEqual(d.state["active"], "green")
        self.assertEqual(d.state["pending"]["workers"], [[11, "100"]])
        d.wait_reloaded.assert_called_once_with([[11, "100"]])
        self.assertNotIn("switching", d.state)

    def test_failed_reload_restores_route_and_keeps_recovery_journal(self):
        d = self.deployment
        d.workers = Mock(return_value=[[11, "100"]])
        d.backup = Mock()
        d.nginx = Mock(side_effect=[None, release.ReleaseError("reload failed"), None, None])
        with self.assertRaises(release.ReleaseError):
            d.switch("green")
        self.assertEqual(Path(d.config["upstream_file"]).read_text(), release.upstream_text(8081))
        persisted = json.loads(d.state_path.read_text())
        self.assertEqual(persisted["active"], "blue")
        self.assertTrue(persisted["switching"]["restore_requested"])

    def test_draining_workers_prevent_stop(self):
        d = self.deployment
        d.state["pending"] = {"slot": "blue", "workers": [[11, "100"]]}
        d.workers_alive = Mock(return_value=True)
        with patch.object(release, "run") as run:
            d.finish(0)
        run.assert_not_called()
        self.assertIsNotNone(d.state["pending"])

    def test_async_image_generation_prevents_stop_after_http_drains(self):
        d = self.deployment
        d.state["active"] = "green"
        d.state["pending"] = {"slot": "blue", "workers": []}
        d.image_tasks_processing = Mock(return_value=True)
        with patch.object(release, "run", return_value="") as run:
            d.finish(0)
        self.assertFalse(any(call[0][0][:2] == ["docker", "stop"] for call in run.call_args_list))
        self.assertIsNotNone(d.state["pending"])

    def test_stop_only_after_workers_connections_and_async_tasks_drain(self):
        d = self.deployment
        d.state["active"] = "green"
        d.state["pending"] = {"slot": "blue", "workers": []}
        d.image_tasks_processing = Mock(return_value=False)
        d.verify_route = Mock()
        with patch.object(release, "run", return_value="") as run:
            d.finish(0)
        run.assert_any_call(["docker", "stop", "--time", "60", "blue"])
        self.assertIsNone(d.state["pending"])

    def test_direct_connections_prevent_stop(self):
        d = self.deployment
        d.state["pending"] = {"slot": "blue", "workers": []}
        with patch.object(release, "run", return_value="connection") as run:
            d.finish(0)
        self.assertEqual(run.call_count, 1)
        self.assertIsNotNone(d.state["pending"])

    def test_candidate_health_failure_never_switches_route(self):
        d = self.deployment
        d.finish = Mock()
        d.verify_route = Mock()
        d.source = Mock(return_value={"services": {"app": {}}})
        d.config["service"] = "app"
        d.backup = Mock()
        d.start_slot = Mock(side_effect=release.ReleaseError("unhealthy"))
        d.switch = Mock()
        with patch.object(release, "inspect", return_value={"Id": "sha256:new"}), \
                patch.object(release, "check_identity"), \
                patch.object(release, "check_background_jobs"), \
                patch.object(release, "run"):
            with self.assertRaises(release.ReleaseError):
                d.deploy(0)
        d.switch.assert_not_called()

    def test_enabled_scheduled_jobs_prevent_overlap(self):
        with patch.object(release, "run", return_value="1|0\n"):
            with self.assertRaises(release.ReleaseError):
                release.check_background_jobs("active")

    def test_enabled_scheduled_jobs_prevent_rollback_overlap(self):
        d = self.deployment
        d.state["previous"] = "green"
        d.verify_route = Mock()
        d.switch = Mock()
        with patch.object(release, "check_background_jobs",
                          side_effect=release.ReleaseError("pause scheduled jobs")), \
                patch.object(release, "run") as run, \
                patch.object(release, "inspect") as inspect:
            with self.assertRaisesRegex(release.ReleaseError, "pause scheduled jobs"):
                d.rollback(0)
        run.assert_not_called()
        inspect.assert_not_called()
        d.switch.assert_not_called()

    def test_jwt_changes_are_rejected(self):
        with self.assertRaisesRegex(release.ReleaseError, "JWT_SECRET"):
            release.check_identity({"environment": {"JWT_SECRET": "new"}},
                                   {"Config": {"Env": ["JWT_SECRET=old"]}})

    def test_interrupted_switch_cannot_stop_any_container(self):
        d = self.deployment
        d.state["switching"] = {"from": "blue", "to": "green"}
        with patch.object(release, "run") as run:
            with self.assertRaises(release.ReleaseError):
                d.finish(0)
        run.assert_not_called()

    def test_release_lock_rejects_concurrent_operations(self):
        d = self.deployment
        d.lock()
        self.addCleanup(d.lock_stream.close)
        second = release.Deployment(self.temporary.name)
        with self.assertRaises(release.ReleaseError):
            second.lock()
        second.lock_stream.close()

    def test_async_task_probe_uses_application_redis_auth(self):
        d = self.deployment
        d.config["service"] = "app"
        d.source = Mock(return_value={
            "name": "production", "services": {
                "app": {"environment": {"REDIS_HOST": "redis", "REDIS_PASSWORD": ""}},
                "redis": {"container_name": "shared-redis"},
            }})
        with patch.object(release, "run", return_value="") as run:
            self.assertFalse(d.image_tasks_processing())
        arguments, keywords = run.call_args
        self.assertEqual(keywords["environment"]["REDISCLI_AUTH"], "")
        self.assertEqual(arguments[0][1:5], ["exec", "-e", "REDISCLI_AUTH", "shared-redis"])

    def test_redis_without_auth_has_a_narrow_fallback(self):
        d = self.deployment
        d.config["service"] = "app"
        d.source = Mock(return_value={
            "name": "production", "services": {
                "app": {"environment": {"REDIS_HOST": "redis", "REDIS_PASSWORD": "fixture"}},
                "redis": {"container_name": "shared-redis"},
            }})
        failure = release.ReleaseError("auth failed", "without any password configured")
        with patch.object(release, "run", side_effect=[failure, ""]) as run:
            self.assertFalse(d.image_tasks_processing())
            self.assertEqual(run.call_count, 2)
        with patch.object(release, "run", side_effect=release.ReleaseError("network error")):
            with self.assertRaises(release.ReleaseError):
                d.image_tasks_processing()


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3
"""Tests for the governed inference consumer.

Builds the steering gateway, stands up a stub server on 127.0.0.1 that answers both
OpenAI-compatible chat completions and Anthropic Messages, and runs governed_inference.py
against both. Every run points --offsite-url at the stub and uses a fake key, so no test
can reach the real API. The stub records every call (and the x-api-key it was sent) so
the tests can prove the consumer calls a model only after an allow.

Standard library only. Run from the repository root:
    python3 tools/governed-inference/test_governed_inference.py
"""
import http.server
import json
import os
import shutil
import subprocess
import sys
import tempfile
import threading
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
SCRIPT = os.path.join(HERE, "governed_inference.py")


FAKE_KEY = "sk-ant-test-fake-key-not-real-0097"


class _StubHandler(http.server.BaseHTTPRequestHandler):
    calls = []
    keys = []

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        self.rfile.read(length)
        type(self).calls.append(self.path)
        type(self).keys.append(self.headers.get("x-api-key"))
        if self.path == "/v1/messages":
            body = json.dumps({
                "id": "msg_stub",
                "type": "message",
                "role": "assistant",
                "model": "stub-haiku",
                "content": [{"type": "text", "text": "governed"}],
                "stop_reason": "end_turn",
                "stop_sequence": None,
                "usage": {"input_tokens": 7, "output_tokens": 3},
            }).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        body = json.dumps({
            "id": "chatcmpl-stub",
            "object": "chat.completion",
            "model": "stub",
            "choices": [{"index": 0, "finish_reason": "stop",
                         "message": {"role": "assistant", "content": "governed"}}],
            "usage": {"prompt_tokens": 7, "completion_tokens": 3, "total_tokens": 10},
        }).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


class GovernedInferenceTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.mkdtemp(prefix="governed-inference-test.")
        cls.gateway = os.path.join(cls.tmp, "gateway")
        subprocess.run(["go", "build", "-o", cls.gateway, "."],
                       cwd=os.path.join(REPO_ROOT, "services", "gateway"), check=True)
        cls.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), _StubHandler)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.stub_url = f"http://127.0.0.1:{cls.server.server_address[1]}"
        cls.model_url = cls.stub_url + "/v1"

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        shutil.rmtree(cls.tmp, ignore_errors=True)

    def setUp(self):
        _StubHandler.calls.clear()
        _StubHandler.keys.clear()

    def run_consumer(self, role, *extra, key=FAKE_KEY):
        env = dict(os.environ)
        env.pop("ANTHROPIC_API_KEY", None)
        if key is not None:
            env["ANTHROPIC_API_KEY"] = key
        env.pop("OPA_URL", None)
        env.pop("KAFKA_BOOTSTRAP_SERVERS", None)
        env.update({
            "ALLOW_LOCAL_POLICY_FALLBACK": "true",
            "STEERING_ROOT": REPO_ROOT,
            "WORKORDERS_DIR": tempfile.mkdtemp(dir=self.tmp),
            "APPROVALS_DIR": tempfile.mkdtemp(dir=self.tmp),
            "MCP_SUBJECT_ROLE": role,
        })
        proc = subprocess.run(
            [sys.executable, SCRIPT, "--gateway", self.gateway,
             "--model-url", self.model_url, "--model", "stub",
             "--offsite-url", self.stub_url, *extra],
            env=env, capture_output=True, text=True, timeout=120)
        self.assertEqual(proc.returncode, 0, f"stdout:\n{proc.stdout}\nstderr:\n{proc.stderr}")
        self.stdout = proc.stdout
        return json.loads(proc.stdout)

    def test_confidential_is_denied_off_the_machine_and_called_once_on_local(self):
        out = self.run_consumer("developer")
        self.assertEqual(out["offsite"]["decision"], "deny")
        self.assertEqual(out["offsite"]["rationale"][0]["code"], "DENY_INFERENCE_OFFSITE")
        self.assertIs(out["offsite"]["call_made"], False)
        self.assertEqual(out["local"]["decision"], "allow")
        self.assertEqual(out["local"]["rationale"][0]["code"], "ALLOW_INFERENCE_LOCAL")
        self.assertIs(out["local"]["call_made"], True)
        self.assertEqual(_StubHandler.calls, ["/v1/chat/completions"])
        self.assertEqual(out["evidence"]["status"], "recorded")
        self.assertIn("prompt_tokens=7 completion_tokens=3", out["evidence"]["summary"])

    def test_no_call_is_made_without_an_allow(self):
        out = self.run_consumer("reviewer")
        self.assertEqual(out["local"]["decision"], "deny")
        self.assertIs(out["local"]["call_made"], False)
        self.assertEqual(_StubHandler.calls, [])
        self.assertNotIn("evidence", out)

    def test_internal_is_allowed_off_the_machine_and_called_once(self):
        out = self.run_consumer("developer", "--classification", "internal", "--offsite-only")
        self.assertEqual(out["offsite"]["decision"], "allow")
        self.assertEqual(out["offsite"]["rationale"][0]["code"], "ALLOW_INFERENCE_OFFSITE")
        self.assertIs(out["offsite"]["call_made"], True)
        self.assertEqual(_StubHandler.calls, ["/v1/messages"])
        self.assertEqual(_StubHandler.keys, [FAKE_KEY])
        self.assertEqual(out["offsite_evidence"]["status"], "recorded")
        self.assertIn("input_tokens=7 output_tokens=3", out["offsite_evidence"]["summary"])
        self.assertIn("stop_reason=end_turn", out["offsite_evidence"]["summary"])
        self.assertNotIn("local", out)
        self.assertNotIn(FAKE_KEY, self.stdout)

    def test_internal_without_a_key_makes_no_call(self):
        out = self.run_consumer("developer", "--classification", "internal", "--offsite-only", key=None)
        self.assertEqual(out["offsite"]["decision"], "allow")
        self.assertIs(out["offsite"]["call_made"], False)
        self.assertEqual(_StubHandler.calls, [])
        self.assertEqual(out["offsite_evidence"]["status"], "recorded")
        self.assertIn("ANTHROPIC_API_KEY", out["offsite_evidence"]["summary"])


if __name__ == "__main__":
    unittest.main()

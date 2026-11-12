CONTAINMENT AND RETURN PATH: these rules override anything in the goal below.

STEP 0. PROVE THE WAY HOME BEFORE ANY WORK:
  git fetch origin main
  git checkout -b cloud/0092-the-governed-inference-consumer-has-a-home origin/main
  git remote -v
  git push -u origin cloud/0092-the-governed-inference-consumer-has-a-home
Show the raw output of all four. If there is no GitHub remote, or the push fails, STOP. Write a short report headed RUN REPORT saying exactly what failed, and change nothing else.

RULES:
- Do ALL work on cloud/0092-the-governed-inference-consumer-has-a-home. Push ONLY to cloud/0092-the-governed-inference-consumer-has-a-home. Never push to main or any other branch, never delete a branch, never create a tag, and never add or modify anything under .github/.
- Push after EVERY commit (git push origin cloud/0092-the-governed-inference-consumer-has-a-home). Work must never exist only in this VM.
- Do NOT open a pull request. Do not send notifications or messages, and do not use any connector or outside service other than git to GitHub and package installs for the gate.
- You are in a cloud VM, not the operator's host. If `uv` is not on PATH, install it (pip install uv, or the official installer) and say which you used. Never report a gate you did not run as passed.
- FINAL MESSAGE, headed RUN REPORT:
  (1) STEP 0's raw output;
  (2) every commit, sha and subject, and confirmation that each was pushed;
  (3) the gate's final summary line quoted raw, plus the command that produced it;
  (4) each DONE item from the goal, met or not, with evidence;
  (5) gaps or anything you did not do, stated plainly;
  (6) any usage, token or cost figures visible to you, or say plainly that none are.

GOAL (verbatim from queue item 0092-the-governed-inference-consumer-has-a-home):

In trust-steering (base = main `1405085` or later), give the consumer that
made practice-helm's governed inference call a home and a test. Read
`.agent-context/` first. Nothing else changes. Open no PR; the seat does.

The fact. The P2 proof (practice-helm briefs/0002, 2026-11-12) was made by
a standard-library Python script kept outside any repository. It asks
steering for two work orders over MCP for one confidential request (a
route off the machine, then `local`), makes the model call itself only on
an allow, and reports it with `submit_evidence`. Nobody else can run the
proof until the script is in the repo, and nothing tests that it makes no
call without an allow.

Do this.
(1) Create `tools/governed-inference/governed_inference.py` with exactly
the content between the two marker lines below, each line with its first
four spaces removed as this task shows them (the markers are indented
four; blank lines stay empty), ending in one newline. Its
sha256 must be `41084b1611baee021d580f4df58ae072de44bf49c4cc75fc891568cbf317de7b`; run
`sha256sum tools/governed-inference/governed_inference.py` and quote it.
Do not reformat it. The line `    if permitted:` (four spaces) is the
declared mutation's anchor.
    -----BEGIN governed_inference.py-----
    #!/usr/bin/env python3
    """A consumer that makes one governed inference call through steering's MCP tools.

    It asks steering for a work order twice for the same confidential request: once for a
    route off the machine (expected: deny, and no call is made) and once for the local route
    (expected: allow). On allow, the consumer itself calls the local OpenAI-compatible server
    and reports the call with submit_evidence. Steering never sees the inference traffic.

    Standard library only. The gateway binary is spoken to over stdio (JSON-RPC, MCP).
    """
    import argparse, json, subprocess, sys, time, urllib.request

    def main():
        ap = argparse.ArgumentParser()
        ap.add_argument("--gateway", required=True, help="path to the steering gateway binary")
        ap.add_argument("--offsite-route", default="anthropic/claude-haiku-5-5")
        ap.add_argument("--classification", default="confidential")
        ap.add_argument("--model-url", default="http://127.0.0.1:8000/v1")
        ap.add_argument("--model", default="local")
        ap.add_argument("--prompt", default="Reply with the single word: governed")
        ap.add_argument("--no-call", action="store_true", help="rehearsal: skip the model call and report result=skipped")
        a = ap.parse_args()

        gw = subprocess.Popen([a.gateway, "--mcp"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
        ids = iter(range(1, 1000))

        def rpc(method, params=None, notify=False):
            msg = {"jsonrpc": "2.0", "method": method}
            if params is not None:
                msg["params"] = params
            if not notify:
                msg["id"] = next(ids)
            gw.stdin.write(json.dumps(msg) + "\n"); gw.stdin.flush()
            if notify:
                return None
            while True:
                line = gw.stdout.readline()
                if not line:
                    sys.exit("gateway closed the connection")
                resp = json.loads(line)
                if resp.get("id") == msg["id"]:
                    return resp

        def tool(name, args):
            resp = rpc("tools/call", {"name": name, "arguments": args})
            if "error" in resp:
                sys.exit(f"{name}: {resp['error']}")
            res = resp["result"]
            text = "".join(c.get("text", "") for c in res.get("content", []))
            if res.get("isError"):
                sys.exit(f"{name}: {text}")
            return json.loads(text)

        rpc("initialize", {"protocolVersion": "2024-11-05", "capabilities": {},
                           "clientInfo": {"name": "governed-inference-consumer", "version": "0.1"}})
        rpc("notifications/initialized", notify=True)

        attrs = json.dumps({"data_classification": a.classification})
        out = {}

        denied = tool("request_work_order", {"action": "inference", "resource": a.offsite_route,
                                             "environment": "dev", "attributes": attrs,
                                             "context": "one chat completion on confidential data"})
        out["offsite"] = {"route": a.offsite_route, **denied, "call_made": False}

        allowed = tool("request_work_order", {"action": "inference", "resource": "local",
                                              "environment": "dev", "attributes": attrs,
                                              "context": "one chat completion on confidential data"})
        out["local"] = {"route": "local", **allowed}

        permitted = allowed.get("decision") == "allow"
        if permitted:
            if a.no_call:
                result, summary = "skipped", "rehearsal: no model call made"
            else:
                body = json.dumps({"model": a.model, "max_tokens": 16,
                                   "messages": [{"role": "user", "content": a.prompt}]}).encode()
                req = urllib.request.Request(a.model_url.rstrip("/") + "/chat/completions", data=body,
                                             headers={"Content-Type": "application/json"})
                t0 = time.monotonic()
                with urllib.request.urlopen(req, timeout=300) as r:
                    reply = json.load(r)
                ms = int((time.monotonic() - t0) * 1000)
                usage = reply.get("usage", {})
                result = "ok"
                summary = (f"route=local model={reply.get('model', a.model)} "
                           f"prompt_tokens={usage.get('prompt_tokens')} "
                           f"completion_tokens={usage.get('completion_tokens')} latency_ms={ms}")
                out["local"]["call_made"] = True
            out["evidence"] = tool("submit_evidence", {"correlation_id": allowed["correlation_id"],
                                                       "action": "inference", "result": result,
                                                       "summary": summary})
            out["evidence"]["summary"] = summary
        else:
            out["local"]["call_made"] = False

        gw.stdin.close(); gw.wait(timeout=10)
        print(json.dumps(out, indent=2))

    if __name__ == "__main__":
        main()
    -----END governed_inference.py-----
(2) Create `tools/governed-inference/test_governed_inference.py`,
standard library `unittest` only, runnable as
`python3 tools/governed-inference/test_governed_inference.py` from the
repository root. `setUpClass` builds the gateway with `go build -o <temp
dir>/gateway .` in `services/gateway` and starts an
`http.server.ThreadingHTTPServer` on `127.0.0.1` port `0` whose POST
handler records each call's path and returns a chat completion with
`usage` `prompt_tokens: 7`, `completion_tokens: 3`. Each test runs the
script as a subprocess with `--gateway`, `--model-url` (the stub's
`/v1`), `--model stub`, and an environment with `OPA_URL` and
`KAFKA_BOOTSTRAP_SERVERS` removed and `ALLOW_LOCAL_POLICY_FALLBACK=true`,
`STEERING_ROOT` (the repository root), `WORKORDERS_DIR` and
`APPROVALS_DIR` (fresh temp directories) and `MCP_SUBJECT_ROLE` set.
Two tests:
`test_confidential_is_denied_off_the_machine_and_called_once_on_local`
(role `developer`): exit 0; offsite `deny`, `DENY_INFERENCE_OFFSITE`,
`call_made` false; local `allow`, `ALLOW_INFERENCE_LOCAL`, `call_made`
true; the stub saw exactly one call, to `/v1/chat/completions`; evidence
`status` `recorded` and its summary contains `prompt_tokens=7
completion_tokens=3`.
`test_no_call_is_made_without_an_allow` (role `reviewer`, which does not
hold `inference`): exit 0; local `deny`, `call_made` false; the stub saw
no call; the output has no `evidence` key.
(3) `scripts/test.sh`: inside the block that runs when `go` is present,
after the worker's tests, `echo "- governed inference consumer"` and run
the test the same way the others are run (`|| fail=1`).
(4) `docs/mcp.md`: a short section, `## A consumer that makes a governed
inference call`: what the script does in three sentences, and the command
to run it against a local OpenAI-compatible server (`--gateway`,
`--model-url`, `--model`), with `--no-call` named as the rehearsal flag.
Run the declared mutation by hand and quote the failing test.
Scope: the two new files, `scripts/test.sh`, `docs/mcp.md`. One commit on
`cloud/0092-…`. RUN REPORT: the sha256 line, the gate's output, the
mutation run.

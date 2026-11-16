CONTAINMENT AND RETURN PATH: these rules override anything in the goal below.

STEP 0. PROVE THE WAY HOME BEFORE ANY WORK:
  git fetch origin main
  git checkout -b cloud/0097-the-consumer-makes-the-allowed-call-off-the-machine origin/main
  git remote -v
  git push -u origin cloud/0097-the-consumer-makes-the-allowed-call-off-the-machine
Show the raw output of all four. If there is no GitHub remote, or the push fails, STOP. Write a short report headed RUN REPORT saying exactly what failed, and change nothing else.

RULES:
- Do ALL work on cloud/0097-the-consumer-makes-the-allowed-call-off-the-machine. Push ONLY to cloud/0097-the-consumer-makes-the-allowed-call-off-the-machine. Never push to main or any other branch, never delete a branch, never create a tag, and never add or modify anything under .github/.
- Push after EVERY commit (git push origin cloud/0097-the-consumer-makes-the-allowed-call-off-the-machine). Work must never exist only in this VM.
- Do NOT open a pull request. Do not send notifications or messages, and do not use any connector or outside service other than git to GitHub and package installs for the gate.
- You are in a cloud VM, not the operator's host. If `uv` is not on PATH, install it (pip install uv, or the official installer) and say which you used. Never report a gate you did not run as passed.
- FINAL MESSAGE, headed RUN REPORT:
  (1) STEP 0's raw output;
  (2) every commit, sha and subject, and confirmation that each was pushed;
  (3) the gate's final summary line quoted raw, plus the command that produced it;
  (4) each DONE item from the goal, met or not, with evidence;
  (5) gaps or anything you did not do, stated plainly;
  (6) any usage, token or cost figures visible to you, or say plainly that none are.

GOAL (verbatim from queue item 0097-the-consumer-makes-the-allowed-call-off-the-machine):

In trust-steering (public; base = main `fa1001f` or later), let the governed
inference consumer make the call off the machine when policy allows it.
Today `tools/governed-inference/governed_inference.py` treats the off-machine
work order as the one that is denied: it records `call_made: False` for it
whatever the decision and only ever calls the local server. With
`--classification internal` the policy allows the route
(`ALLOW_INFERENCE_OFFSITE`) and the consumer ignores it. READ the script and
its test FIRST. No policy file, no service code, no `.github/` file changes.
Open no PR; the seat opens it. MAKE NO REAL API CALL: every test uses a stub
server on 127.0.0.1, and you have no key.

Do, in `tools/governed-inference/governed_inference.py` (standard library
only, as now; no new dependency):
(A) After the off-machine `request_work_order`, the result is held in a
variable named `offsite`, and the call is guarded by exactly this line:
    if offsite.get("decision") == "allow":
Under it the consumer makes the call, sets `out["offsite"]["call_made"]`,
and reports it with `submit_evidence` against that work order's
`correlation_id`; the tool's reply goes in `out["offsite_evidence"]` with
the `summary` added. On any other decision nothing is called and
`call_made` stays False, as today.
(B) The call is one request to the Anthropic Messages API, made with
`urllib`: `POST <offsite-url>/v1/messages`, headers `Content-Type:
application/json`, `x-api-key: <key>`, `anthropic-version: 2026-10-21`;
body `model` = the route with its `anthropic/` prefix removed, `max_tokens`
1024, one user message holding `--prompt`. The key is read from the
environment variable `ANTHROPIC_API_KEY` and nowhere else. New flag
`--offsite-url`, default `https://api.anthropic.com`.
(C) The evidence summary is `route=<route> model=<reply model>
input_tokens=<n> output_tokens=<n> latency_ms=<n> stop_reason=<s>`. It
never holds the prompt, the reply text or the key, and the script's stdout
never holds the key. `result` is `ok`; `error` when `stop_reason` is
`refusal` or the API answers with an HTTP error (summary `route=<route>
http_status=<code>`); `skipped` with `--no-call` ("rehearsal: no model call
made") or when `ANTHROPIC_API_KEY` is unset or empty ("no ANTHROPIC_API_KEY
in the environment: no model call made"), and in both `call_made` is False.
(D) New flag `--offsite-only`: ask only for the off-machine work order,
print the result and exit; without it the local work order follows as
today. The work orders' `context` names the declared classification
instead of the fixed word "confidential". The module docstring says what
the script now does.
(E) `docs/mcp.md`, the consumer's section: the off-machine call, the two
flags, `ANTHROPIC_API_KEY`, that the call is billed to the key's owner,
and that the evidence carries token counts and no content.

Tests, in `tools/governed-inference/test_governed_inference.py`: the stub
server also answers `POST /v1/messages` with a Messages-shaped reply
(`usage.input_tokens` 7, `output_tokens` 3, `stop_reason` `end_turn`) and
records the path and the `x-api-key` it was sent. Every run of the
consumer passes `--offsite-url` at the stub and a fake key, so no test can
reach the real API. New tests: `--classification internal --offsite-only`
→ allow, `ALLOW_INFERENCE_OFFSITE`, exactly one `/v1/messages` call
carrying the fake key, evidence recorded with `input_tokens=7
output_tokens=3`, the fake key nowhere in stdout; the same with no key →
no call, evidence summary names the missing key. The two existing tests
still pass unchanged in what they assert (a confidential request reaches
`/v1/chat/completions` once and `/v1/messages` never).

CHECKS, raw output quoted in the RUN REPORT: the test file's output; the
gate's last eight lines; `--classification internal --offsite-only
--no-call` run by hand with its JSON; `git status --short` and `git diff
--stat` before the commit. One commit on
`cloud/0097-the-consumer-makes-the-allowed-call-off-the-machine`, message
"The governed inference consumer makes the call off the machine when
policy allows it, and evidences it (helm queue/0097)". Scope:
`tools/governed-inference/`, `docs/mcp.md`, `CHANGELOG.md`.

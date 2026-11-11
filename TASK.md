CONTAINMENT AND RETURN PATH: these rules override anything in the goal below.

STEP 0. PROVE THE WAY HOME BEFORE ANY WORK:
  git fetch origin main
  git checkout -b cloud/0090-mcp-evidence-joins-its-decision-trace origin/main
  git remote -v
  git push -u origin cloud/0090-mcp-evidence-joins-its-decision-trace
Show the raw output of all four. If there is no GitHub remote, or the push fails, STOP. Write a short report headed RUN REPORT saying exactly what failed, and change nothing else.

RULES:
- Do ALL work on cloud/0090-mcp-evidence-joins-its-decision-trace. Push ONLY to cloud/0090-mcp-evidence-joins-its-decision-trace. Never push to main or any other branch, never delete a branch, never create a tag, and never add or modify anything under .github/.
- Push after EVERY commit (git push origin cloud/0090-mcp-evidence-joins-its-decision-trace). Work must never exist only in this VM.
- Do NOT open a pull request. Do not send notifications or messages, and do not use any connector or outside service other than git to GitHub and package installs for the gate.
- You are in a cloud VM, not the operator's host. If `uv` is not on PATH, install it (pip install uv, or the official installer) and say which you used. Never report a gate you did not run as passed.
- FINAL MESSAGE, headed RUN REPORT:
  (1) STEP 0's raw output;
  (2) every commit, sha and subject, and confirmation that each was pushed;
  (3) the gate's final summary line quoted raw, plus the command that produced it;
  (4) each DONE item from the goal, met or not, with evidence;
  (5) gaps or anything you did not do, stated plainly;
  (6) any usage, token or cost figures visible to you, or say plainly that none are.

GOAL (verbatim from queue item 0090-mcp-evidence-joins-its-decision-trace):

In trust-steering (Go; base = main `b7d1020` or later), make the evidence
that `submit_evidence` records join the decision trace of the work order
it closes. Read `.agent-context/` first. Nothing else changes. Open no PR;
the seat does.

The fact. `handleSubmitEvidence` (`services/gateway/mcp_server.go`) builds
its evidence event with `TraceID: randomHex(16)` and with `details` holding
only the caller's `summary`. The worker's evidence for the same kind of
event carries the trace's own id (`services/worker/main.go` 150,
`TraceID: trace.TraceID`) and `"policy_decision": decision` (140). So a
packet exported for a work order closed over MCP has an evidence record
whose `trace_id` matches no trace. Seen 2026-11-11 on a real broker:
packet `corr-81ec2849dbf11f5e`, checked by
`agent-audit-packet/verify.sh` → `FAIL every evidence record closes the
loop on an allowed decision: evidence.jsonl:1 and its trace disagree on
trace_id; 1 of 1 evidence record(s) carry no details.policy_decision`,
exit 1.

Do this.
(1) `services/gateway/workorders.go`: `WorkOrder` gains
`TraceID string` with tag `json:"trace_id,omitempty"`, after `EventID`;
`recordWorkOrder` sets `w.TraceID = trace.TraceID`.
(2) `services/gateway/mcp_server.go`, `handleSubmitEvidence`: `details`
starts as `map[string]string{"policy_decision": workOrder.Decision}` and
still takes `summary` when given. After it, exactly the line
`	traceID := workOrder.TraceID` (one tab; the declared mutation anchors
on it), then: when it is empty (a work order stored before trace ids were
kept) `traceID = randomHex(16)`. The evidence event uses `TraceID:
traceID`.
(3) A test-only observer beside `mcpTraceObserver`:
`var mcpEvidenceObserver func(EvidenceEvent)`, called with the evidence
event once it is built and before any publish.
(4) `services/gateway/workorders_test.go`:
`TestSubmitEvidenceJoinsTheDecisionTrace` — with both observers set,
issue a work order for `inference` on `local` declaring `confidential`,
submit evidence for it, and assert the evidence's `TraceID` equals the
decision trace's and `Details["policy_decision"]` is `allow`. Reset both
observers in `t.Cleanup`.
(5) `docs/mcp.md`, under `submit_evidence`: one sentence — the recorded
evidence carries the work order's `trace_id` and its policy decision.
Run `gofmt -l .` (no output) and `go vet ./...` in `services/gateway`.
Run the declared mutation by hand and quote the failing test's name.
Scope: `services/gateway/workorders.go`, `mcp_server.go`,
`workorders_test.go`, `docs/mcp.md`. One commit on `cloud/0090-…`.
RUN REPORT: the gate's output, the mutation run, the new test name.

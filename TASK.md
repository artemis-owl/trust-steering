CONTAINMENT AND RETURN PATH: these rules override anything in the goal below.

STEP 0. PROVE THE WAY HOME BEFORE ANY WORK:
  git fetch origin main
  git checkout -b cloud/0091-the-worker-leaves-consumer-work-orders-alone origin/main
  git remote -v
  git push -u origin cloud/0091-the-worker-leaves-consumer-work-orders-alone
Show the raw output of all four. If there is no GitHub remote, or the push fails, STOP. Write a short report headed RUN REPORT saying exactly what failed, and change nothing else.

RULES:
- Do ALL work on cloud/0091-the-worker-leaves-consumer-work-orders-alone. Push ONLY to cloud/0091-the-worker-leaves-consumer-work-orders-alone. Never push to main or any other branch, never delete a branch, never create a tag, and never add or modify anything under .github/.
- Push after EVERY commit (git push origin cloud/0091-the-worker-leaves-consumer-work-orders-alone). Work must never exist only in this VM.
- Do NOT open a pull request. Do not send notifications or messages, and do not use any connector or outside service other than git to GitHub and package installs for the gate.
- You are in a cloud VM, not the operator's host. If `uv` is not on PATH, install it (pip install uv, or the official installer) and say which you used. Never report a gate you did not run as passed.
- FINAL MESSAGE, headed RUN REPORT:
  (1) STEP 0's raw output;
  (2) every commit, sha and subject, and confirmation that each was pushed;
  (3) the gate's final summary line quoted raw, plus the command that produced it;
  (4) each DONE item from the goal, met or not, with evidence;
  (5) gaps or anything you did not do, stated plainly;
  (6) any usage, token or cost figures visible to you, or say plainly that none are.

GOAL (verbatim from queue item 0091-the-worker-leaves-consumer-work-orders-alone):

In trust-steering (Go; base = main `fcaacac` or later), make the worker
leave alone every work order its consumer executes, and let a denied one
be exported. Read `.agent-context/` first. Nothing else changes. Open no
PR; the seat does.

The ruling (operator, 2026-11-11: "Worker skips them"). A work order
issued over MCP is executed by the consumer that asked for it, which
reports with `submit_evidence` (`docs/mcp.md` 25). The worker must publish
no evidence for it.

The fact. With the stream profile up, `runKafkaConsumer`
(`services/worker/kafka.go`) turns every trace on `decision.trace.v1` into
evidence with `traceToEvidence`: `action.executed`, `result: ok` for an
allow. Seen 2026-11-11: packet `corr-f7d3e7961088a78d`, action
`inference`, evidence from `service_name: worker`, `result: ok`, when no
inference ran. Nothing in a trace says who executes it. And
`tools/audit-packet-fetch` waits for a trace and an evidence record, so
with the worker off a denied MCP request cannot be exported: `timeout
waiting for decision trace and evidence for correlation_id`, exit 1
(`corr-5e2ef4da4f5e3cdb`, whose trace was on the topic).

Do this.
(1) `services/gateway/mcp_server.go`, `handleRequestWorkOrder`: after the
caller's attributes are copied, always set `attrs["executor"] =
"consumer"` and assign `reqObj.Attributes = attrs` (drop the `len(attrs) >
0` guard). `isTrustReservedAttribute` gains `"executor"`, so a caller
cannot set it.
(2) `services/worker/main.go`: `consumerExecutes(trace DecisionTrace)
bool`, true when `trace.Request.Attributes["executor"] == "consumer"`; and
`evidenceForConsumedTrace(trace DecisionTrace) (EvidenceEvent, bool,
error)`, whose first statement is exactly the line
`	if consumerExecutes(trace) {` (one tab; the declared mutation anchors on
it, so it appears once in the file) returning `EvidenceEvent{}, false,
nil`; otherwise it returns `traceToEvidence(trace)` with the bool true
when there is no error. `handleExecute` is unchanged.
(3) `services/worker/kafka.go`: the consumer calls
`evidenceForConsumedTrace`; when the bool is false it commits the offset
and returns without producing.
(4) `tools/audit-packet-fetch/main.go`: `evidenceExpected(decision []byte)
bool`, false only when `policy.decision` is `deny` and
`request.attributes.executor` is `consumer` (unparseable input: true). The
loop waits for evidence only while `evidenceExpected(decisionBytes)`. The
evidence file is still written, empty when there is none.
(5) `scripts/export_audit_packet.sh`: an empty evidence file is not
schema-checked and not corr-checked, and gives an empty `evidence.jsonl`.
(6) Tests. `services/worker/main_test.go`:
`TestWorkerLeavesConsumerExecutedWorkOrdersAlone` (an allowed trace with
`executor: consumer` → bool false) and
`TestWorkerStillExecutesItsOwnWorkOrders` (no attributes → bool true,
`action.executed`, `ok`). `services/gateway/workorders_test.go`:
`TestRequestWorkOrderTraceNamesTheConsumerAsExecutor` (with
`mcpTraceObserver`), and a `request_work_order` whose attributes carry
`executor` is refused as trust-reserved. New file
`tools/audit-packet-fetch/main_test.go`: `TestEvidenceExpected`, four
cases (denied + consumer → false; allowed + consumer, denied without
executor, not JSON → true).
(7) `docs/mcp.md`: two sentences — every trace of a work order issued
over MCP carries `request.attributes.executor: consumer` and the worker
publishes no evidence for it; a denied one exports with an empty
`evidence.jsonl`.
Run `gofmt -l services tools` (no output) and `go vet ./...` in the three
modules. Run the declared mutation by hand and quote the failing test.
Scope: the files named above. One commit on `cloud/0091-…`.
RUN REPORT: the gate's output, the mutation run, the new test names.

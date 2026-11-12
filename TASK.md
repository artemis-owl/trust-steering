CONTAINMENT AND RETURN PATH: these rules override anything in the goal below.

STEP 0. PROVE THE WAY HOME BEFORE ANY WORK:
  git fetch origin main
  git checkout -b cloud/0093-trust-steering-is-ready-to-be-read origin/main
  git remote -v
  git push -u origin cloud/0093-trust-steering-is-ready-to-be-read
Show the raw output of all four. If there is no GitHub remote, or the push fails, STOP. Write a short report headed RUN REPORT saying exactly what failed, and change nothing else.

RULES:
- Do ALL work on cloud/0093-trust-steering-is-ready-to-be-read. Push ONLY to cloud/0093-trust-steering-is-ready-to-be-read. Never push to main or any other branch, never delete a branch, never create a tag, and never add or modify anything under .github/.
- Push after EVERY commit (git push origin cloud/0093-trust-steering-is-ready-to-be-read). Work must never exist only in this VM.
- Do NOT open a pull request. Do not send notifications or messages, and do not use any connector or outside service other than git to GitHub and package installs for the gate.
- You are in a cloud VM, not the operator's host. If `uv` is not on PATH, install it (pip install uv, or the official installer) and say which you used. Never report a gate you did not run as passed.
- FINAL MESSAGE, headed RUN REPORT:
  (1) STEP 0's raw output;
  (2) every commit, sha and subject, and confirmation that each was pushed;
  (3) the gate's final summary line quoted raw, plus the command that produced it;
  (4) each DONE item from the goal, met or not, with evidence;
  (5) gaps or anything you did not do, stated plainly;
  (6) any usage, token or cost figures visible to you, or say plainly that none are.

GOAL (verbatim from queue item 0093-trust-steering-is-ready-to-be-read):

In trust-steering (base = main `a64cbaa` or later), fix three things a
reader of the repository or of a packet it exports would be misled or
exposed by. Read `.agent-context/` first. Nothing else changes. Open no
PR; the seat does.

The facts, read by the seat at `a64cbaa` on 2026-11-12.
(a) `docs/mcp.md` 334 links `roles.json` as a `file:///Users/…` URL: the
operator's home path, and a link that works on one machine.
(b) `scripts/export_audit_packet.sh` 146–158 copies whatever is in
`supplychain/sbom/out` and then writes a note saying the directory
"contains the public key (`cosign.pub`)" and that "These attestations
mathematically prove the integrity of the binaries enforcing the policy
decisions contained in this Audit Packet." Seen in the packets of
2026-11-12: no `cosign.pub`, two SBOMs created `2026-04-18T01:06:34Z`,
decisions made by a gateway built that day from source. The note claims
what the packet does not carry.
(c) `handleDecide` (`services/gateway/main.go`) passes a caller's
`request.attributes.executor` through to the trace. The worker leaves
every trace marked `executor: consumer` alone (queue/0091), so an HTTP
caller can switch off the worker's evidence for its own request. The MCP
path reserves the key; the HTTP path does not.

Do this.
(1) `docs/mcp.md`: the link becomes
`[roles.json](../policies/opa/rego/roles.json)`.
(2) `services/gateway/main.go`, `handleDecide`: immediately before the
comment "Approval state only enters policy input via the server-side
store", exactly the line
`	delete(req.Request.Attributes, "executor")` (one tab; the declared
mutation anchors on it, so it appears once in the file), with a comment
above it saying who sets the executor and why a caller's is stripped.
(3) `services/gateway/approvals_test.go`:
`TestHandleDecideStripsCallerSuppliedExecutor`, in the shape of
`TestHandleDecideIgnoresCallerSuppliedApproval`: `decideProd` with
attributes `executor: consumer` returns 200 and a trace whose
`request.attributes` has no `executor` key.
(4) `scripts/export_audit_packet.sh`: the note at
`supplychain/README.md` is generated from what the directory holds. With
neither a key nor an SBOM: "No supply-chain file accompanies this
packet." With `cosign.pub`: one line naming it and saying this export
verifies no signature with it. Per SBOM: its file name and the `created`
time from its `creationInfo`. With any SBOM: a closing sentence that an
SBOM describes the image it was generated from at the time shown, and
that this export does not check that those images, or any locally built
binary, produced the decisions in the packet. The sentence about
mathematical proof goes.
(5) `scripts/test.sh`: before "- schema files present", a step
`- no home path in tracked files` that sets `fail=1` and prints the
matches when `git grep -nIE '/Users/[A-Za-z0-9._-]+' -- .
':!scripts/test.sh'` finds any.
Run `gofmt -l services tools` (no output) and `go vet ./...` in
`services/gateway`. Export one packet from
`schemas/examples/decision_trace.sample.json` with an empty evidence file
(`AUDIT_DECISION_TRACE_FILE`, `AUDIT_EVIDENCE_FILE`, absolute paths) and
quote the generated note. Run the declared mutation by hand and quote the
failing test.
Scope: the five files named. One commit on `cloud/0093-…`.
RUN REPORT: the gate's output, the generated note, the mutation run.

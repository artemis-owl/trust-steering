CONTAINMENT AND RETURN PATH: these rules override anything in the goal below.

STEP 0. PROVE THE WAY HOME BEFORE ANY WORK:
  git fetch origin main
  git checkout -b cloud/0089-an-inference-call-is-a-governed-action origin/main
  git remote -v
  git push -u origin cloud/0089-an-inference-call-is-a-governed-action
Show the raw output of all four. If there is no GitHub remote, or the push fails, STOP. Write a short report headed RUN REPORT saying exactly what failed, and change nothing else.

RULES:
- Do ALL work on cloud/0089-an-inference-call-is-a-governed-action. Push ONLY to cloud/0089-an-inference-call-is-a-governed-action. Never push to main or any other branch, never delete a branch, never create a tag, and never add or modify anything under .github/.
- Push after EVERY commit (git push origin cloud/0089-an-inference-call-is-a-governed-action). Work must never exist only in this VM.
- Do NOT open a pull request. Do not send notifications or messages, and do not use any connector or outside service other than git to GitHub and package installs for the gate.
- You are in a cloud VM, not the operator's host. If `uv` is not on PATH, install it (pip install uv, or the official installer) and say which you used. Never report a gate you did not run as passed.
- FINAL MESSAGE, headed RUN REPORT:
  (1) STEP 0's raw output;
  (2) every commit, sha and subject, and confirmation that each was pushed;
  (3) the gate's final summary line quoted raw, plus the command that produced it;
  (4) each DONE item from the goal, met or not, with evidence;
  (5) gaps or anything you did not do, stated plainly;
  (6) any usage, token or cost figures visible to you, or say plainly that none are.

GOAL (verbatim from queue item 0089-an-inference-call-is-a-governed-action):

In trust-steering (Go + Rego; base = main `82c782d` or later), make an
inference call a governed action: the policy decides, from the declared
data classification, whether a request may use a route off the machine.
Read `.agent-context/` first. Nothing else changes. Open no PR; the seat
does.

The fact. `docs/mcp.md` 45–46 already offers `inference` as an action of
`request_work_order`, and `data_classification` is a field of the decision
trace (`schemas/decision_trace.avsc` 71: "public, internal, confidential,
restricted"). No rule reads either: `policies/opa/rego/roles.json` grants
`inference` to no role, so every such request is `DEFAULT_DENY`, and the
MCP path stamps every trace `DataClassification: "internal"`
(`services/gateway/mcp_server.go` 230) whatever the caller declared.
Steering stays out of the data plane (`docs/mcp.md` 25): the consumer asks
for a work order, makes the call itself, and reports it with
`submit_evidence`. This item adds no proxying and no call to any model.

The rule. The request's `resource` is the route (a served model name).
The route `local` never leaves the machine and is allowed for any
classification. Any other route is off the machine and is allowed only
when `request.attributes.data_classification` is `public` or `internal`.
A missing or unknown classification is treated as not permitted off the
machine. The role must hold `inference`.

Do this.
(1) `policies/opa/rego/roles.json`: add `"inference"` to `developer`
only.
(2) `policies/opa/rego/decision.rego`: the simple-action `allow` (15–20)
and its `rationale` (37–42) each gain `action != "inference"` beside
`action != "deploy"`. Add
`offsite_classifications := {"public", "internal"}`, a rule
`inference_route_permitted` true when `input.request.resource == "local"`,
and a second body for it whose only line is exactly
`    input.request.attributes.data_classification in offsite_classifications`
(four spaces; the declared mutation anchors on it, so it appears once in
the file). `allow` for `inference` needs the role and
`inference_route_permitted`. Three rationales, mutually exclusive, each
one item: `ALLOW_INFERENCE_LOCAL` (resource is `local`),
`ALLOW_INFERENCE_OFFSITE` (another route, permitted),
`DENY_INFERENCE_OFFSITE` (role holds `inference`, another route, not
permitted; the message names the route and says to use the local route).
A role without `inference` stays `DEFAULT_DENY`.
(3) `services/gateway/main.go` `evaluatePolicyLocal` (365): the same rule
and the same three codes, so the fallback and OPA cannot disagree.
(4) `services/gateway/mcp_server.go` 230: the trace's
`DataClassification` is the caller's `data_classification` attribute when
it is one of the four schema values, else `"internal"` as today.
(5) `docs/mcp.md`: under `request_work_order`, say that for `inference`
the `resource` is the route, `local` is the on-machine route, and
`attributes.data_classification` decides whether another route is allowed.
(6) Tests. `policies/opa/tests/decision_test.rego`, each asserting both
`allow` and `rationale[0].code`:
`test_inference_confidential_offsite_denied`,
`test_inference_confidential_local_allowed`,
`test_inference_internal_offsite_allowed`,
`test_inference_unclassified_offsite_denied`,
`test_inference_unknown_classification_offsite_denied`,
`test_inference_role_without_it_denied` (a `reviewer`, `DEFAULT_DENY`).
`services/gateway/main_test.go`: one table test running the same six
inputs through `evaluatePolicyLocal` with the same expected decision and
code. `services/gateway/workorders_test.go` or beside it: a
`request_work_order` for `inference` declaring `confidential` yields a
trace whose `data_classification` is `confidential`.
Run `opa test -v policies/opa/rego policies/opa/tests` and quote its last
line; if `opa` cannot be installed in the run's environment, say so in the
RUN REPORT and do not report the Rego tests as run. Run the declared
mutation by hand and quote the failing test names.
Scope: the two policy files, the Rego test file, `services/gateway/main.go`,
`mcp_server.go`, their tests, `docs/mcp.md`. One commit on `cloud/0089-…`.
RUN REPORT: the gate's output, the mutation run, the new test names.

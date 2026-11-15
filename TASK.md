CONTAINMENT AND RETURN PATH: these rules override anything in the goal below.

STEP 0. PROVE THE WAY HOME BEFORE ANY WORK:
  git fetch origin main
  git checkout -b cloud/0096-steering-says-what-it-does origin/main
  git remote -v
  git push -u origin cloud/0096-steering-says-what-it-does
Show the raw output of all four. If there is no GitHub remote, or the push fails, STOP. Write a short report headed RUN REPORT saying exactly what failed, and change nothing else.

RULES:
- Do ALL work on cloud/0096-steering-says-what-it-does. Push ONLY to cloud/0096-steering-says-what-it-does. Never push to main or any other branch, never delete a branch, never create a tag, and never add or modify anything under .github/.
- Push after EVERY commit (git push origin cloud/0096-steering-says-what-it-does). Work must never exist only in this VM.
- Do NOT open a pull request. Do not send notifications or messages, and do not use any connector or outside service other than git to GitHub and package installs for the gate.
- You are in a cloud VM, not the operator's host. If `uv` is not on PATH, install it (pip install uv, or the official installer) and say which you used. Never report a gate you did not run as passed.
- FINAL MESSAGE, headed RUN REPORT:
  (1) STEP 0's raw output;
  (2) every commit, sha and subject, and confirmation that each was pushed;
  (3) the gate's final summary line quoted raw, plus the command that produced it;
  (4) each DONE item from the goal, met or not, with evidence;
  (5) gaps or anything you did not do, stated plainly;
  (6) any usage, token or cost figures visible to you, or say plainly that none are.

GOAL (verbatim from queue item 0096-steering-says-what-it-does):

In trust-steering (public; base = main `a928c49` or later), make the
repository say what it does. A read of the public repo on 2026-11-15 found
documented commands that fail, a demo that leaves servers running, and
prose that promises guarantees no code provides. Build the small repairs;
cut every other claim down to what the code does. Add no feature, no
dependency and no new guarantee. Open no PR; the seat opens it.

BUILD
(A) `scripts/demo_local.sh` leaves nothing running and refuses busy ports.
Fact: cleanup kills `$!` of `( cd …; go run . ) &`, which is the subshell;
the compiled gateway and worker survive on `*:8081` and `*:9090`, and a
later run passes against them (`wait_for_http` accepts any listener).
Do: build each service into `${out_dir}` and run the binary, exactly:
    (cd "${root_dir}/services/gateway" && go build -o "${out_dir}/gateway" .)
    env "${gateway_env[@]}" "${out_dir}/gateway" >"${out_dir}/gateway.log" 2>&1 &
    GATEWAY_PID=$!
and the same shape for the worker. Directly after the `export GOCACHE`
line, exactly:
    for port in 8081 9090; do
      if curl -s -o /dev/null "http://127.0.0.1:${port}/" 2>/dev/null; then
        echo "demo_local: port ${port} is already serving; stop that process first" >&2
        exit 1
      fi
    done
The step headers say "Policy decision:" where they say "OPA Decision:",
since the gateway may have used its local fallback; the README's quoted
run is replaced by a real run of the new script, pasted.
(B) Documented commands run as written.
- `docs/runbook.md`: each MCP client config sets `STEERING_ROOT`, as
  `docs/mcp.md` already says; its env table lists it. Without it every
  request is denied (`roles.json not found`).
- `docs/mcp.md`, the governed-inference command: state the environment it
  needs (`ALLOW_LOCAL_POLICY_FALLBACK=true` or `OPA_URL`) and that the
  gateway binary is built first. Its env table lists `WORKORDERS_DIR` and
  `ROLES_CONFIG_PATH`. The example correlation id is one the tool accepts.
- `make sbom` / `make sign`: the docs state the prerequisite the scripts
  have (local images tagged `steering-gateway:dev` and `steering-worker:dev`,
  built by `scripts/kind_load.sh`).
- `docs/runbook.md` prerequisites for `make test`: go, curl, python3, jq,
  shasum, git; `opa` optional.
(C) `scripts/demo_compose.sh`, `demo_kind.sh` and `demo_stream.sh` print the
decision the gateway returned (`jq -r .policy.decision`) and say in one
line that the sample request is denied by policy and its evidence is
`action.skipped`. Do not change the sample request.

CUT (prose; say what the code does, in the fewest words)
(D) "Immutable" (README lines 3 and 9, the demo's step 4 header): the
packet is a directory of plain files; say "audit packet".
(E) "an allow is only returned once the decision trace is durably
persisted" (README, CHANGELOG, roadmap): true only with
`KAFKA_BOOTSTRAP_SERVERS` set; say so, and that without a broker the trace
is returned to the caller and a work-order record is written.
(F) "the agent can never self-approve", "Unforgeable": the MCP channel
rejects `human_approved`; the grant itself is a file written by
`scripts/approve.sh`, which authenticates nobody. Say exactly that.
(G) "No action without policy decision … enforceable": the worker acts on
any well-formed trace it is sent and does not check where it came from;
it also executes nothing, it records. Say that in
`docs/decision-trace-schema.md`, CHANGELOG and `services/README.md`.
(H) Policy hash: `bundle_hash` is the sha256 of `decision.rego`, computed
by the launching script; `roles.json` is not in it. Say that where the
docs say "exact policy bundle" or "content-addressed".
(I) Runtime schema validation: records are validated in CI on the samples
and by the demo scripts on their output, not at runtime. Fix
`schemas/README.md`, roadmap, `docs/decision-trace-schema.md`.
(J) Provenance: `sign_images.sh` signs images and attests an SBOM; nothing
ties an attestation to a commit or build recipe, the export copies a
public key and SBOMs, takes one correlation id, and makes no zip. Fix the
roadmap's "[DONE]" line, CHANGELOG, `supplychain/README.md`,
`supplychain/sbom/README.md`, `supplychain/build-pipeline-strategy.md`.
(K) Facts: the repo has tag `v0.2.0` and no `v0.1.0` (CHANGELOG, roadmap);
six of seven modules are `go 1.24.0`; `schemas/examples/decision_trace.sample.json`
and `docs/mcp.md` use a rationale code the policy does not have
(`ALLOW_DEV`): use the real one for that request or mark the sample as a
deny; `docs/control-map.md` line 13 names both schemas for fields only
the trace has; `scripts/README.md` lists every script and drops
"placeholder"; `docs/corpus.md`, `infra/k3d/README.md`,
`docs/reading-plan-6w.md` line 6 and the roadmap's test counts say what
is true now.
(L) Private context: `docs/roadmap.md` line 32 names a local account as
approver: write `user:<operator>`. Links into repos a reader cannot open
(`.agent-context/*.md`, roadmap's `../../kestrel-helm/…`) lose the link
and keep a plain-text name.

CHECKS, raw output quoted in the RUN REPORT: the gate's last eight lines;
the demo's stdout; a second run of the demo straight after the first
(it must pass, not hit a busy port); `git status --short` and `git diff
--stat` before the commit. One commit on
`cloud/0096-steering-says-what-it-does`, message "The demo leaves nothing
running, documented commands run, and the docs claim what the code does
(helm queue/0096)". Scope: `scripts/`, `docs/`, `README.md`, `CHANGELOG.md`,
`supplychain/**/*.md`, `schemas/README.md`, `schemas/examples/`,
`services/README.md`, `infra/k3d/README.md`, `.agent-context/`. No file
under `services/*/` other than a README, no policy file, no `.github/`.
Where a cut needs a judgment this goal does not make, cut less and say so
in the RUN REPORT.

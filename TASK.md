CONTAINMENT AND RETURN PATH: these rules override anything in the goal below.

STEP 0. PROVE THE WAY HOME BEFORE ANY WORK:
  git fetch origin main
  git checkout -b cloud/0095-the-local-demo-allows-what-it-says-it-allows origin/main
  git remote -v
  git push -u origin cloud/0095-the-local-demo-allows-what-it-says-it-allows
Show the raw output of all four. If there is no GitHub remote, or the push fails, STOP. Write a short report headed RUN REPORT saying exactly what failed, and change nothing else.

RULES:
- Do ALL work on cloud/0095-the-local-demo-allows-what-it-says-it-allows. Push ONLY to cloud/0095-the-local-demo-allows-what-it-says-it-allows. Never push to main or any other branch, never delete a branch, never create a tag, and never add or modify anything under .github/.
- Push after EVERY commit (git push origin cloud/0095-the-local-demo-allows-what-it-says-it-allows). Work must never exist only in this VM.
- Do NOT open a pull request. Do not send notifications or messages, and do not use any connector or outside service other than git to GitHub and package installs for the gate.
- You are in a cloud VM, not the operator's host. If `uv` is not on PATH, install it (pip install uv, or the official installer) and say which you used. Never report a gate you did not run as passed.
- FINAL MESSAGE, headed RUN REPORT:
  (1) STEP 0's raw output;
  (2) every commit, sha and subject, and confirmation that each was pushed;
  (3) the gate's final summary line quoted raw, plus the command that produced it;
  (4) each DONE item from the goal, met or not, with evidence;
  (5) gaps or anything you did not do, stated plainly;
  (6) any usage, token or cost figures visible to you, or say plainly that none are.

GOAL (verbatim from queue item 0095-the-local-demo-allows-what-it-says-it-allows):

In trust-steering (public; base = main `f38e9cf` or later), make the local
process demo do what it prints, and quote one run of it in the README.
READ CLAUDE.md FIRST if present. Nothing else changes. Open no PR; the
seat opens it.

Fact (seat, 2026-11-14, main `f38e9cf`, host `opa` on PATH):
`./scripts/demo_local.sh` prints `OPA Decision: deny` / `Denied by
default.` for BOTH step 1 and step 2, the worker then records
`action.skipped`, and the script ends `Demo completed successfully!`,
exit 0. Cause: line 52 runs `opa run --server ... "${root_dir}/policies/opa"`,
so `rego/roles.json` loads at `data.rego.roles` and the policy's
`data.roles` is empty. Compose mounts `policies/opa/rego` and is not
affected. `opa eval -d policies/opa/rego` on the step-2 request gives
`allow: true`.

Do:
(A) `scripts/demo_local.sh` line 52 loads `"${root_dir}/policies/opa/rego"`.
(B) Directly after the step-1 `decision=$(jq ...)` line, exactly:
    [[ "${decision}" == "deny" ]] || { echo "demo_local: step 1 expected deny, got ${decision}" >&2; exit 1; }
    and after the step-2 one, exactly:
    [[ "${decision}" == "allow" ]] || { echo "demo_local: step 2 expected allow, got ${decision}" >&2; exit 1; }
(C) README.md, under the local process demo's command block: one fenced
    block holding the script's real stdout from a run you make with `opa`
    on PATH, from the `Step 1` banner through `Demo completed
    successfully!`, with the output directory given as a relative
    argument so no absolute path appears; one sentence above it naming
    the commit it was run at and the date. Paste, do not retype.
If `opa` is not installed in the VM, install the release binary the
repo's CI uses and say so in the RUN REPORT; do not quote a run made on
the fallback path.

CHECKS, raw output quoted in the RUN REPORT: the gate's last five lines;
the demo's stdout; `git status --short` and `git diff --stat` before the
commit (no file under `out/` or `.gocache/` is committed). One commit on
`cloud/0095-the-local-demo-allows-what-it-says-it-allows`, message "The
local demo loads the roles it checks, fails when a step decides wrong,
and its run is quoted in the README (helm queue/0095)". Scope:
`scripts/demo_local.sh`, `README.md`.

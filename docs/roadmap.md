# Roadmap

This roadmap is organized around one anchor demo (“Decision Trace Gateway”) and the evidence you want to be able to hand to a reviewer. Priority order: **credibility over breadth** — the trust boundary must be real before adding surface area.

## Current state (as of July 2026)

- **Primary local runtime:** Docker Compose stack (`infra/compose/`, `make compose-up`) — OPA, gateway, worker, pinned images, loopback-only ports, policy bundle hash aligned with `scripts/kind_deploy.sh`.
- **MCP surface (built):** the gateway embeds an MCP server (`gateway --mcp`) exposing `request_work_order`, `submit_evidence`, and `get_policy_status`; see `docs/mcp.md`. End-to-end validation with a commodity agent client is Phase 6 work.
- **Streaming (Phase 2 slice):** With `COMPOSE_PROFILES=stream`, Redpanda runs and `compose_up.sh` sets `KAFKA_BOOTSTRAP_SERVERS=redpanda:9092` for gateway/worker. Gateway produces JSON to `decision.trace.v1`; worker consumes and produces evidence JSON to `decision.evidence.v1`. Without `KAFKA_BOOTSTRAP_SERVERS`, behavior stays HTTP-only. Integration check: `scripts/test_stream.sh` (Docker + broker).
- **Observability (Phase 3 slice):** With `COMPOSE_PROFILES=obs` (or `--profile obs`), OTel collector + Jaeger run; gateway/worker export OTLP (`compose_up.sh` sets the endpoint). Trace context crosses Kafka record headers when `stream,obs` is used. Integration check: `scripts/test_obs.sh` / `make test-obs` (Docker + Jaeger Query API).
- **Kubernetes parity:** Kind cluster + `scripts/kind_deploy.sh` / `make kind-demo` for periodic smoke and manifest-shaped deploys (streaming not wired in Kind yet, but observability tracing parity is achieved).
- Schemas, policy-as-code, Go services, schema checks in `scripts/test.sh`, CI (`.github/workflows/ci.yml`), and AI RMF–anchored control map (`docs/control-map.md`).

## Phase 6 — Trust hardening + agent integration (current focus)

The July 2026 audit found that the gateway's trust boundary was self-attested: callers could assert their own role and approval flags, and audit events could be silently dropped on broker failure. Fixing that credibly *is* the product. In priority order:

1. **[DONE 2026-07-09]** Evidence durability: MCP tool calls fail closed when the decision trace or evidence event cannot be persisted (same contract as HTTP `/decide`); worker commits offsets only after evidence is durably produced.
2. **[DONE 2026-07-09]** Non-spoofable MCP subject: role/identity come from gateway configuration (`MCP_SUBJECT_ROLE`, `MCP_AGENT_ID`); trust-reserved attributes (`role`, `agent_id`, `human_approved`) are rejected from client input.
3. **[DONE 2026-07-09]** Key hygiene: cosign keys untracked and gitignored (`sign_images.sh` regenerates on demand); old key purged from git history (`git filter-repo`, force-pushed) and rotated.
4. **[DONE 2026-07-09]** Unforgeable approval gate: prod deploys record a server-side pending approval; a human grants via `scripts/approve.sh` (approver identity, TTL); the gateway consumes the grant (single-use, matched to correlation_id + action/resource/environment) and is the only writer of `human_approved`/`approver` in policy input. Covered by gateway tests; flow documented in `docs/mcp.md`.
5. **Trust-in-the-Loop demo (integration built; evidence session remaining):** the `steering-demo` sibling repo integrates the gateway into a governed project — an approvals-console web UI (the human grant surface) deployed through the gate itself, with an advisory tier (MCP + agent rules) and a physical tier (PreToolUse hook + deploy-script verification of the consumed work order). The full deny → grant-in-console → allow → gated-deploy loop is verified against the real gateway over MCP stdio.
   - [x] **Run the demo with a live agent session (Claude Code / Cursor) and record it as demo evidence; then archive `steering-demo`** (operator ruling, 2026-07-23 repo audit — it is a validation vehicle, not a product)
     - **Status:** `Completed` | **Priority:** *High* | **Project:** [[steering]] | **Source:** `roadmap.md`
     - Completed 2026-07-29, live Claude Code session over MCP stdio against the real stack
       (OPA policy, Redpanda topics, work-order store). Full loop on `corr-7c0d7d79a7243dd6`:
       request → **deny** (`approval_status: pending`) → agent stopped per rules → operator
       granted **in the console** (`user:williamcorbett`, 15-min TTL) → retry → **allow**
       (grant consumed single-use) → `make deploy ENV=prod` verified the consumed record and
       landed console `84e4761` on :9090 → `submit_evidence` accepted by the new linkage
       enforcement → audit packet exported (`out/audit-packets/corr-7c0d7d79a7243dd6`).
       Negative proofs captured live: ungated prod deploy blocked pre-grant (physical tier,
       exit 2); forged evidence for an unissued correlation_id rejected fail-closed; replay
       of the consumed grant denied (`approval_status: consumed`). Post-demo
       `grant-reconcile`: 6 closed grants, 0 unlinked — the demo grant closed in the audit
       trail. Raw artifacts (all tool responses, deploy transcript, packet copy) captured
       per the evidence-capture convention. `steering-demo` archived per the ruling.
6. **Evidence linkage:** `submit_evidence` should validate the correlation_id against a previously issued (and allowed) work order.
   - [x] **Validate `submit_evidence` correlation_id against an issued, allowed work order**
     - **Status:** `Completed` | **Priority:** *High* | **Project:** [[steering]] | **Source:** `roadmap.md`
     - Delivered 2026-07-29 ([helm brief 0003](../../kestrel-helm/briefs/0003-evidence-grant-linkage.md)):
       issued work orders are recorded server-side (`services/gateway/workorders.go`, one JSON
       file per correlation_id under `out/workorders/`, the approvals-store pattern) on both the
       MCP and HTTP `/decide` paths; `submit_evidence` rejects fail-closed unless the
       correlation_id names an issued work order decided `allow` whose action matches the
       evidence. Accepted evidence closes the work order (count, first/last time, last result).
       Seven new gateway tests cover accept, reject-unknown, reject-denied, action-mismatch,
       malformed-id, `/decide` recording, and the deny→grant→allow retry; full `make test` green.
   - [x] **An issued work order that is never closed is invisible — no grant/evidence reconciliation**
     - **Status:** `Completed` | **Priority:** *High* | **Project:** [[steering]] | **Source:** `roadmap.md`
     - Delivered 2026-07-29 ([helm brief 0003](../../kestrel-helm/briefs/0003-evidence-grant-linkage.md)):
       `tools/grant-reconcile` joins both topics by correlation_id and reports unclosed grants,
       unlinked evidence, and identical-task duplicate grants (exit 2 on findings). The dogfood
       shape is a regression fixture (`main_test.go`). Live re-verification against the real
       packet: 11 traces / 5 evidence / 9 grants — 5 closed (0 unlinked: the 5/5 core check
       reconfirmed), **4 unclosed surfaced**: the diagnosed `corr-1f83be18839abb94` (its lost
       task-1 text recovered from the trace's `context` attribute) *plus three silent grants
       from the two failed dogfood attempts* (2× 11:53 smoke, 1× 12:36 budget-killed run) —
       the gap class was systemic, not a one-off. Duplicate-grant detection located the
       13:40/13:46 pair exactly as diagnosed.
     - Diagnosed 2026-07-29 from the first verification against a *real* evidence packet
       (kestrel-team dogfood `run-1785159637`, not a fixture). The core check passed: 5/5
       evidence events on `decision.evidence.v1` link to allowed work orders on
       `decision.trace.v1`, policy history visible including a `deploy` → deny.
       The apparent "5/6 coverage gap" is **not a dropped evidence event**:
       - Six work orders were issued, but the run's plan had five tasks (`plan_size: 5` in
         `~/.kestrel/runs.jsonl`), and the five evidence events map 1:1 onto the *last* five
         work orders, each submitted ~60s after its grant.
       - `corr-1f83be18839abb94` (13:40:40Z) and `corr-4ea18174e92634df` (13:46:32Z) carry the
         **same task description** — task 1 was granted twice, ~6 minutes apart. The second
         grant completed and submitted evidence; the first never closed.
       - Retention is not a factor: `retention.ms` is 7 days on both topics and every record
         from the run is inside the window (`decision.trace.v1` log-start 31/HW 42,
         `decision.evidence.v1` log-start 41/HW 46).
       - What happened during those six minutes is no longer recoverable — the executor's
         session logs went with the workspace the wrap-up bug destroyed.
     - **The gap this exposes:** the trust boundary has no lifecycle for a work order that is
       issued and never closed. Nothing expires a grant and nothing reconciles issued grants
       against submitted evidence, so an *allowed action with no recorded outcome* is silently
       absent from the audit trail. The parent item (validating `correlation_id` on
       `submit_evidence`) closes the reverse direction — evidence without a grant — and needs
       this as its companion. Closing charter focus #2 depends on it.
     - Reads: Redpanda via in-container `rpk` on the internal listener (`localhost:9092`);
       host-side reads use `127.0.0.1:19092`. Priority proposed by an agent; scope and priority
       are the operator's to set.

## Next tasks (after Phase 6)

- Policy/schema change gates in CI (tests + short risk note per change); basic CI (build/test/opa/schema checks) landed 2026-07-09.
- **[DONE]** Implement `scripts/export_audit_packet.sh` per `docs/audit-packet.md`.

## Deferred (polish, not proof)

These add surface area but do not strengthen the core trust story; pick them up only after Phase 6:

- Extend observability: metrics/logs pipelines, Grafana/dashboards—see `docs/reading-plan-6w.md` Week 4+.
- Streaming parity in Kind (Compose remains the streaming path).
- **Kind parity is frozen as-is (operator ruling, 2026-07-25):** rehearsal/smoke only, no
  further investment. Ruled during the helm drift review — the streaming/OTEL/provenance
  buildout is in-mandate (evidence and audit), kind is the named boundary.
- **Kind cluster lifecycle hygiene — resolved 2026-08-02 (hygiene, not investment; parity
  stays frozen).** The 2026-07-27 kestrel-helm host audit found two idle kind clusters
  (`steering-control-plane`; `artemis-control-plane` + 2 workers) running for 2+ weeks —
  standing RAM/CPU with no active rehearsal, contrary to the ecosystem's
  no-trailing-processes discipline (cf. punchcard dormancy, ADR-0008). **Both deleted
  2026-08-02** (operator-approved cleanup: `kind delete cluster --name artemis|steering`;
  `kind get clusters` → "No kind clusters found"). Rehearsal is run-on-demand via
  `make kind-demo`, which recreates from scratch. Standing-process posture as practiced:
  the Compose services (OPA/Redpanda/OTEL — load-bearing for kestrel-team governance
  since 2026-07-27) are the *only* standing processes; evidence topics carry 7-day
  retention, so exports are point-in-time captures under `out/` (2026-07-31, 2026-08-02).
- Formal specs (`specs/`, TLA+ or similar).

## Phase 0 — Repo bootstrap (done)

- Opinionated layout for policies, schemas, observability, and supply chain evidence.
- Narrative specs: Decision Trace and Audit Packet.

## Phase 1 — Decision trace + policy loop

- Decision Trace and Evidence events + examples.
- OPA policy decisions with rationale and policy versioning.
- “No action without a policy decision” enforceable and testable.
- **Delivered path:** Compose (`make compose-up`) and Kind (`make kind-demo`); local process demo remains (`scripts/demo_local.sh`).

## Phase 2 — Streaming contracts + evidence append

- Publish decision-trace events to a topic boundary (`decision.trace.v1`; Compose stream profile + `KAFKA_*` envs).
- Consume and execute in a worker, emitting evidence events (`decision.evidence.v1`).
- Schema validation and compatibility checks as gates (CI + runtime); `scripts/test.sh` remains HTTP-only; `scripts/test_stream.sh` covers broker path.

## Phase 3 — Observability as evidence

- OpenTelemetry traces + collector + Jaeger in Compose (`obs` profile); correlation and trace IDs across HTTP and Kafka (`stream,obs`); Jaeger search by tags.
- [ ] **Observability further work: metrics/logs, dashboards, deeper “evidence queries”**
  - **Status:** `Planned` | **Priority:** *Low* | **Project:** [[steering]] | **Source:** `roadmap.md`

## Phase 4 — Provenance and software supply chain evidence (done)

- **[DONE]** SBOM per artifact (via Syft).
- **[DONE]** Sign and verify artifacts (via Cosign).
- **[DONE]** Attach attestations tied to a commit and a build recipe.

## Phase 5 — Audit packet export + change management

- **[DONE]** Export evidence for a correlation_id/window: events, policy bundle hash, schema versions, provenance proofs.
- [ ] **Add change gates and lightweight risk notes for policy/schema changes**
  - **Status:** `Planned` | **Priority:** *Medium* | **Project:** [[steering]] | **Source:** `roadmap.md`

## Deployment stance (local / small machines)

- **Default daily stack:** Docker Compose — one `up`/`down`, minimal moving parts, suitable for a Mac mini or laptop.
- **Kind:** Optional parity and rehearsal for Kubernetes; not required for the core HTTP → policy → trace → worker loop.

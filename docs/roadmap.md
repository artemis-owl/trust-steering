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
   - [ ] **Run the demo with a live agent session (Claude Code / Cursor) and record it as demo evidence; then archive `steering-demo`** (operator ruling, 2026-07-23 repo audit — it is a validation vehicle, not a product)
     - **Status:** `Planned` | **Priority:** *High* | **Project:** [[steering]] | **Source:** `roadmap.md`
6. Evidence linkage: `submit_evidence` should validate the correlation_id against a previously issued (and allowed) work order.

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
- **Investigate: kind cluster lifecycle hygiene (stub — hygiene, not investment; does not
  unfreeze parity).** The 2026-07-27 kestrel-helm host audit found two idle kind clusters
  (`steering-control-plane`; `artemis-control-plane` + 2 workers) running for 2+ weeks —
  standing RAM/CPU with no active rehearsal, contrary to the ecosystem's
  no-trailing-processes discipline (cf. punchcard dormancy, ADR-0008). Investigate:
  which cluster belongs to what, a teardown/recreate one-liner per cluster
  (`kind delete cluster` + the existing Make targets) so rehearsal is run-on-demand, and
  whether the Compose services (OPA/Redpanda/OTEL — now load-bearing for kestrel-team
  governance since 2026-07-27) should be documented as the *only* standing processes.
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

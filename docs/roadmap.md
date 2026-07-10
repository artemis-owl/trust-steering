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
3. **[DONE 2026-07-09]** Key hygiene: cosign keys untracked and gitignored (`sign_images.sh` regenerates on demand). *Follow-up: purge the old key from git history on the remote and treat it as rotated.*
4. **Unforgeable approval gate (next up):** an out-of-band human approval flow — approvals minted server-side (approval endpoint or CLI issuing a signed/opaque token bound to a correlation_id) so `human_approved` can never be asserted by the governed agent. This replaces the request-attribute gate in `decision.rego`.
5. **Trust-in-the-Loop demo:** connect a commodity agent (Claude Code, Cursor, Aider, Cline) over MCP and demonstrate deny-by-default, approval-gated execution end to end; record the session as demo evidence. Confirm MCP tool-execution parity in the client of choice.
6. Evidence linkage: `submit_evidence` should validate the correlation_id against a previously issued (and allowed) work order.

## Next tasks (after Phase 6)

- Policy/schema change gates in CI (tests + short risk note per change); basic CI (build/test/opa/schema checks) landed 2026-07-09.
- **[DONE]** Implement `scripts/export_audit_packet.sh` per `docs/audit-packet.md`.

## Deferred (polish, not proof)

These add surface area but do not strengthen the core trust story; pick them up only after Phase 6:

- Extend observability: metrics/logs pipelines, Grafana/dashboards—see `docs/reading-plan-6w.md` Week 4+.
- Streaming parity in Kind (Compose remains the streaming path).
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

- OpenTelemetry traces + collector + Jaeger in Compose (`obs` profile); correlation and trace IDs across HTTP and Kafka (`stream,obs`); Jaeger search by tags. Further work: metrics/logs, dashboards, deeper “evidence queries.”

## Phase 4 — Provenance and software supply chain evidence (done)

- **[DONE]** SBOM per artifact (via Syft).
- **[DONE]** Sign and verify artifacts (via Cosign).
- **[DONE]** Attach attestations tied to a commit and a build recipe.

## Phase 5 — Audit packet export + change management

- **[DONE]** Export evidence for a correlation_id/window: events, policy bundle hash, schema versions, provenance proofs.
- Add change gates and lightweight risk notes for policy/schema changes.

## Deployment stance (local / small machines)

- **Default daily stack:** Docker Compose — one `up`/`down`, minimal moving parts, suitable for a Mac mini or laptop.
- **Kind:** Optional parity and rehearsal for Kubernetes; not required for the core HTTP → policy → trace → worker loop.

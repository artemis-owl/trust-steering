# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Versions are release markers for the control plane as a whole (services, tools,
policies, schemas). Event-schema compatibility is versioned separately by topic
name (`decision.trace.v1`, `decision.evidence.v1`) and is *not* implied by the
version numbers below.

## [Unreleased]

### Changed

- `scripts/demo_local.sh` builds the gateway and worker into its output
  directory and runs the binaries, so its cleanup stops them; it refuses to
  start if anything already answers on `8081` or `9090`. Its step headers say
  "Policy decision:" (the gateway may have used its local fallback, not OPA).
- `scripts/demo_compose.sh`, `demo_kind.sh` and `demo_stream.sh` print the
  decision the gateway returned and say that the sample request is denied by
  policy, so its evidence is `action.skipped`.
- Docs cut to what the code does: no "immutable", "unforgeable" or
  "enforceable" claims the code does not back; documented commands state the
  environment they need.

## [0.2.0] - 2026-09-05

Module hygiene and release scaffolding — the small repo floor owed under
kestrel-helm `decisions/0014`, whose uv-lockfile plank translates to Go-module
hygiene in this repo. No policy, schema, or service-behavior change.

### Changed

- Six of the seven Go modules now declare the same `go 1.24.0` directive,
  consistent with the Go version CI pins (`.github/workflows/ci.yml`,
  `go-version: "1.24"`). Previously the directives had drifted four ways:
  `1.22` (`tools/schema-check`, `tools/approve`, `tools/kafka-read-one`,
  `services/worker`), `1.23.0` (`services/gateway`), `1.24.0`
  (`tools/audit-packet-fetch`), and `1.25.0` (`tools/grant-reconcile`).
  **Documented exception:** `tools/grant-reconcile` stays at `go 1.25.0` —
  its dependency `franz-go/pkg/kadm@v1.18.0` requires go ≥ 1.25.0, so the
  1.24.0 downgrade fails the build (predicted in the roadmap finding filed by
  this change, confirmed by attended verification 2026-09-05; the why lives
  in a comment on the directive itself).
- `services/gateway/go.mod`: dropped the stray `toolchain go1.24.2` line. It was
  compensating for a `go 1.23.0` directive under a Go 1.24 CI runner; with the
  directive itself at `1.24.0` the line is redundant and was the only
  `toolchain` pin in the repo.
- `services/gateway/Dockerfile`, `services/worker/Dockerfile`: builder stage
  `golang:1.23` → `golang:1.24`, so the Compose image builds match the module
  directives and CI instead of resolving a newer toolchain at build time. The
  distroless runtime stage, exposed ports, and entrypoints are unchanged.

### Added

- This `CHANGELOG.md`.

## [0.1.0] - 2026-08-02

Retroactive entry for the substrate standing at the time this changelog was
introduced. It records what existed, not a new release; see
[`docs/roadmap.md`](docs/roadmap.md) for the phase-by-phase history and the
open items behind each area.

### Added

- **Decision trace + evidence append path** — `services/gateway` maps an agent's
  execution context to an OPA decision and emits a decision-trace event;
  `services/worker` consumes it and appends an evidence event. HTTP-only by
  default; with `KAFKA_BOOTSTRAP_SERVERS` set, the path runs over
  `decision.trace.v1` → `decision.evidence.v1` (Redpanda under the Compose
  `stream` profile). With `KAFKA_BOOTSTRAP_SERVERS` set, an allow is only
  returned once the trace is published — the path fails closed. Without a
  broker the trace is returned to the caller and a work-order record is
  written. Roadmap: Phases 1–2.
- **Policy gates as code** — `policies/opa/rego/decision.rego` plus
  `policies/opa/tests`, run in CI via `opa test`. The worker records evidence
  for any well-formed trace it is sent and does not check where the trace came
  from; it executes nothing, it records. Roadmap: Phase 1.
- **Event schemas with conformance fixtures** — `schemas/decision_trace.avsc`
  and `schemas/evidence.avsc`, each with a sample payload under
  `schemas/examples/`, validated by `tools/schema-check` in CI and in the demo
  scripts. Roadmap: Phase 2.
- **Out-of-band human approval** — prod deploys record a server-side pending
  approval; the grant is a file written by `scripts/approve.sh`
  (`tools/approve`) with an approver name and a TTL. The script authenticates
  nobody. The MCP channel rejects `human_approved` from the agent; the grant is
  single-use and matched to correlation_id + action/resource/environment, and
  the gateway is the only writer of `human_approved`/`approver` in policy
  input. Roadmap: Phase 6.4.
- **MCP surface** — `gateway --mcp` exposes `request_work_order`,
  `submit_evidence`, and `get_policy_status`. Subject identity and role come
  from gateway configuration; trust-reserved client attributes (`role`,
  `agent_id`, `human_approved`) are rejected. See [`docs/mcp.md`](docs/mcp.md).
  Roadmap: Phase 6.1–6.2.
- **Evidence–grant linkage, both directions** — issued work orders are recorded
  server-side (`services/gateway/workorders.go`); `submit_evidence` rejects
  fail-closed unless the correlation_id names an issued, allowed work order
  whose action matches. `tools/grant-reconcile` closes the reverse direction,
  joining both topics to surface unclosed grants, unlinked evidence, and
  duplicate grants (exit 2 on findings). Roadmap: Phase 6.6.
- **Audit packets** — `scripts/export_audit_packet.sh` and
  `tools/audit-packet-fetch` write, for one correlation_id, a directory of
  plain files: the events, the policy file and its sha256, the schemas and
  their hashes, trace pointers, and copies of the cosign public key and SBOMs
  when present (no signature is verified and no zip is made). See
  [`docs/audit-packet.md`](docs/audit-packet.md). Roadmap: Phase 5.
- **Observability as evidence** — OpenTelemetry traces from gateway and worker
  to an OTel collector + Jaeger under the Compose `obs` profile; trace context
  crosses Kafka record headers under `stream,obs`. Roadmap: Phase 3.
- **Supply-chain scaffolding** — SBOM generation via Syft (`make sbom`),
  image signing and an SBOM attestation via Cosign (`make sign`), and
  verification (`make verify-provenance`), with keys untracked and regenerated
  on demand. Both need local images `steering-gateway:dev` and
  `steering-worker:dev` (built by `scripts/kind_load.sh`). Nothing ties an
  attestation to a commit or build recipe. See [`supplychain/`](supplychain/).
  Roadmap: Phase 4.
- **Local runtime** — Docker Compose (`make compose-up`) as the default stack:
  OPA, gateway, worker, pinned images, loopback-only ports. A kind path
  (`make kind-demo`) exists for rehearsal/smoke only and is frozen as-is by the
  2026-07-25 operator ruling.
- **CI** — `.github/workflows/ci.yml` on push to `main` and on pull requests:
  `go vet` + `go test` for both services, builds for four of the five tools,
  `tools/schema-check` against both schemas and their sample payloads, and
  `opa test` over the Rego policies.

The repo has one tag, `v0.2.0`. There is no `v0.1.0` tag; the 0.1.0 entry
describes the substrate as it stood at `2dcf795`.

[Unreleased]: https://github.com/artemis-owl/trust-steering/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/artemis-owl/trust-steering/releases/tag/v0.2.0

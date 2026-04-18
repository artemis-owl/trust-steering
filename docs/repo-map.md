# Repo map

This repository is organized to support incremental, local-only work on **DecisionOps / proof-carrying operations**.

## Directories
- `docs/` — reading corpus, roadmap, `runbook.md`, narrative specs, and “audit packet” definition
- `specs/` — formal specs (optional) for critical invariants/workflows
- `infra/` — local environment and deployment scaffolding (**Docker Compose** primary stack in `infra/compose/`, kind manifests for parity, k3d/Helm/GitOps later)
- `services/` — runnable components (gateway and worker)
- `policies/` — OPA/Rego policies, tests, and policy bundles
- `schemas/` — message/event schemas and example payloads
- `observability/` — OpenTelemetry collector config + dashboards
- `supplychain/` — SBOM, signing, attestations, provenance notes/scripts
- `scripts/` — developer workflows (up/down/test/export)
- `tools/` — small helpers (e.g. `tools/schema-check`, `tools/kafka-read-one`)

## Core artifacts (what “done” looks like)
- A schema-defined **Decision Trace** event (`schemas/decision_trace.avsc`)
- A schema-defined **Evidence** event (`schemas/evidence.avsc`)
- Policy decisions are executable + testable (`policies/opa/rego`, `policies/opa/tests`)
- Observability reconstructs the full chain via correlation/trace IDs (`observability/`)
- An exportable “audit packet” bundles evidence for review (`docs/audit-packet.md`, `scripts/export_audit_packet.sh`)
 - AI RMF outcome mapping that ties artifacts to governance (`docs/control-map.md`)

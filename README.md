# trust-steering
steering repo for trust initiative
# steering

Local-first lab repo for trust + technology engineering: policy-bound decisions, traceable operations, and audit-ready evidence—without requiring ongoing cloud spend.

## What we’re building
**Decision Trace Gateway (anchor project)**: a small system that can prove *why* an action happened.

Target flow:
`HTTP request → policy decision (OPA) → decision-trace event → stream (Redpanda/Kafka) → worker action → evidence append → observability (OTel) → exportable audit packet`

## Why local-first (no ongoing cloud spend)
- Fast iteration and tight feedback loops on a single Mac.
- Reproducible demos and evidence artifacts you can run offline.
- Cloud is optional later; this repo is structured so “cloud spend” is a deliberate, justified choice (see `docs/cloud-spend.md`).

## Start here
- Repo map: `docs/repo-map.md`
- 6‑week reading + build plan: `docs/reading-plan-6w.md`
- Roadmap: `docs/roadmap.md`
- Corpus index (NIST-led): `docs/corpus.md`

## Repo layout (high level)
- `docs/` reading plan, specs, roadmap, evidence definitions
- `specs/` formal-ish specs (TLA+ or similar), later
- `infra/` local cluster and deployment manifests (kind/k3d, Helm, GitOps later)
- `services/` gateway + worker services
- `policies/` OPA/Rego policies, tests, bundles
- `schemas/` event schemas + sample payloads
- `observability/` OTel collector config + dashboards
- `supplychain/` SBOM/signing/attestations (SLSA-lite)
- `scripts/` “make it go” scripts (local dev + evidence export)

## Working agreements
- Prefer local tooling and reproducible scripts over manual steps.
- Every “action” should be explainable: policy input → decision → execution → evidence.
- Keep artifacts small and inspectable (JSON events, schema-validated payloads, hashes).

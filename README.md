# Steering: Enterprise DecisionOps Control Plane

The Steering Control Plane provides a local-first reference architecture for **Enterprise DecisionOps**. It demonstrates how AI agents can operate safely by mapping runtime execution context (e.g., test coverage, human approval) to verifiable audit evidence via an immutable policy engine.

## The Architecture
**Decision Trace Gateway (Anchor Project)**: A proxy system that explicitly maps an agent's execution context to policy and proves *why* an action was authorized.

**The Core Flow**:
`AI Agent (MCP Context) → Gateway Manager (OPA) → Decision Trace Event → Stream (Redpanda/Kafka) → Worker Action → Evidence Append → Observability (Jaeger) → Immutable Audit Packet`

## Why local-first (no ongoing cloud spend)
- Fast iteration and tight feedback loops on a single Mac.
- Reproducible demos and evidence artifacts you can run offline.
- Cloud is optional later; this repo is structured so “cloud spend” is a deliberate, justified choice (see `docs/cloud-spend.md`).

## Start here
- Local runbook (modes, ports, troubleshooting): `docs/runbook.md`
- Repo map: `docs/repo-map.md`
- 6‑week reading + build plan: `docs/reading-plan-6w.md`
- Roadmap: `docs/roadmap.md`
- Corpus index (NIST-led): `docs/corpus.md`
- Control map (AI RMF outcomes): `docs/control-map.md`

## Current demo paths
Minimal evidence loop:
`Decision request JSON -> gateway /decide -> Decision Trace -> worker /execute -> Evidence`

**Docker Compose (recommended default):**
```
make compose-up
./scripts/demo_compose.sh
make compose-down
```
See `infra/compose/README.md` for Redpanda (`COMPOSE_PROFILES=stream`) and Kafka env vars.

**Streaming path (Compose + stream profile):** gateway publishes decision traces to `decision.trace.v1`, worker consumes them and publishes evidence to `decision.evidence.v1` when `KAFKA_BOOTSTRAP_SERVERS` is set (the script sets this for containers). Smoke test:

```
COMPOSE_PROFILES=stream ./scripts/compose_up.sh
./scripts/demo_stream.sh
./scripts/compose_down.sh
```

**Compose + traces (Jaeger):** enable the **`obs`** profile so gateway/worker export OTLP to the collector (`compose_up.sh` sets `OTEL_EXPORTER_OTLP_ENDPOINT` when `obs` is in `COMPOSE_PROFILES` or you pass e.g. `--profile obs`). For a **single trace** across gateway → Kafka → worker, use **`COMPOSE_PROFILES=stream,obs`**. Open http://127.0.0.1:16686 and search by service or tags (`steering.correlation_id`, `policy.decision`). Integration check: `make test-obs` (Docker). Details: [`docs/runbook.md`](docs/runbook.md), [`observability/README.md`](observability/README.md).

Local **process** demo (no containers; gateway on port `8081` so it does not conflict with Compose on `8080`):
```
./scripts/demo_local.sh
```

In-cluster kind smoke demo (includes OPA, gateway, worker, OTel collector, and Jaeger with traces emitted over OTLP and UI forwarded to 16686):
```
make kind-demo
```

Compose and kind use OPA in-container; the local process path prefers a host `opa` binary when available and otherwise uses the gateway’s explicit bootstrap fallback when `ALLOW_LOCAL_POLICY_FALLBACK=true`. Demos validate emitted JSON against the Avro schemas where `tools/schema-check` runs.

## Repo layout (high level)
- `docs/` reading plan, specs, roadmap, evidence definitions
- `specs/` formal-ish specs (TLA+ or similar), later
- `infra/` Docker Compose stack (`infra/compose/`), kind manifests, later k3d/Helm/GitOps
- `services/` gateway + worker services
- `policies/` OPA/Rego policies, tests, bundles
- `schemas/` event schemas + sample payloads
- `observability/` OTel collector config + dashboards
- `supplychain/` SBOM/signing/attestations (SLSA-lite)
- `scripts/` “make it go” scripts (local dev + evidence export)
- `tools/` schema validation and one-shot Kafka helpers (`tools/kafka-read-one`)

## Working agreements
- Prefer local tooling and reproducible scripts over manual steps.
- Every “action” should be explainable: policy input → decision → execution → evidence.
- Keep artifacts small and inspectable (JSON events, schema-validated payloads, hashes).

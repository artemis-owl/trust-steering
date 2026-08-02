# Infra

Local-first environment scaffolding:

- **`infra/compose/`** — primary stack: Docker Compose (`make compose-up` from repo root). OPA + gateway + worker; optional Redpanda (`COMPOSE_PROFILES=stream`). See `infra/compose/README.md`.
- **`infra/kind/`** — Kubernetes parity smoke tests (`make kind-demo`), not required for daily dev.
- **`infra/k3d/`** — placeholder; frozen by decision (2026-07-25 charter ruling: kind
  parity is rehearsal-only, no further Kubernetes-path investment).

GitOps (Argo CD/Flux) and Helm charts are **deferred by decision** (same 2026-07-25
ruling) — not planned work; revisiting requires a new operator ruling.

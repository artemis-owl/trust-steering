# Infra

Local-first environment scaffolding:

- **`infra/compose/`** — primary stack: Docker Compose (`make compose-up` from repo root). OPA + gateway + worker; optional Redpanda (`COMPOSE_PROFILES=stream`). See `infra/compose/README.md`.
- **`infra/kind/`** — Kubernetes parity smoke tests (`make kind-demo`), not required for daily dev.
- **`infra/k3d/`** — placeholder for an optional k3d path later.

GitOps (Argo CD/Flux) and Helm charts can be added later under `infra/`.

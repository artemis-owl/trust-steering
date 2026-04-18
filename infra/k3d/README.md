# k3d (planned)

Placeholder for local Kubernetes cluster instructions and manifests.

**Today:** the primary local stack is **Docker Compose** (`infra/compose/`, `make compose-up`). Kind is used for parity smoke tests (`make kind-demo`).

Target later:
- `scripts/up.sh` or k3d equivalent creates cluster
- local registry (optional)
- deploy stream (Redpanda), OPA, OTel collector, and the services


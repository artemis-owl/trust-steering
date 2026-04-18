# kind

This directory contains a minimal kind cluster configuration for **Kubernetes parity** smoke tests. For day-to-day development, prefer **Docker Compose** (`infra/compose/`, `make compose-up`).

## Create cluster
```
KIND_CLUSTER_NAME=steering ./scripts/up.sh
```

## Delete cluster
```
KIND_CLUSTER_NAME=steering ./scripts/down.sh
```

## Notes
- The config is intentionally minimal (single control-plane node).
- Service manifests live under `infra/kind/manifests/`.

## One-command smoke demo
```
make kind-demo
```

This flow:
- creates or reuses the kind cluster
- builds and loads the local images
- deploys OPA, gateway, worker, OTel collector, and Jaeger
- runs the Decision Trace -> Evidence smoke check (with traces emitted to Jaeger over OTLP)
- writes logs and JSON outputs to a temp directory

## Manual deploy steps
1. Create the cluster:
```
KIND_CLUSTER_NAME=steering ./scripts/up.sh
```
2. Build and load local images:
```
KIND_CLUSTER_NAME=steering ./scripts/kind_load.sh
```
3. Apply manifests:
```
./scripts/kind_deploy.sh
```

Included manifests:
- `namespace.yaml`
- `opa.yaml`
- `gateway.yaml`
- `worker.yaml`
- `jaeger.yaml`
- `otel-collector.yaml`

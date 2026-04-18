# Observability

OpenTelemetry (OTel) is **evidence plumbing**: traces carry `steering.correlation_id` and `policy.decision` as span attributes so operators can find denied flows and follow gateway → (Kafka) → worker.

## Local stack (Compose)

Enable the **`obs`** profile so gateway/worker export OTLP to the collector and traces land in **Jaeger**:

```sh
COMPOSE_PROFILES=obs ./scripts/compose_up.sh
```

*(Note: Tracing is also enabled by default when running the Kubernetes smoke tests via `make kind-demo`, tracking identically inside the cluster).*

| Port (loopback) | Purpose |
|-----------------|--------|
| **4318** | OTLP HTTP (collector); set `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318` for services (Compose sets this when `obs` is active) |
| **16686** | Jaeger UI: http://127.0.0.1:16686 |

`compose_up.sh` exports `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318` when `COMPOSE_PROFILES` includes `obs` or a Compose arg matches `obs` (e.g. `--profile obs`). Override only if you run a different collector address.

Collector config: [`otel-collector.yaml`](otel-collector.yaml) (OTLP in → batch → Jaeger OTLP gRPC).

## Querying traces (“evidence”)

In Jaeger **Search**:

- **Service**: `steering-gateway`, `steering-worker`, or child spans (e.g. OPA client, `kafka.process_decision_trace`).
- **Tags**: `steering.correlation_id=<value>`, `policy.decision=deny` (or `allow`).

HTTP clients may send **`traceparent`**; the gateway/worker join that trace. Domain JSON `trace_id` / `span_id` align with the active OTel span when a span context exists (otherwise generated hex IDs as before).

## Without Compose

If `OTEL_EXPORTER_OTLP_ENDPOINT` is unset, services use a **no-op** tracer (tests and `demo_local.sh` need no collector).

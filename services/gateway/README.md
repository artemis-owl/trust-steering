# Gateway

Responsibilities:
- authenticate/identify subject
- construct policy input from request attributes
- call OPA for allow/deny + rationale (or local fallback)
- emit a Decision Trace event (JSON) and optionally publish it to Kafka
- ensure correlation ids and trace context are propagated

Interfaces:
- HTTP API: `POST /decide` (returns decision trace JSON)
- Kafka: producer to `decision.trace.v1` when `KAFKA_BOOTSTRAP_SERVERS` is set (failure to publish returns 503)

Run locally:
```
cd services/gateway
OPA_URL=http://localhost:8181 \
POLICY_BUNDLE_HASH=sha256:$(shasum -a 256 ../../policies/opa/rego/decision.rego | awk '{print $1}') \
POLICY_BUNDLE_VERSION=0.1.0 \
go run .
```

Optional env:
- `OPA_URL` (e.g., `http://localhost:8181`)
- `POLICY_BUNDLE_HASH`, `POLICY_BUNDLE_VERSION`
- `SCHEMA_VERSION`
- `ALLOW_LOCAL_POLICY_FALLBACK=true` to use the in-process bootstrap rule when OPA is unavailable
- `KAFKA_BOOTSTRAP_SERVERS` — comma-separated brokers; omit to disable publishing
- `KAFKA_TOPIC_DECISION_TRACE` — default `decision.trace.v1`

Sample request:
- `schemas/examples/decision_request.sample.json`

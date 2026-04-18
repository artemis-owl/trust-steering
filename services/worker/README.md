# Worker

Responsibilities:
- consume Decision Trace events (from Kafka when configured, or via HTTP)
- execute/skip based on decision
- emit Evidence events (success/failure/side effects)
- preserve correlation ids and trace context

Interfaces:
- HTTP API: `POST /execute` (accepts decision trace JSON, returns evidence)
- Kafka: consumer on `decision.trace.v1`, producer to `decision.evidence.v1` when `KAFKA_BOOTSTRAP_SERVERS` is set

Run locally:
```
cd services/worker
go run .
```

Optional env:
- `SERVICE_NAME`, `SERVICE_VERSION`, `IMAGE_DIGEST`
- `EVIDENCE_SCHEMA_VERSION`
- `KAFKA_BOOTSTRAP_SERVERS` — omit to disable streaming (HTTP only)
- `KAFKA_TOPIC_DECISION_TRACE` — default `decision.trace.v1`
- `KAFKA_TOPIC_EVIDENCE` — default `decision.evidence.v1`
- `KAFKA_CONSUMER_GROUP` — default `steering-worker`

Input expectations:
- requires a well-formed Decision Trace with identity, correlation, request, and policy metadata
- rejects traces missing `policy.engine`, `policy.bundle_hash`, or `policy.decision`

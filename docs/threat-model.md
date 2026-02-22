# Threat model (starter)

This is a lightweight, iterative threat model for the Decision Trace Gateway system. Keep it short and update as the system evolves.

## System overview
Actors request an action; the gateway evaluates policy, emits a decision trace, publishes to a stream, a worker executes, and emits evidence. Observability provides reconstruction; an audit packet exports evidence.

## Assets
- Decision Trace events (authorization + rationale evidence)
- Evidence events (execution evidence)
- Policy bundles (the “law” that decides)
- Schema definitions (contract for what evidence means)
- Observability data (traces/logs/metrics)
- Signing keys / verification material (when supplychain is enabled)

## Trust boundaries
- External client → gateway (authentication, input validation)
- Gateway → policy engine (OPA) (decision integrity)
- Gateway/worker → stream (topic ACLs, schema enforcement)
- Worker → action target (idempotency, authorization)
- Services → observability backends (data integrity, access controls)
- Export tooling → audit packet output (tamper resistance)

## Threats (STRIDE-ish)
- **Spoofing**: forged identity/claims in requests.
- **Tampering**: modifying events in transit or at rest; altering policy bundles.
- **Repudiation**: actor denies they requested an action; missing correlation.
- **Information disclosure**: leaking PII/secrets in traces, logs, or events.
- **Denial of service**: policy engine or stream overwhelmed, causing “fail open” behavior.
- **Elevation of privilege**: bypassing policy checks or mis-scoped worker permissions.

## Mitigation themes (engineering targets)
- Authenticate requests and record verified identity attributes.
- Content-address policy bundles; include hashes in Decision Trace/Evidence.
- Validate and sign evidence bundles (later: signatures/attestations).
- Data minimization: classify fields; avoid raw payload logging.
- Make denial the default; avoid “fail open” paths.
- Idempotency keys and replay protection where appropriate.


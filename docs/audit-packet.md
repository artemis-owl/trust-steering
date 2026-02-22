# Audit Packet

An **audit packet** is an exportable bundle that allows a third party to validate:
“This action was authorized by this policy, executed by this workload, and is fully traceable.”

## What an audit packet contains
Minimum contents for a given `correlation_id` (or time window):
- **Decision Trace events** (schema-validated)
- **Evidence events** produced by execution (schema-validated)
- **Policy bundle identity**: version + hash, and the policy inputs used (redacted/minimized if needed)
- **Schema identities**: schema versions/hashes for the events included
- **Observability pointers**: trace IDs, log query hints, metric names (avoid huge raw dumps by default)
- **Supply chain evidence (when available)**:
  - SBOM for each deployed artifact
  - signature verification output
  - provenance/attestation references tied to a commit/build recipe

## Completeness criteria
An audit packet is “complete” when a reviewer can:
- Validate payloads against the schemas in `schemas/`
- Validate the policy identity (exact policy bundle hash/version)
- Reconstruct the causal chain using `correlation_id` and `trace_id`
- Confirm the executing workload identity and its artifact provenance (when supplychain is enabled)

## Output shape (suggested)
`export_audit_packet.sh` should produce:
- `out/audit-packets/<correlation_id>/decision_traces.jsonl`
- `out/audit-packets/<correlation_id>/evidence.jsonl`
- `out/audit-packets/<correlation_id>/policy/` (bundle hash + metadata)
- `out/audit-packets/<correlation_id>/schemas/` (copies or hashes)
- `out/audit-packets/<correlation_id>/observability/` (trace ids, query hints)
- `out/audit-packets/<correlation_id>/supplychain/` (sbom/signature verification outputs)


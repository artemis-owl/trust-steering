# Decision Trace (narrative schema)

The Decision Trace is the primary evidence object in this repo: a compact, schema-validated event that records **what was requested**, **what policy decided**, **why**, and **what happened next**, with enough metadata to reconstruct the full chain in observability and in an exported audit packet.

## Design goals
- **Reconstructable**: a reviewer can follow request → decision → execution → evidence.
- **Reproducible**: given the same inputs + policy version + schema version, the decision is explainable and stable.
- **Minimal**: log what you need to explain decisions; avoid unnecessary payloads/PII.
- **Correlatable**: every event ties into traces/logs/metrics with IDs that flow across async boundaries.

## Required fields (conceptual)
- **Identity / actor**: who requested (subject), what service is acting (principal), and on whose behalf.
- **Request**: action, resource, environment, and relevant attributes (ABAC-style).
- **Policy**: policy engine, policy version/bundle hash, decision (allow/deny), rationale codes/messages.
- **Timing**: event time, request time, decision time (as needed).
- **Correlation**: `correlation_id` (stable business/workflow id), `trace_id` (OTel trace), and optional `span_id`.
- **Outcome**: downstream action reference (e.g., “published to topic X”), and links to follow-on evidence.
- **Data classification**: classification + PII flags for minimization and handling rules.

## Invariants (non-negotiable)
1. **No evidence without a Decision Trace**: the worker records evidence only for a trace it is sent. It accepts any well-formed trace and does not check where it came from (that the gateway issued it); it executes nothing, it records.
2. **Every action is traceable**: every Decision Trace includes a `correlation_id` and a `trace_id`.
3. **Decisions are versioned**: Decision Trace records `policy.bundle_version` and `policy.bundle_hash`. `bundle_hash` is the sha256 of `decision.rego`, computed by the script that launches the gateway (default `local-dev` if unset); `roles.json` is not in it.
4. **Schema-validated events**: Decision Trace and Evidence events are validated against `schemas/` in CI (on the sample payloads) and by the demo scripts (on their output). The services do not validate at runtime.

## Versioning strategy
- Event schemas are versioned independently (e.g., `schema_version` field + compatibility rules).
- The policy hash covers `decision.rego` only (sha256, computed by the launching script); `roles.json` is not hashed, so the hash alone does not pin the roles that were in force.
- Services record their build/provenance identifiers in Evidence events.


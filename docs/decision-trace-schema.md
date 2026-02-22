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
1. **No action without policy decision**: if an action executes, a prior Decision Trace must exist.
2. **Every action is traceable**: every Decision Trace includes a `correlation_id` and a `trace_id`.
3. **Decisions are versioned**: Decision Trace records the exact policy bundle version/hash used.
4. **Schema-validated events**: Decision Trace and Evidence events must validate against schemas in `schemas/`.

## Versioning strategy
- Event schemas are versioned independently (e.g., `schema_version` field + compatibility rules).
- Policy bundles are versioned and content-addressed (hash) so “what policy ran” is unambiguous.
- Services record their build/provenance identifiers in Evidence events.


# Schemas

This directory holds the message/event contracts used across the system:
- Decision Trace events (authorization + rationale)
- Evidence events (execution evidence and provenance pointers)

Schemas are intended to be:
- versioned (explicit `schema_version`)
- validated in CI (sample payloads in `schemas/examples/`) and by the demo scripts on their output; the services do not validate at runtime
- evolved with compatibility discipline

See `docs/decision-trace-schema.md` for the narrative spec.


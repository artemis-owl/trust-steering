# Roadmap

This roadmap is organized around one anchor demo (“Decision Trace Gateway”) and the evidence you want to be able to hand to a reviewer.

## Phase 0 — Repo bootstrap (now)
- Create an opinionated layout for policies, schemas, observability, and supply chain evidence.
- Write the narrative specs: Decision Trace and Audit Packet.

## Phase 1 — Decision trace + policy loop
- Define Decision Trace and Evidence events + examples.
- Implement OPA policy decisions with rationale and policy versioning.
- Ensure “no action without a policy decision” is enforceable and testable.

## Phase 2 — Streaming contracts + evidence append
- Publish decision-trace events to a topic boundary.
- Consume and execute in a worker, emitting evidence events.
- Add schema validation and compatibility checks as gates.

## Phase 3 — Observability as evidence
- Add OpenTelemetry instrumentation + collector.
- Make correlation IDs and trace IDs first-class across async boundaries.
- Provide “evidence queries”: find denied decisions, reconstruct a full chain.

## Phase 4 — Provenance and software supply chain evidence
- SBOM per artifact.
- Sign and verify artifacts.
- Attach attestations tied to a commit and a build recipe.

## Phase 5 — Audit packet export + change management
- Export evidence for a correlation_id/window: events, policy bundle hash, schema versions, provenance proofs.
- Add change gates and lightweight risk notes for policy/schema changes.


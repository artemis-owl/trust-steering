# 6‑week local-only reading + build plan (NIST-led)

Goal: end with a runnable local demo that produces audit‑ready evidence:
`HTTP → policy decision → decision trace event → stream → worker action → evidence event → OTel trace/logs/metrics → exportable audit packet`

## Week 1 — Skeleton + Decision Trace v0.1
**Reading**
- NIST AI RMF 1.0 + Playbook: focus on Govern/Map/Measure/Manage and “evidence”
- NIST CSF 2.0: outcomes language + continuous improvement loop

**Build**
- Create repo scaffolding and a narrative spec for the Decision Trace.
- Define required fields + invariants.

**Deliverables**
- `docs/decision-trace-schema.md`
- `schemas/decision_trace.avsc` and `schemas/evidence.avsc` (initial versions)

## Week 2 — Policy-as-code decisioning (OPA) + tests
**Reading**
- NIST RMF (SP 800-37): continuous monitoring mindset
- NIST SP 800-30: risk assessment framing for “deny/allow with rationale”

**Build**
- Add OPA/Rego policy for basic allow/deny with rationale.
- Add policy unit tests and a “policy diff” workflow in CI.

**Deliverables**
- `policies/opa/rego/decision.rego`
- `policies/opa/tests/decision_test.rego`

## Week 3 — Streaming governance: contracts “in motion”
**Reading**
- NIST SP 800-53: focus on audit, access enforcement, config management, integrity
- NIST Privacy Framework: data minimization as an engineering requirement

**Build**
- Stand up a local Kafka-compatible broker (Redpanda recommended later).
- Enforce schema validation for produced/consumed events (CI + runtime).

**Deliverables**
- `schemas/examples/*.json`
- `scripts/test.sh` grows to include schema checks

## Week 4 — Evidence-grade observability (OTel) end-to-end
**Reading**
- NIST SP 800-207 (Zero Trust): explicit policy enforcement + continuous verification
- SRE selected: SLIs/SLOs and incident response for socio-technical systems

**Build**
- Propagate trace context across HTTP → stream headers → worker.
- Add OTel Collector + local tracing/logging/metrics backends.

**Deliverables**
- `observability/otel-collector.yaml`
- `observability/dashboards/` (starter dashboards, later)

## Week 5 — Supply chain provenance: SBOM + signing + attestations
**Reading**
- NIST SSDF (SP 800-218): “prove you built it securely”
- NIST SP 800-161: supply chain risk and provenance

**Build**
- Build local images.
- Generate SBOMs and attach attestations; verify signatures locally.

**Deliverables**
- `supplychain/sbom/README.md`
- `supplychain/signing/README.md`

## Week 6 — Audit Packet export + change gates + threat model
**Reading**
- Revisit AI RMF Playbook: map system to Govern/Map/Measure/Manage
- Optional: one formal spec for a critical invariant (TLA+ or property-based tests)

**Build**
- Export an “audit packet” by correlation_id/time window.
- Add lightweight change gates: policy/schema changes require tests + a short risk note.
- Write a short threat model.

**Deliverables**
- `docs/audit-packet.md`
- `docs/threat-model.md`
- `scripts/export_audit_packet.sh`


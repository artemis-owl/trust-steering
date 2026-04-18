# Control map (AI RMF–anchored)

This table maps **AI RMF outcome areas** to concrete repo artifacts and evidence. It is designed to drive **weekly build artifacts** and ensure everything we build can be explained as “governance/evidence” for AI‑related systems.

> Note: this is intentionally outcome‑level (Govern/Map/Measure/Manage). We can add precise AI RMF sub‑outcome codes later once the system has stable artifacts.

## Mapping table (starter)
| AI RMF outcome area | Intent | Repo artifact(s) | Evidence produced |
|---|---|---|---|
| **Govern** | Clear decision authority and policy enforcement | `policies/opa/rego/decision.rego`, `policies/opa/tests/` | Policy bundle hash + test results; Decision Trace rationale |
| **Govern** | Change management for policies/schemas | `scripts/test.sh`, `docs/reading-plan-6w.md` | Failing checks on incompatible schema/policy changes (when implemented) |
| **Map** | System context, decision scope, and boundaries | `docs/decision-trace-schema.md`, `docs/threat-model.md` | Narrative definition of what is decided and why |
| **Map** | Data classification and minimization | `schemas/decision_trace.avsc`, `schemas/evidence.avsc` | Explicit fields for data_classification + pii flags |
| **Measure** | Evidence‑grade traceability | `observability/otel-collector.yaml`, `docs/audit-packet.md` | Correlation + trace IDs; audit packet completeness criteria |
| **Measure** | Robustness/consistency of decisions | `policies/opa/tests/decision_test.rego` | Policy unit test outcomes |
| **Manage** | Ongoing monitoring and incident response readiness | `docs/roadmap.md`, `observability/` | Evidence queries and trace reconstruction path |
| **Manage** | Provenance and software supply chain integrity | `supplychain/` | SBOM + signing artifacts (when enabled) |

## Weekly build artifacts (how to use this)
- Each week’s deliverable in `docs/reading-plan-6w.md` should update at least one row above.
- If an artifact adds new evidence, add a row with the **outcome area + intent + proof**.


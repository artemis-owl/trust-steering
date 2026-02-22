# Corpus (NIST-led) + supporting shelf

This is the curated reading spine for the trust + technology engineering domain of this repo.

## Primary spine (NIST)
- NIST AI RMF 1.0 + AI RMF Playbook
- NIST Cybersecurity Framework (CSF) 2.0
- NIST Risk Management Framework (SP 800-37)
- NIST Risk Assessments (SP 800-30)
- NIST Security and Privacy Controls (SP 800-53)
- NIST Secure Software Development Framework (SSDF) (SP 800-218)
- NIST Supply Chain Risk Management (SP 800-161)
- NIST Zero Trust Architecture (SP 800-207)
- NIST Digital Identity Guidelines (SP 800-63, incl. 63A/63B/63C and newer revisions if applicable)
- NIST Privacy Framework

## Supporting shelf (engineering + theory)
- Google SRE (selected chapters)
- Lamport, *Specifying Systems* (TLA+)
- Pearl, *Causality* (selected concepts: counterfactuals and explanations)
- “Zanzibar-ish” authorization concepts (relationship/attribute-based access control)

## How to use this corpus here
- Each build artifact in `services/`, `policies/`, `schemas/`, and `supplychain/` should map to an evidence need in `docs/audit-packet.md`.
- If you want a formal mapping table later, add `docs/control-map.md` (anchor on AI RMF outcomes or 800‑53 families).


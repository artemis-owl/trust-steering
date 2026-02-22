# OPA policies

OPA/Rego policies define **what is allowed** and **why**. Policies should be:
- testable (unit tests in `policies/opa/tests/`)
- bundle-able and content-addressable (hashable bundle output)
- explainable (return rationale codes/messages, not just booleans)

Starter policy lives in `policies/opa/rego/decision.rego`.


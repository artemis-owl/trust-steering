# Cloud spend policy (personal)

This repo is intentionally **local-first**. The default expectation is **$0 ongoing cloud spend**.

## Default stance
- Build, run, and demo everything on a single Mac using local containers/Kubernetes.
- Treat cloud usage as an exception with an explicit reason and a time-bound plan.

## When cloud spend is justified
Cloud spend is reasonable only when it unlocks something you *cannot* get locally with similar learning value, for example:
- Multi-region, real DNS/certs, or real identity integrations that require public endpoints
- Managed service semantics you’re specifically studying (e.g., IAM policy edge cases)
- Performance/scale tests that exceed local hardware constraints

## Guardrails
- Require a written justification PR note: *why now*, *expected cost/month*, *kill switch date*
- Prefer free tiers, credits, or ephemeral pay-as-you-go experiments with hard caps
- Keep cloud-only configuration isolated under `infra/cloud/` (create only if needed)


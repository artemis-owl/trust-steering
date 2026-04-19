# Enterprise Build Pipeline Strategy

This document provides a conceptual mapping between our **local provenance scripts** (`scripts/generate_sbom.sh`, `scripts/sign_images.sh`, etc.) and how this translates into a standard **Enterprise CI/CD environment** (e.g., GitLab Pipelines, GitHub Actions).

## Objective

Our local environment uses basic tools (`syft` and `cosign` with local keys) to fulfill the requirements of NIST SSDF (SP 800-218). In an enterprise context, these responsibilities shift from local files to identity providers and managed key infrastructures (like Sigstore or HashiCorp Vault).

## Pipeline Mapping

| Local Execution | Enterprise Pipeline (e.g. GitLab CI) | Enterprise Difference |
|-----------------|---------------------------------|-----------------------|
| `generate_sbom.sh` | A build stage job running the `anchore/syft:latest` image. | The generated `.spdx.json` is exported as a CI/CD job artifact to be archived or ingested by a security dashboard. |
| `sign_images.sh` | A publish stage job that pushes to the Enterprise OCI Registry (e.g. AWS ECR, GitLab Registry). | **Keyless Signing:** Instead of generating `.keys/cosign.key`, the CI job uses the platform's OIDC token (e.g. `CI_JOB_JWT`) to authenticate with Sigstore's Fulcio. The signature is bound to the identity of the CI runner and the specific Git commit, not a static private key. |
| `verify_provenance.sh`| Executed by Kubernetes Admission Controllers (e.g., Kyverno). | The K8s cluster enforces a policy: *No pod can start unless its image is signed, and the `verify-attestation` step ensures the SBOM has no critical vulnerabilities.* |
| `export_audit_packet.sh` | A scheduled or release-triggered artifact packaging job. | The ZIP file is pushed to an immutable S3 bucket or compliant object store for long-term audit retention. |

## Why Start Local?

By establishing these scripts locally, we establish the **developer contract**:
- We know exactly how SBOMs are shaped.
- We know the commands to attach attestations.
- We integrate the proofs immediately into the local "Audit Packet" before spending cycles configuring cloud IAM and Managed PKI.

This local parity ensures that when this workload is lifted to an enterprise CI system, the architectural foundation of "Supply Chain Provenance" is already proven.

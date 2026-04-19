# Supply chain evidence

This area manages the automated SLSA-lite software supply chain pipeline for the Steering architecture:
- **SBOM Generation**: Uses `scripts/generate_sbom.sh` to extract SPDX JSON info via Syft (`supplychain/sbom/out/`).
- **Signing & Attestation**: Uses `scripts/sign_images.sh` to cryptographically sign images via Cosign and attach the SBOM as an attestation.
- **Verification**: Uses `scripts/verify_provenance.sh` to pull and verify signatures.

These outputs are packaged into the final Audit Packet.


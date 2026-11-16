# Supply chain evidence

This area holds local supply-chain scripts for the gateway and worker images. They need local images tagged `steering-gateway:dev` and `steering-worker:dev`, built by `scripts/kind_load.sh` (which needs an existing kind cluster).
- **SBOM Generation**: Uses `scripts/generate_sbom.sh` to extract SPDX JSON info via Syft (`supplychain/sbom/out/`).
- **Signing & Attestation**: Uses `scripts/sign_images.sh` to sign images via Cosign and attach the SBOM as an attestation. Nothing ties an attestation to a commit or build recipe.
- **Verification**: Uses `scripts/verify_provenance.sh` to pull and verify signatures.

`scripts/export_audit_packet.sh` copies the cosign public key and the SBOM files into a packet when they exist. It verifies no signature and copies no attestation.


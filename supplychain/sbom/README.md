# Software Bill of Materials (SBOM)

This directory manages the generation of SBOMs for the core Steering artifacts (`steering-gateway` and `steering-worker`).

## Purpose
An SBOM is essentially a rigorous "ingredients list" for software. It enumerates every dependency, library, and module compiled into the final image. Generating an SBOM fulfills a fundamental requirement of modern software supply chain security (e.g., NIST SSDF SP 800-218).

When an AI action is evaluated by the gateway, the auditor must know that the gateway itself didn't harbor a critical CVE.

## Implementation Details
We use [Syft](https://github.com/anchore/syft) to scan our Go binaries and Docker images.
Syft outputs standard `SPDX JSON` formatting, which is the industry standard format easily ingested by vulnerability scanners.

To generate the SBOMs locally (needs `syft` and local images tagged `steering-gateway:dev` and `steering-worker:dev`, built by `scripts/kind_load.sh`):
```bash
./scripts/generate_sbom.sh
```

The resulting `gateway.spdx.json` and `worker.spdx.json` files are automatically written to `supplychain/sbom/out/`.

## Integration with the Audit Packet
`scripts/sign_images.sh` attaches the SBOM JSON to the image as a Cosign attestation (see `supplychain/signing/README.md`); nothing ties that attestation to a commit or build recipe. `scripts/export_audit_packet.sh` copies the SBOM files (and the cosign public key, if present) into the packet directory for one correlation_id. It makes no zip and does not check that those images produced the decisions in the packet.

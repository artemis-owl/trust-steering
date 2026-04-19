# Software Bill of Materials (SBOM)

This directory manages the generation of SBOMs for the core Steering artifacts (`steering-gateway` and `steering-worker`).

## Purpose
An SBOM is essentially a rigorous "ingredients list" for software. It enumerates every dependency, library, and module compiled into the final image. Generating an SBOM fulfills a fundamental requirement of modern software supply chain security (e.g., NIST SSDF SP 800-218).

When an AI action is evaluated by the gateway, the auditor must know that the gateway itself didn't harbor a critical CVE.

## Implementation Details
We use [Syft](https://github.com/anchore/syft) to scan our Go binaries and Docker images.
Syft outputs standard `SPDX JSON` formatting, which is the industry standard format easily ingested by vulnerability scanners.

To generate the SBOMs locally:
```bash
./scripts/generate_sbom.sh
```

The resulting `gateway.spdx.json` and `worker.spdx.json` files are automatically written to `supplychain/sbom/out/`.

## Integration with the Audit Packet
SBOMs alone are useful, but their real value is when they are **attested** and exported. The SBOM JSON is attached cryptographically to the image via Cosign (see `supplychain/signing/README.md`). Finally, `scripts/export_audit_packet.sh` zips these files together with the decision traces to provide a holistic compliance handshake.

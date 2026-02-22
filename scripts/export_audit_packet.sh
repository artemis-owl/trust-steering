#!/usr/bin/env bash
set -euo pipefail

correlation_id="${1:-}"
if [[ -z "$correlation_id" ]]; then
  echo "Usage: ./scripts/export_audit_packet.sh <correlation_id>" >&2
  exit 2
fi

out_dir="out/audit-packets/${correlation_id}"
mkdir -p "$out_dir"

cat >"${out_dir}/README.txt" <<EOF
Audit packet export is not implemented yet.

Intended contents (see docs/audit-packet.md):
- decision_traces.jsonl
- evidence.jsonl
- policy bundle hash/version
- schema identities
- observability pointers
- supplychain proofs (SBOM/signature verification) when available
EOF

echo "Wrote placeholder audit packet to: ${out_dir}"


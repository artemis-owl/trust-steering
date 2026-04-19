#!/usr/bin/env bash
set -euo pipefail

correlation_id="${1:-}"
if [[ -z "${correlation_id}" ]]; then
  echo "Usage: ./scripts/export_audit_packet.sh <correlation_id>" >&2
  echo "  Kafka: set KAFKA_BOOTSTRAP_SERVERS (e.g. 127.0.0.1:19092 with Compose stream profile)." >&2
  echo "  Files: set AUDIT_DECISION_TRACE_FILE and AUDIT_EVIDENCE_FILE (HTTP demos; no broker)." >&2
  exit 2
fi

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export GOCACHE="${GOCACHE:-${root_dir}/.gocache}"
export GOMODCACHE="${GOMODCACHE:-${root_dir}/.gomodcache}"
mkdir -p "${GOCACHE}" "${GOMODCACHE}"
out_dir="${root_dir}/out/audit-packets/${correlation_id}"
policy_file="${root_dir}/policies/opa/rego/decision.rego"
schema_decision="${root_dir}/schemas/decision_trace.avsc"
schema_evidence="${root_dir}/schemas/evidence.avsc"

for cmd in python3 shasum; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} is required for scripts/export_audit_packet.sh" >&2
    exit 1
  fi
done
if ! command -v go >/dev/null 2>&1; then
  echo "go is required for scripts/export_audit_packet.sh" >&2
  exit 1
fi

if [[ ! -f "${policy_file}" ]]; then
  echo "Missing policy file: ${policy_file}" >&2
  exit 1
fi

tmp_dec=""
tmp_ev=""
tmpdir=""
rm_tmpdir() {
  if [[ -n "${tmpdir}" && -d "${tmpdir}" ]]; then
    rm -rf "${tmpdir}"
  fi
}
trap rm_tmpdir EXIT

verify_corr_in_file() {
  local f="$1"
  python3 -c "
import json, sys
with open(sys.argv[1]) as fp:
    d = json.load(fp)
c = d.get('correlation_id')
if c != sys.argv[2]:
    print('correlation_id mismatch in %s: expected %r, got %r' % (sys.argv[1], sys.argv[2], c), file=sys.stderr)
    sys.exit(1)
" "${f}" "${correlation_id}"
}

json_one_line() {
  python3 -c "import json,sys; print(json.dumps(json.load(open(sys.argv[1])), separators=(',', ':')))" "$1"
}

if [[ -n "${AUDIT_DECISION_TRACE_FILE:-}" && -n "${AUDIT_EVIDENCE_FILE:-}" ]]; then
  if [[ ! -f "${AUDIT_DECISION_TRACE_FILE}" || ! -f "${AUDIT_EVIDENCE_FILE}" ]]; then
    echo "AUDIT_DECISION_TRACE_FILE and AUDIT_EVIDENCE_FILE must be existing files" >&2
    exit 1
  fi
  verify_corr_in_file "${AUDIT_DECISION_TRACE_FILE}"
  verify_corr_in_file "${AUDIT_EVIDENCE_FILE}"
  tmp_dec="${AUDIT_DECISION_TRACE_FILE}"
  tmp_ev="${AUDIT_EVIDENCE_FILE}"
elif [[ -n "${KAFKA_BOOTSTRAP_SERVERS:-}" ]]; then
  tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/steering-audit-fetch.XXXXXX")"
  tmp_dec="${tmpdir}/decision.json"
  tmp_ev="${tmpdir}/evidence.json"
  export CORRELATION_ID="${correlation_id}"
  (
    cd "${root_dir}/tools/audit-packet-fetch"
    go run . \
      -correlation-id="${correlation_id}" \
      -decision-out="${tmp_dec}" \
      -evidence-out="${tmp_ev}"
  )
else
  echo "Either set KAFKA_BOOTSTRAP_SERVERS or both AUDIT_DECISION_TRACE_FILE and AUDIT_EVIDENCE_FILE" >&2
  exit 1
fi

(
  cd "${root_dir}/tools/schema-check"
  go run . "${schema_decision}" "${tmp_dec}" >/dev/null
  go run . "${schema_evidence}" "${tmp_ev}" >/dev/null
)

mkdir -p "${out_dir}/policy" "${out_dir}/schemas" "${out_dir}/observability" "${out_dir}/supplychain"

json_one_line "${tmp_dec}" >"${out_dir}/decision_traces.jsonl"
json_one_line "${tmp_ev}" >"${out_dir}/evidence.jsonl"

cp "${policy_file}" "${out_dir}/policy/decision.rego"
export BUNDLE_HASH="sha256:$(shasum -a 256 "${policy_file}" | awk '{print $1}')"
export BUNDLE_VERSION="${POLICY_BUNDLE_VERSION:-0.1.0}"
export EXPORT_OUT_DIR="${out_dir}"
python3 -c "
import json, os
path = os.path.join(os.environ['EXPORT_OUT_DIR'], 'policy', 'metadata.json')
open(path, 'w').write(json.dumps({
    'bundle_hash': os.environ['BUNDLE_HASH'],
    'bundle_version': os.environ['BUNDLE_VERSION'],
    'rego_file': 'decision.rego',
}, indent=2) + '\n')
"

cp "${schema_decision}" "${schema_evidence}" "${out_dir}/schemas/"
(
  cd "${out_dir}/schemas"
  shasum -a 256 decision_trace.avsc evidence.avsc >SHA256SUMS
)

export AUDIT_TMP_DEC="${tmp_dec}"
python3 -c "
import json, os
with open(os.environ['AUDIT_TMP_DEC']) as f:
    d = json.load(f)
jaeger = os.environ.get('JAEGER_UI_URL', 'http://127.0.0.1:16686')
out = os.path.join(os.environ['EXPORT_OUT_DIR'], 'observability', 'pointers.json')
open(out, 'w').write(json.dumps({
    'correlation_id': d.get('correlation_id'),
    'trace_id': d.get('trace_id'),
    'span_id': d.get('span_id'),
    'jaeger_ui': jaeger,
}, indent=2) + '\n')
"

if [ -d "${root_dir}/supplychain/sbom/out" ]; then
  cp -r "${root_dir}/supplychain/sbom/out" "${out_dir}/supplychain/sbom_out"
fi
if [ -f "${root_dir}/.keys/cosign.pub" ]; then
  cp "${root_dir}/.keys/cosign.pub" "${out_dir}/supplychain/"
fi

cat >"${out_dir}/supplychain/README.md" <<EOF
# Supply Chain Evidence
This directory contains the public key (\`cosign.pub\`) used to cryptographically sign the Gateway and Worker images.
If SBOMs were generated, they are included in the \`sbom_out/\` directory as SPDX JSON artifacts.
These attestations mathematically prove the integrity of the binaries enforcing the policy decisions contained in this Audit Packet.
EOF

trap - EXIT
rm_tmpdir

echo "Audit packet written to: ${out_dir}"
echo "Hint: if traces were exported to Jaeger, search ${JAEGER_UI_URL:-http://127.0.0.1:16686} by trace_id in observability/pointers.json or tag steering.correlation_id."

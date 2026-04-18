#!/usr/bin/env bash
set -euo pipefail

# Smoke test against the consolidated Compose stack (./scripts/compose_up.sh).
# Uses gateway on 8080 and worker on 9090 by default.

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${1:-$(mktemp -d "${TMPDIR:-/tmp}/steering-compose-demo.XXXXXX")}"
request_file="${DECISION_REQUEST_FILE:-${root_dir}/schemas/examples/decision_request.sample.json}"
gateway_base="${GATEWAY_URL:-http://127.0.0.1:8080}"
worker_base="${WORKER_URL:-http://127.0.0.1:9090}"

for cmd in go curl; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} is required for scripts/demo_compose.sh" >&2
    exit 1
  fi
done

mkdir -p "${out_dir}"
export GOCACHE="${GOCACHE:-${root_dir}/.gocache}"

curl -fsS \
  -H "Content-Type: application/json" \
  -X POST "${gateway_base}/decide" \
  --data @"${request_file}" \
  >"${out_dir}/decision_trace.json"

curl -fsS \
  -H "Content-Type: application/json" \
  -X POST "${worker_base}/execute" \
  --data @"${out_dir}/decision_trace.json" \
  >"${out_dir}/evidence.json"

(
  cd "${root_dir}/tools/schema-check"
  go run . "${root_dir}/schemas/decision_trace.avsc" "${out_dir}/decision_trace.json" >/dev/null
  go run . "${root_dir}/schemas/evidence.avsc" "${out_dir}/evidence.json" >/dev/null
)

printf 'Compose demo outputs written to %s\n' "${out_dir}"
printf 'Decision Trace: %s\n' "${out_dir}/decision_trace.json"
printf 'Evidence: %s\n' "${out_dir}/evidence.json"

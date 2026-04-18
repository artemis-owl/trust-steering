#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${1:-$(mktemp -d "${TMPDIR:-/tmp}/steering-demo.XXXXXX")}"
request_file="${DECISION_REQUEST_FILE:-${root_dir}/schemas/examples/decision_request.sample.json}"

for cmd in go curl shasum jq; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} is required for scripts/demo_local.sh" >&2
    exit 1
  fi
done

mkdir -p "${out_dir}"

cleanup() {
  for pid in "${WORKER_PID:-}" "${GATEWAY_PID:-}" "${OPA_PID:-}"; do
    if [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1; then
      kill "${pid}" >/dev/null 2>&1 || true
      wait "${pid}" >/dev/null 2>&1 || true
    fi
  done
}
trap cleanup EXIT

wait_for_http() {
  local url="$1"
  local name="$2"
  for _ in $(seq 1 50); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.2
  done
  echo "${name} did not become ready at ${url}" >&2
  exit 1
}

export GOCACHE="${GOCACHE:-${root_dir}/.gocache}"

policy_hash="sha256:$(shasum -a 256 "${root_dir}/policies/opa/rego/decision.rego" | awk '{print $1}')"
policy_version="${POLICY_BUNDLE_VERSION:-0.1.0}"

gateway_env=(
  "PORT=8081"
  "POLICY_BUNDLE_HASH=${policy_hash}"
  "POLICY_BUNDLE_VERSION=${policy_version}"
)

if command -v opa >/dev/null 2>&1; then
  opa run --server --addr "127.0.0.1:8181" "${root_dir}/policies/opa" >"${out_dir}/opa.log" 2>&1 &
  OPA_PID=$!
  wait_for_http "http://127.0.0.1:8181/health" "OPA"
  gateway_env+=("OPA_URL=http://127.0.0.1:8181")
else
  gateway_env+=("ALLOW_LOCAL_POLICY_FALLBACK=true")
fi

(
  cd "${root_dir}/services/gateway"
  env "${gateway_env[@]}" go run . >"${out_dir}/gateway.log" 2>&1
) &
GATEWAY_PID=$!
wait_for_http "http://127.0.0.1:8081/healthz" "gateway"

(
  cd "${root_dir}/services/worker"
  go run . >"${out_dir}/worker.log" 2>&1
) &
WORKER_PID=$!
wait_for_http "http://127.0.0.1:9090/healthz" "worker"

cat > "${out_dir}/req_denied.json" <<EOF
{
  "subject": { "type": "agent", "id": "mcp-client", "attributes": {"role": "developer"} },
  "request": { "action": "deploy", "resource": "backend", "environment": "dev", "attributes": {} },
  "data_classification": "internal"
}
EOF

cat > "${out_dir}/req_allowed.json" <<EOF
{
  "subject": { "type": "agent", "id": "mcp-client", "attributes": {"role": "developer"} },
  "request": { "action": "deploy", "resource": "backend", "environment": "dev", "attributes": {"tests_passed": "true"} },
  "data_classification": "internal"
}
EOF

printf "\n======================================================\n"
printf "Step 1: AI attempts deploy WITHOUT 'tests_passed'\n"
printf "======================================================\n"
curl -fsS -H "Content-Type: application/json" -X POST "http://127.0.0.1:8081/decide" --data @"${out_dir}/req_denied.json" > "${out_dir}/trace_denied.json"
decision=$(jq -r .policy.decision "${out_dir}/trace_denied.json")
rationale=$(jq -r '.policy.rationale[0].message' "${out_dir}/trace_denied.json")
echo "OPA Decision: ${decision}"
echo "Rationale:    ${rationale}"

printf "\n======================================================\n"
printf "Step 2: AI attempts deploy WITH 'tests_passed'\n"
printf "======================================================\n"
curl -fsS -H "Content-Type: application/json" -X POST "http://127.0.0.1:8081/decide" --data @"${out_dir}/req_allowed.json" > "${out_dir}/trace_allowed.json"
decision=$(jq -r .policy.decision "${out_dir}/trace_allowed.json")
rationale=$(jq -r '.policy.rationale[0].message' "${out_dir}/trace_allowed.json")
echo "OPA Decision: ${decision}"
echo "Rationale:    ${rationale}"

printf "\n======================================================\n"
printf "Step 3: Action Executed -> Evidence Generated\n"
printf "======================================================\n"
curl -fsS -H "Content-Type: application/json" -X POST "http://127.0.0.1:9090/execute" --data @"${out_dir}/trace_allowed.json" > "${out_dir}/evidence.json"
echo "Evidence recorded for correlation_id: $(jq -r .correlation_id "${out_dir}/evidence.json")"

printf "\n======================================================\n"
printf "Step 4: Compiling Immutable Audit Packet\n"
printf "======================================================\n"
(
  cd "${root_dir}/tools/schema-check"
  go run . "${root_dir}/schemas/decision_trace.avsc" "${out_dir}/trace_allowed.json" >/dev/null
  go run . "${root_dir}/schemas/evidence.avsc" "${out_dir}/evidence.json" >/dev/null
)

correlation_id=$(jq -r .correlation_id "${out_dir}/trace_allowed.json")
export AUDIT_DECISION_TRACE_FILE="${out_dir}/trace_allowed.json"
export AUDIT_EVIDENCE_FILE="${out_dir}/evidence.json"
"${root_dir}/scripts/export_audit_packet.sh" "${correlation_id}"

printf '\nDemo completed successfully!\n'

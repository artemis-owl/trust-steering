#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${1:-$(mktemp -d "${TMPDIR:-/tmp}/steering-kind-demo.XXXXXX")}"
request_file="${DECISION_REQUEST_FILE:-${root_dir}/schemas/examples/decision_request.sample.json}"
namespace="steering"
cluster_name="${KIND_CLUSTER_NAME:-steering}"
gateway_port="${GATEWAY_PORT:-18080}"
worker_port="${WORKER_PORT:-19090}"

for cmd in kind docker kubectl curl go; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} is required for scripts/demo_kind.sh" >&2
    exit 1
  fi
done

if ! docker info >/dev/null 2>&1; then
  echo "Docker daemon is not running." >&2
  exit 1
fi

mkdir -p "${out_dir}"
export GOCACHE="${GOCACHE:-${root_dir}/.gocache}"

capture_diagnostics() {
  if ! kubectl get namespace "${namespace}" >/dev/null 2>&1; then
    return
  fi

  kubectl get pods -n "${namespace}" -o wide >"${out_dir}/pods.txt" 2>&1 || true
  for app in opa gateway worker jaeger otel-collector; do
    kubectl logs -n "${namespace}" deployment/"${app}" --tail=-1 >"${out_dir}/${app}.log" 2>&1 || true
  done
}

cleanup() {
  local exit_code="$1"
  for pid in "${GATEWAY_FORWARD_PID:-}" "${WORKER_FORWARD_PID:-}" "${JAEGER_FORWARD_PID:-}"; do
    if [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1; then
      kill "${pid}" >/dev/null 2>&1 || true
      wait "${pid}" >/dev/null 2>&1 || true
    fi
  done

  if [[ "${exit_code}" -ne 0 ]]; then
    capture_diagnostics
  fi
}
trap 'cleanup $?' EXIT

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

wait_for_rollout() {
  local deployment="$1"
  kubectl rollout status -n "${namespace}" deployment/"${deployment}" --timeout=180s
}

if ! kind get clusters 2>/dev/null | grep -qx "${cluster_name}"; then
  KIND_CLUSTER_NAME="${cluster_name}" "${root_dir}/scripts/up.sh"
fi

KIND_CLUSTER_NAME="${cluster_name}" "${root_dir}/scripts/kind_load.sh"
POLICY_BUNDLE_VERSION="${POLICY_BUNDLE_VERSION:-0.1.0}" "${root_dir}/scripts/kind_deploy.sh"

wait_for_rollout opa
wait_for_rollout gateway
wait_for_rollout worker
wait_for_rollout jaeger
wait_for_rollout otel-collector

kubectl port-forward -n "${namespace}" svc/gateway "${gateway_port}:8080" >"${out_dir}/gateway-port-forward.log" 2>&1 &
GATEWAY_FORWARD_PID=$!

kubectl port-forward -n "${namespace}" svc/jaeger "16686:16686" >"${out_dir}/jaeger-port-forward.log" 2>&1 &
JAEGER_FORWARD_PID=$!
wait_for_http "http://127.0.0.1:${gateway_port}/healthz" "gateway"

kubectl port-forward -n "${namespace}" svc/worker "${worker_port}:9090" >"${out_dir}/worker-port-forward.log" 2>&1 &
WORKER_FORWARD_PID=$!
wait_for_http "http://127.0.0.1:${worker_port}/healthz" "worker"

curl -fsS \
  -H "Content-Type: application/json" \
  -X POST "http://127.0.0.1:${gateway_port}/decide" \
  --data @"${request_file}" \
  >"${out_dir}/decision_trace.json"

curl -fsS \
  -H "Content-Type: application/json" \
  -X POST "http://127.0.0.1:${worker_port}/execute" \
  --data @"${out_dir}/decision_trace.json" \
  >"${out_dir}/evidence.json"

(
  cd "${root_dir}/tools/schema-check"
  go run . "${root_dir}/schemas/decision_trace.avsc" "${out_dir}/decision_trace.json" >/dev/null
  go run . "${root_dir}/schemas/evidence.avsc" "${out_dir}/evidence.json" >/dev/null
)

capture_diagnostics

printf 'Kind demo outputs written to %s\n' "${out_dir}"
printf 'Decision Trace: %s\n' "${out_dir}/decision_trace.json"
printf 'Evidence: %s\n' "${out_dir}/evidence.json"
printf 'Jaeger UI: http://127.0.0.1:16686\n'

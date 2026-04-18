#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${root_dir}/infra/compose/docker-compose.yml"
policy_file="${root_dir}/policies/opa/rego/decision.rego"

for cmd in curl docker shasum; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} is required for scripts/compose_up.sh" >&2
    exit 1
  fi
done

if ! docker info >/dev/null 2>&1; then
  echo "Docker daemon is not running." >&2
  exit 1
fi

if [[ ! -f "${policy_file}" ]]; then
  echo "Missing policy file: ${policy_file}" >&2
  exit 1
fi

export POLICY_BUNDLE_HASH="sha256:$(shasum -a 256 "${policy_file}" | awk '{print $1}')"
export POLICY_BUNDLE_VERSION="${POLICY_BUNDLE_VERSION:-0.1.0}"

stream_profile=0
if [[ "${COMPOSE_PROFILES:-}" == *stream* ]]; then
  stream_profile=1
fi
for arg in "$@"; do
  if [[ "${arg}" == *stream* ]]; then
    stream_profile=1
  fi
done
if [[ "${stream_profile}" -eq 1 ]]; then
  export COMPOSE_PROFILES="${COMPOSE_PROFILES:-stream}"
  # Always use the Docker service name; do not inherit host-only values like 127.0.0.1:19092.
  export KAFKA_BOOTSTRAP_SERVERS="redpanda:9092"
  echo "COMPOSE_PROFILES=${COMPOSE_PROFILES}"
  echo "KAFKA_BOOTSTRAP_SERVERS=${KAFKA_BOOTSTRAP_SERVERS}"
fi

obs_profile=0
if [[ "${COMPOSE_PROFILES:-}" == *obs* ]]; then
  obs_profile=1
fi
for arg in "$@"; do
  if [[ "${arg}" == *obs* ]]; then
    obs_profile=1
  fi
done
if [[ "${obs_profile}" -eq 1 ]]; then
  export OTEL_EXPORTER_OTLP_ENDPOINT="${OTEL_EXPORTER_OTLP_ENDPOINT:-http://otel-collector:4318}"
  echo "OTEL_EXPORTER_OTLP_ENDPOINT=${OTEL_EXPORTER_OTLP_ENDPOINT}"
fi

echo "POLICY_BUNDLE_HASH=${POLICY_BUNDLE_HASH}"
echo "POLICY_BUNDLE_VERSION=${POLICY_BUNDLE_VERSION}"

docker compose -f "${compose_file}" up -d --build "$@"

wait_http() {
  local url="$1"
  local name="$2"
  for _ in $(seq 1 60); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  echo "${name} did not become ready at ${url}" >&2
  exit 1
}

wait_http "http://127.0.0.1:8181/health" "OPA"
if [[ "${stream_profile}" -eq 1 ]]; then
  # Redpanda returns 503 until ready; retry until 200.
  for _ in $(seq 1 90); do
    code="$(curl -sS -o /dev/null -w "%{http_code}" "http://127.0.0.1:9644/v1/status/ready" 2>/dev/null || echo 000)"
    if [[ "${code}" == "200" ]]; then
      break
    fi
    sleep 0.5
  done
  code="$(curl -sS -o /dev/null -w "%{http_code}" "http://127.0.0.1:9644/v1/status/ready")"
  if [[ "${code}" != "200" ]]; then
    echo "redpanda did not become ready (last HTTP ${code})" >&2
    exit 1
  fi
  # Admin /ready can succeed before the Kafka API accepts metadata; brief settle time.
  sleep 3
  # Ensure topics exist before gateway/worker traffic (consumer subscribe is reliable).
  COMPOSE_PROFILES="${COMPOSE_PROFILES:-stream}" docker compose -f "${compose_file}" exec -T redpanda \
    rpk topic create decision.trace.v1 decision.evidence.v1 -p 1 2>/dev/null || true
fi
wait_http "http://127.0.0.1:8080/healthz" "gateway"
wait_http "http://127.0.0.1:9090/healthz" "worker"

echo "Compose stack is up."

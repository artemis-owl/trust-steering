#!/usr/bin/env bash
set -euo pipefail

# Integration check: stream + obs profiles, demo_stream, then assert Jaeger received traces.
# Requires Docker, curl, python3, go, openssl (same as demo_stream).

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${root_dir}/infra/compose/docker-compose.yml"
jaeger_api="${JAEGER_QUERY_URL:-http://127.0.0.1:16686}"

for cmd in docker curl python3 go openssl; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} is required for scripts/test_obs.sh" >&2
    exit 1
  fi
done

if ! docker info >/dev/null 2>&1; then
  echo "Docker daemon is not running; skipping obs test." >&2
  exit 0
fi

unset KAFKA_BOOTSTRAP_SERVERS 2>/dev/null || true

cleanup() {
  docker compose -f "${compose_file}" down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

COMPOSE_PROFILES=stream,obs "${root_dir}/scripts/compose_up.sh"

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/steering-obs-test.XXXXXX")"
"${root_dir}/scripts/demo_stream.sh" "${tmp_dir}"

jaeger_has_traces() {
  local svc="$1"
  local url="${jaeger_api}/api/traces?service=${svc}&limit=5"
  local body
  body="$(curl -fsS "${url}")" || return 1
  python3 -c "
import json, sys
d = json.loads(sys.argv[1])
data = d.get('data') or []
sys.exit(0 if len(data) > 0 else 1)
" "${body}"
}

for _ in $(seq 1 60); do
  if jaeger_has_traces "steering-gateway" && jaeger_has_traces "steering-worker"; then
    echo "obs test OK (Jaeger has steering-gateway and steering-worker traces)"
    exit 0
  fi
  sleep 0.5
done

echo "Jaeger did not receive traces for steering-gateway and steering-worker within timeout" >&2
exit 1

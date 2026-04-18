#!/usr/bin/env bash
set -euo pipefail

# Integration check: stream profile + demo_stream. Requires Docker, curl, python3, go.
# Does not replace scripts/test.sh (HTTP-only, no broker).

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

for cmd in docker curl python3 go openssl; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} is required for scripts/test_stream.sh" >&2
    exit 1
  fi
done

if ! docker info >/dev/null 2>&1; then
  echo "Docker daemon is not running; skipping stream test." >&2
  exit 0
fi

# Avoid inheriting host-only broker URLs into Compose service env.
unset KAFKA_BOOTSTRAP_SERVERS 2>/dev/null || true

cleanup() {
  COMPOSE_PROFILES=stream KAFKA_BOOTSTRAP_SERVERS=redpanda:9092 \
    docker compose -f "${root_dir}/infra/compose/docker-compose.yml" down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

COMPOSE_PROFILES=stream "${root_dir}/scripts/compose_up.sh"

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/steering-stream-test.XXXXXX")"
"${root_dir}/scripts/demo_stream.sh" "${tmp_dir}"

echo "stream test OK"

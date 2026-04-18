#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${root_dir}/infra/compose/docker-compose.yml"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required for scripts/compose_down.sh" >&2
  exit 1
fi

docker compose -f "${compose_file}" down "$@"

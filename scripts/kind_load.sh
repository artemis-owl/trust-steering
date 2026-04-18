#!/usr/bin/env bash
set -euo pipefail

cluster_name="${KIND_CLUSTER_NAME:-steering}"
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

for cmd in docker kind; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} is required for scripts/kind_load.sh" >&2
    exit 1
  fi
done

if ! docker info >/dev/null 2>&1; then
  echo "Docker daemon is not running." >&2
  exit 1
fi

if ! kind get clusters 2>/dev/null | grep -qx "${cluster_name}"; then
  echo "kind cluster '${cluster_name}' does not exist" >&2
  exit 1
fi

docker build -t steering-gateway:dev "${root_dir}/services/gateway"
docker build -t steering-worker:dev "${root_dir}/services/worker"

kind load docker-image --name "${cluster_name}" steering-gateway:dev steering-worker:dev

echo "Loaded steering-gateway:dev and steering-worker:dev into kind cluster '${cluster_name}'."

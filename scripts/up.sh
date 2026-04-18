#!/usr/bin/env bash
set -euo pipefail

cluster_name="${KIND_CLUSTER_NAME:-steering}"
config_file="infra/kind/cluster.yaml"

if ! command -v kind >/dev/null 2>&1; then
  echo "kind not found. Install kind to continue." >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found. Install Docker to continue." >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "Docker daemon is not running." >&2
  exit 1
fi

if [[ ! -f "$config_file" ]]; then
  echo "Missing kind config: ${config_file}" >&2
  exit 1
fi

if kind get clusters | grep -q "^${cluster_name}$"; then
  echo "kind cluster '${cluster_name}' already exists."
  exit 0
fi

echo "Creating kind cluster '${cluster_name}'..."
kind create cluster --name "${cluster_name}" --config "${config_file}"
echo "kind cluster '${cluster_name}' created."

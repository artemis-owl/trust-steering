#!/usr/bin/env bash
set -euo pipefail

cluster_name="${KIND_CLUSTER_NAME:-steering}"

if ! command -v kind >/dev/null 2>&1; then
  echo "kind not found. Nothing to tear down." >&2
  exit 0
fi

if ! kind get clusters | grep -q "^${cluster_name}$"; then
  echo "kind cluster '${cluster_name}' does not exist."
  exit 0
fi

echo "Deleting kind cluster '${cluster_name}'..."
kind delete cluster --name "${cluster_name}"
echo "kind cluster '${cluster_name}' deleted."

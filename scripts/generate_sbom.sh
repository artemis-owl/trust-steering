#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
sbom_dir="${root_dir}/supplychain/sbom/out"

# Ensure tools are available
if ! command -v syft >/dev/null 2>&1; then
  echo "syft is required but not installed. Run: brew install syft" >&2
  exit 1
fi

mkdir -p "${sbom_dir}"

echo "Generating SBOM for steering-gateway:dev..."
syft "steering-gateway:dev" -o spdx-json="${sbom_dir}/gateway.spdx.json"

echo "Generating SBOM for steering-worker:dev..."
syft "steering-worker:dev" -o spdx-json="${sbom_dir}/worker.spdx.json"

echo "SBOMs generated successfully in ${sbom_dir}."

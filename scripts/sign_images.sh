#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
keys_dir="${root_dir}/.keys"
sbom_dir="${root_dir}/supplychain/sbom/out"
registry="localhost:5001"

for cmd in docker cosign; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} is required but not installed." >&2
    exit 1
  fi
done

# Start local registry if not running
if ! docker inspect local-registry >/dev/null 2>&1; then
  echo "Starting local Docker registry on port 5001..."
  docker run -d -p 5001:5000 --name local-registry registry:2
fi

# Ensure keys exist
mkdir -p "${keys_dir}"
if [ ! -f "${keys_dir}/cosign.key" ]; then
  echo "Generating Cosign keypair..."
  export COSIGN_PASSWORD=""
  cd "${keys_dir}" && cosign generate-key-pair
  cd "${root_dir}"
fi

push_and_sign() {
  local image_name=$1
  local sbom_path=$2
  local local_tag="${image_name}:dev"
  local registry_tag="${registry}/${image_name}:dev"

  echo "Pushing ${local_tag} to local registry at ${registry_tag}..."
  docker tag "${local_tag}" "${registry_tag}"
  docker push "${registry_tag}"

  export COSIGN_PASSWORD=""

  echo "Signing image ${registry_tag}..."
  cosign sign --key "${keys_dir}/cosign.key" --yes "${registry_tag}"

  if [ -f "${sbom_path}" ]; then
    echo "Attaching SBOM to ${registry_tag}..."
    cosign attest --key "${keys_dir}/cosign.key" --type spdxjson --predicate "${sbom_path}" --yes "${registry_tag}"
  else
    echo "Warning: SBOM not found at ${sbom_path}, skipping attestation."
  fi
}

push_and_sign "steering-gateway" "${sbom_dir}/gateway.spdx.json"
push_and_sign "steering-worker" "${sbom_dir}/worker.spdx.json"

echo "Images signed and attested successfully!"

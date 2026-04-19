#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
keys_dir="${root_dir}/.keys"
registry="localhost:5001"

if ! command -v cosign >/dev/null 2>&1; then
  echo "cosign is required but not installed." >&2
  exit 1
fi

if [ ! -f "${keys_dir}/cosign.pub" ]; then
  echo "Public key not found at ${keys_dir}/cosign.pub. Have you run sign_images.sh?" >&2
  exit 1
fi

verify_image() {
  local image_name=$1
  local registry_tag="${registry}/${image_name}:dev"

  echo "----------------------------------------"
  echo "Verifying provenance for ${registry_tag}"
  echo "----------------------------------------"

  echo "[1/2] Verifying cryptographic signature..."
  cosign verify --key "${keys_dir}/cosign.pub" "${registry_tag}"

  echo "[2/2] Verifying attached SBOM attestation..."
  cosign verify-attestation --key "${keys_dir}/cosign.pub" --type spdxjson "${registry_tag}"
}

verify_image "steering-gateway"
verify_image "steering-worker"

echo "Provenance verification successful! Both images are signed and attested."

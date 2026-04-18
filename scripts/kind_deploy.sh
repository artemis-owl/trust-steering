#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
namespace="steering"
policy_file="${root_dir}/policies/opa/rego/decision.rego"
policy_version="${POLICY_BUNDLE_VERSION:-0.1.0}"

for cmd in kubectl shasum; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} is required for scripts/kind_deploy.sh" >&2
    exit 1
  fi
done

if [[ ! -f "${policy_file}" ]]; then
  echo "Missing policy file: ${policy_file}" >&2
  exit 1
fi

policy_hash="sha256:$(shasum -a 256 "${policy_file}" | awk '{print $1}')"

kubectl apply -f "${root_dir}/infra/kind/manifests/namespace.yaml"
kubectl -n "${namespace}" create configmap opa-policy \
  --from-file=decision.rego="${policy_file}" \
  --dry-run=client \
  -o yaml | kubectl apply -f -
kubectl apply -f "${root_dir}/infra/kind/manifests/opa.yaml"
kubectl apply -f "${root_dir}/infra/kind/manifests/gateway.yaml"
kubectl apply -f "${root_dir}/infra/kind/manifests/worker.yaml"

kubectl -n "${namespace}" create configmap otel-collector-config \
  --from-file=otel-collector.yaml="${root_dir}/observability/otel-collector.yaml" \
  --dry-run=client \
  -o yaml | kubectl apply -f -
kubectl apply -f "${root_dir}/infra/kind/manifests/jaeger.yaml"
kubectl apply -f "${root_dir}/infra/kind/manifests/otel-collector.yaml"

kubectl -n "${namespace}" set env deployment/gateway \
  POLICY_BUNDLE_HASH="${policy_hash}" \
  POLICY_BUNDLE_VERSION="${policy_version}"
kubectl -n "${namespace}" rollout restart deployment/opa deployment/gateway deployment/worker deployment/jaeger deployment/otel-collector

echo "Applied kind manifests in namespace '${namespace}'."
echo "Gateway policy bundle hash: ${policy_hash}"
echo "Gateway policy bundle version: ${policy_version}"

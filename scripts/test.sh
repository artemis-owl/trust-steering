#!/usr/bin/env bash
set -euo pipefail

fail=0

echo "Running best-effort checks..."

export GOCACHE="${GOCACHE:-$(pwd)/.gocache}"

validate_schema() {
  local schema="$1"
  local payload="$2"
  (cd tools/schema-check && go run . "../../${schema}" "../../${payload}" >/dev/null) || fail=1
}

if command -v opa >/dev/null 2>&1; then
  echo "- opa test policies/opa"
  opa test -v policies/opa >/dev/null
else
  echo "- opa not found; skipping Rego unit tests (install OPA to enable)"
fi

if command -v go >/dev/null 2>&1; then
  echo "- go test services/gateway"
  (cd services/gateway && go test ./... >/dev/null) || fail=1
  echo "- go test services/worker"
  (cd services/worker && go test ./... >/dev/null) || fail=1
  echo "- validate example Decision Trace against Avro"
  validate_schema "schemas/decision_trace.avsc" "schemas/examples/decision_trace.sample.json"
  echo "- validate example Evidence against Avro"
  validate_schema "schemas/evidence.avsc" "schemas/examples/evidence.sample.json"
else
  echo "- go not found; skipping Go tests"
fi

echo "- schema files present"
test -f schemas/decision_trace.avsc || fail=1
test -f schemas/evidence.avsc || fail=1

if command -v go >/dev/null 2>&1 && command -v curl >/dev/null 2>&1; then
  echo "- validate live Decision Trace -> Evidence demo"
  tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/steering-test.XXXXXX")"
  ./scripts/demo_local.sh "${tmp_dir}" >/dev/null || fail=1
else
  echo "- skipping live demo validation (requires go and curl)"
fi

if [[ "$fail" -ne 0 ]]; then
  echo "Checks failed."
  exit 1
fi

echo "OK"

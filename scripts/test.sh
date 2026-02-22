#!/usr/bin/env bash
set -euo pipefail

fail=0

echo "Running best-effort checks..."

if command -v opa >/dev/null 2>&1; then
  echo "- opa test policies/opa"
  opa test -v policies/opa >/dev/null
else
  echo "- opa not found; skipping Rego unit tests (install OPA to enable)"
fi

echo "- schema files present"
test -f schemas/decision_trace.avsc || fail=1
test -f schemas/evidence.avsc || fail=1

if [[ "$fail" -ne 0 ]]; then
  echo "Checks failed."
  exit 1
fi

echo "OK"


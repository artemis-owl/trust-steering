#!/usr/bin/env bash
set -euo pipefail

# End-to-end: Compose with stream profile (Redpanda), POST /decide, wait for evidence on decision.evidence.v1.
# Prerequisites: ./scripts/compose_up.sh with COMPOSE_PROFILES=stream (or pass --profile stream).

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${1:-$(mktemp -d "${TMPDIR:-/tmp}/steering-stream-demo.XXXXXX")}"
sample="${root_dir}/schemas/examples/decision_request.sample.json"

for cmd in curl python3; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} is required for scripts/demo_stream.sh" >&2
    exit 1
  fi
done

mkdir -p "${out_dir}"
export GOCACHE="${GOCACHE:-${root_dir}/.gocache}"

corr_id="corr-stream-$(openssl rand -hex 4)"
export corr_id
export SAMPLE="${sample}"

req_json="$(python3 -c "
import json, os
with open(os.environ['SAMPLE']) as f:
    d = json.load(f)
d['correlation_id'] = os.environ['corr_id']
print(json.dumps(d))
")"

curl -fsS \
  -H "Content-Type: application/json" \
  -X POST "http://127.0.0.1:8080/decide" \
  --data "${req_json}" \
  >"${out_dir}/decision_trace.json"

(
  cd "${root_dir}/tools/schema-check"
  go run . "${root_dir}/schemas/decision_trace.avsc" "${out_dir}/decision_trace.json" >/dev/null
)

export KAFKA_BOOTSTRAP_SERVERS="${KAFKA_BOOTSTRAP_SERVERS:-127.0.0.1:19092}"
export KAFKA_TOPIC="${KAFKA_TOPIC_EVIDENCE:-decision.evidence.v1}"
export CORRELATION_ID="${corr_id}"
# Do not inherit a short TIMEOUT_SEC from the environment (e.g. CI); polling needs headroom.
export TIMEOUT_SEC="${DEMO_STREAM_KAFKA_TIMEOUT_SEC:-90}"

(
  cd "${root_dir}/tools/kafka-read-one"
  go run . >"${out_dir}/evidence.json"
)

(
  cd "${root_dir}/tools/schema-check"
  go run . "${root_dir}/schemas/evidence.avsc" "${out_dir}/evidence.json" >/dev/null
)

printf 'Stream demo outputs written to %s\n' "${out_dir}"
printf 'correlation_id: %s\n' "${corr_id}"
printf 'Decision Trace: %s\n' "${out_dir}/decision_trace.json"
printf 'Evidence: %s\n' "${out_dir}/evidence.json"

# Scripts

Scripts are intentionally small and local-first. Operator matrix (Compose vs stream vs Kind): `docs/runbook.md`.

- `scripts/compose_up.sh` / `scripts/compose_down.sh`: start/stop the **primary** Docker Compose stack (OPA + gateway + worker; optional Redpanda with `COMPOSE_PROFILES=stream`; optional Jaeger + OTel collector with `obs`). For stream, sets `KAFKA_BOOTSTRAP_SERVERS=redpanda:9092` for services (not host localhost). For `obs` (env or e.g. `--profile obs`), sets `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318` for gateway/worker.
- `scripts/demo_compose.sh`: smoke test against the Compose stack (`http://127.0.0.1:8080` / `:9090` by default); prints the decision (the sample request is denied, so its evidence is `action.skipped`)
- `scripts/demo_stream.sh`: `POST /decide` then read matching evidence from `decision.evidence.v1` via `tools/kafka-read-one` (host bootstrap `127.0.0.1:19092`); prints the decision (the sample request is denied)
- `scripts/test_stream.sh`: integration test (Docker + stream profile + `demo_stream.sh`); optional, does not run in `test.sh`
- `scripts/test_obs.sh`: integration test (Docker + `stream,obs` + `demo_stream.sh` + Jaeger API assert); optional (`make test-obs`)
- `scripts/up.sh` / `scripts/down.sh`: create/delete local kind cluster (Kubernetes parity)
- `scripts/demo_local.sh`: run the Decision Trace → Evidence demo with **local processes**: builds the gateway (`8081`, to avoid clashing with Compose on `8080`) and worker (`9090`) into its output directory, stops them on exit, and refuses to start if either port already answers
- `scripts/demo_kind.sh`: create/reuse kind, load images, deploy OPA + gateway + worker, and validate the in-cluster smoke path
- `scripts/test.sh`: best-effort local checks (OPA tests, Go tests, schema validation, live demo when tools are installed)
- `scripts/export_audit_packet.sh <correlation_id>`: write `out/audit-packets/<correlation_id>/` from files (`AUDIT_DECISION_TRACE_FILE`, `AUDIT_EVIDENCE_FILE`) or from Kafka (`KAFKA_BOOTSTRAP_SERVERS`)
- `scripts/kind_load.sh`: build local service images and load them into kind
- `scripts/kind_deploy.sh`: create the OPA policy ConfigMap, apply kind manifests, and set gateway policy metadata
- `scripts/approve.sh list|grant|deny <correlation_id>`: write approval files for pending prod work orders (`tools/approve`); it authenticates nobody
- `scripts/generate_sbom.sh` (`make sbom`): SPDX SBOMs via `syft` for local images `steering-gateway:dev` / `steering-worker:dev` (built by `scripts/kind_load.sh`)
- `scripts/sign_images.sh` (`make sign`): push those images to a loopback registry, sign them with `cosign`, and attest the SBOM
- `scripts/verify_provenance.sh` (`make verify-provenance`): verify those signatures and attestations with `cosign`

Environment:
- `KIND_CLUSTER_NAME` for cluster name (default: `steering`)
- `POLICY_BUNDLE_VERSION` for the gateway policy version during kind deploy (default: `0.1.0`)

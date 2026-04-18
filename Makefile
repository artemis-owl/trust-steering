.PHONY: help test test-stream test-obs repo-map kind-demo compose-up compose-down

help:
	@echo "Targets:"
	@echo "  make help         Show this help"
	@echo "  make compose-up   Start OPA + gateway + worker (Docker Compose)"
	@echo "  make compose-down Stop the Compose stack"
	@echo "  make kind-demo    Run the in-cluster kind smoke demo"
	@echo "  make repo-map     Print a quick repo map"
	@echo "  make test         Run local checks (best-effort, HTTP-only)"
	@echo "  make test-stream  Docker + Redpanda streaming integration (optional)"
	@echo "  make test-obs     Docker + stream + Jaeger OTel integration (optional)"

repo-map:
	@echo "Docs:"
	@echo "  docs/repo-map.md"
	@echo "  docs/runbook.md"
	@echo "  docs/reading-plan-6w.md"
	@echo "  docs/roadmap.md"
	@echo "  docs/corpus.md"
	@echo "Core artifacts:"
	@echo "  policies/opa/rego/decision.rego"
	@echo "  schemas/decision_trace.avsc"
	@echo "  schemas/evidence.avsc"

test:
	@./scripts/test.sh

test-stream:
	@./scripts/test_stream.sh

test-obs:
	@./scripts/test_obs.sh

kind-demo:
	@./scripts/demo_kind.sh

compose-up:
	@./scripts/compose_up.sh

compose-down:
	@./scripts/compose_down.sh

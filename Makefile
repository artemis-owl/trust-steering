.PHONY: help test repo-map

help:
	@echo "Targets:"
	@echo "  make help      Show this help"
	@echo "  make repo-map  Print a quick repo map"
	@echo "  make test      Run local checks (best-effort)"

repo-map:
	@echo "Docs:"
	@echo "  docs/repo-map.md"
	@echo "  docs/reading-plan-6w.md"
	@echo "  docs/roadmap.md"
	@echo "  docs/corpus.md"
	@echo "Core artifacts:"
	@echo "  policies/opa/rego/decision.rego"
	@echo "  schemas/decision_trace.avsc"
	@echo "  schemas/evidence.avsc"

test:
	@./scripts/test.sh


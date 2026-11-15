# Services

This repo has two main components:
- `services/gateway/`: receives requests and produces a Decision Trace (policy-bound)
- `services/worker/`: takes decision traces (HTTP `/execute` or the trace topic) and records an Evidence event for each. It executes nothing, and it accepts any well-formed trace without checking where it came from.

Current bootstrap implementation is in Go for fast local iteration.

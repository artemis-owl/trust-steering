# Services

This repo has two main components:
- `services/gateway/`: receives requests and produces a Decision Trace (policy-bound)
- `services/worker/`: consumes decision traces, executes actions, and emits Evidence

Current bootstrap implementation is in Go for fast local iteration.

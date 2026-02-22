# Worker (planned)

Responsibilities:
- consume Decision Trace events
- execute/skip based on decision
- emit Evidence events (success/failure/side effects)
- preserve correlation ids and trace context from message headers

Planned interfaces:
- consumer from topic: `decision.trace.v1`
- producer to topic: `decision.evidence.v1`


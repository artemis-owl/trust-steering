# Gateway (planned)

Responsibilities:
- authenticate/identify subject
- construct policy input from request attributes
- call OPA for allow/deny + rationale
- emit a Decision Trace event (and later publish to the stream)
- ensure correlation ids and trace context are propagated

Planned interfaces:
- HTTP API: `POST /decide` (returns decision + decision trace)
- (later) producer to topic: `decision.trace.v1`


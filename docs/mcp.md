# MCP Server — Trust Middleware for AI Agents

The steering gateway embeds an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server that exposes task-level trust governance as native agent tools. Any MCP-compatible client — Cursor, Claude Code, VS Code Copilot, ChatGPT — can use the trust layer without custom integration.

## Architecture

```
┌─────────────────────────────────────────────┐
│              MCP Client                     │
│  (Cursor / Claude Code / VS Code / etc.)    │
└────────────┬───────────────────┬────────────┘
             │ stdio (JSON-RPC)  │ Direct (data plane)
             ▼                   ▼
┌────────────────────────┐  ┌──────────────────────┐
│  ./gateway --mcp       │  │ inference engine      │
│  (same binary)         │  │ (modelbase, API, etc) │
│                        │  └──────────────────────┘
│  Tools:                │
│  - request_work_order  │
│  - submit_evidence     │
│  - get_policy_status   │
└────────────────────────┘
```

The MCP server is completely **out of the data plane**. It governs the *authority to act* and audits the *result of acting*. It never touches inference traffic, streaming tokens, or model I/O.

## The Work Order Pattern

The governance model operates at the **task (epic) level**, not per-request:

1. **Pre-flight**: Agent calls `request_work_order` → steering evaluates policy → issues a `correlation_id` if approved.
2. **Execution**: Agent works freely. No interception of individual LLM calls.
3. **Post-flight**: Agent calls `submit_evidence` with the same `correlation_id` → evidence recorded for audit.

This keeps the trust layer "on the loop" (audit) rather than "in the loop" (blocking every decision).

## Tools

### `request_work_order`

Call **before** starting a significant task.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `action` | ✅ | Task type: `build`, `deploy`, `inference`, `refactor` |
| `resource` | ✅ | Target: `wiki`, `cluster`, `repository`, `modelbase` |
| `environment` | | `dev`, `staging`, `prod` (defaults to `dev`) |
| `context` | | Free-text description of what the agent plans to do |

**Returns:**
```json
{
  "correlation_id": "corr-a1b2c3d4e5f6g7h8",
  "decision": "allow",
  "rationale": [{"code": "ALLOW_DEV", "message": "Dev actions allowed for developers."}],
  "event_id": "abc123...",
  "trace_id": "def456..."
}
```

If `decision` is `"deny"`, the agent should **stop** and relay the rationale to the user.

### `submit_evidence`

Call **after** a task completes (or fails).

| Parameter | Required | Description |
|-----------|----------|-------------|
| `correlation_id` | ✅ | The ID returned by `request_work_order` |
| `action` | ✅ | Action that was executed |
| `result` | ✅ | Outcome: `ok`, `error`, or `skipped` |
| `summary` | | Brief description of what happened |

**Returns:**
```json
{
  "event_id": "xyz789...",
  "correlation_id": "corr-a1b2c3d4e5f6g7h8",
  "status": "recorded",
  "result": "ok"
}
```

### `get_policy_status`

Health check. No parameters.

**Returns:**
```json
{
  "gateway_healthy": true,
  "policy_engine": "local-fallback",
  "bundle_hash": "local-dev",
  "kafka_connected": false
}
```

## Setup

### 1. Build the gateway

```sh
cd services/gateway
go build -o gateway .
```

### 2. Configure your agent

Create the MCP config for your client, pointing to the compiled binary:

**Cursor** (`~/.cursor/mcp.json`):
```json
{
  "mcpServers": {
    "steering": {
      "command": "/absolute/path/to/gateway",
      "args": ["--mcp"],
      "env": {
        "ALLOW_LOCAL_POLICY_FALLBACK": "true"
      }
    }
  }
}
```

**Claude Code** (`~/.claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "steering": {
      "command": "/absolute/path/to/gateway",
      "args": ["--mcp"],
      "env": {
        "ALLOW_LOCAL_POLICY_FALLBACK": "true"
      }
    }
  }
}
```

**VS Code Copilot** (`.vscode/settings.json`):
```json
{
  "github.copilot.chat.mcp.servers": {
    "steering": {
      "command": "/absolute/path/to/gateway",
      "args": ["--mcp"],
      "env": {
        "ALLOW_LOCAL_POLICY_FALLBACK": "true"
      }
    }
  }
}
```

### 3. Verify

```sh
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' \
  | ALLOW_LOCAL_POLICY_FALLBACK=true ./gateway --mcp 2>/dev/null
```

You should see all three tools listed.

## Making It Automatic — Agent Rules

The MCP server gives agents the **capability** to interact with the trust layer. Agent rules give them the **habit**. Without rules, the agent only calls the tools when you explicitly ask.

### Recommended agent rules

Add these instructions to your agent's rules file so it automatically governs significant work:

**Cursor** — `.cursor/rules/steering.mdc`:
```markdown
---
description: Trust governance for significant tasks
globs: **/*
---

When beginning a significant task (build, deploy, refactor, or multi-file change):
1. Call `request_work_order` with the action, resource, and a brief context describing the plan.
2. If the decision is "deny", stop and explain the policy rationale to the user. Do not proceed.
3. If "allow", carry the returned `correlation_id` and proceed with the task.
4. When the task completes (or fails), call `submit_evidence` with the `correlation_id`, result, and a summary.

Do NOT request a work order for:
- Answering questions or explaining code
- Single-line edits or typo fixes
- Reading files or investigating issues
- Asking for clarification
```

**Claude Code** — `.claude/CLAUDE.md`:
```markdown
## Trust Governance

Before starting significant work (builds, deploys, refactors, multi-file changes):
1. Call `request_work_order` with action, resource, and context.
2. If denied, stop and explain. If allowed, proceed with the correlation_id.
3. When done, call `submit_evidence` with the correlation_id and result.

Skip for questions, single-line fixes, and read-only operations.
```

### What the user sees

With rules in place, the governance is **automatic and invisible**:

```
User:    "Refactor the auth module to use JWT tokens"

Agent:   [calls request_work_order: action=refactor, resource=auth-module,
          context="Refactor auth from session-based to JWT tokens"]
         → correlation_id: corr-a1b2c3d4, decision: allow

Agent:   [performs the refactoring across multiple files]

Agent:   [calls submit_evidence: correlation_id=corr-a1b2c3d4, result=ok,
          summary="Refactored auth to JWT. Modified 4 files: auth.go,
          middleware.go, handler.go, auth_test.go"]
         → Evidence recorded

Agent:   "Done! I've refactored the auth module to use JWT. Here's
          what changed: ..."
```

The user never had to ask for authorization or evidence submission — the rules made it automatic. The trust layer was "on the loop" for the entire task.

## Audit Trail

Every `request_work_order` call produces a `DecisionTrace` event and every `submit_evidence` call produces an `EvidenceEvent`. Both are keyed by `correlation_id`.

### Without Kafka (local dev)

Events are returned to the MCP client in the tool response but **not persisted to disk**. They exist in the agent's conversation context only.

### With Kafka

Set `KAFKA_BOOTSTRAP_SERVERS` and events are published to:
- `decision.trace.v1` — Decision traces (pre-flight authorization)
- `decision.evidence.v1` — Evidence events (post-flight completion)

These can be consumed, stored, and bundled into [audit packets](audit-packet.md) by `correlation_id`.

### With OTel

Set `OTEL_EXPORTER_OTLP_ENDPOINT` and spans are emitted for each tool call, tagged with `steering.correlation_id` and `policy.decision`. Visible in Jaeger or any OTLP-compatible backend.

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `ALLOW_LOCAL_POLICY_FALLBACK` | `false` | Enable built-in policy (no OPA needed) |
| `OPA_URL` | *(unset)* | OPA instance for real policy evaluation |
| `KAFKA_BOOTSTRAP_SERVERS` | *(unset)* | Durable event persistence |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | *(unset)* | Distributed tracing |
| `POLICY_BUNDLE_HASH` | `local-dev` | Reported in decision traces |
| `SCHEMA_VERSION` | `0.1.0` | Schema version stamped on events |

## FAQ

**Q: Does this proxy my LLM traffic?**
No. The MCP server is completely out of the data plane. It governs task authorization and audits completion. Your inference calls go directly to the model engine.

**Q: What happens if the MCP server is down?**
The MCP client will fail to launch the server process. Without agent rules, the agent works normally (no governance). With rules, the agent would note that the tool is unavailable and inform you.

**Q: Can I deny specific actions?**
Yes. With OPA, write a Rego policy that denies based on `action`, `resource`, `environment`, or subject attributes. With local fallback, the allowed actions are hardcoded in `isAllowedAction()`.

**Q: Does this work with any MCP client?**
Any client that supports MCP tool calls over stdio. This includes Cursor, Claude Desktop, Claude Code, VS Code Copilot, ChatGPT, and [many others](https://modelcontextprotocol.io/clients).

**Q: How is this different from a reverse proxy?**
A proxy intercepts every request in the data plane. This operates at the task level — one authorization check before work starts, one evidence submission when it ends. No per-request overhead, no coupling to inference traffic.

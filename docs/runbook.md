# Local runbook

Quick operator paths for this repo. Details live in [`README.md`](../README.md), [`infra/compose/README.md`](../infra/compose/README.md), and [`scripts/README.md`](../scripts/README.md).

## Modes (pick one)

| Mode | Broker | What runs | Typical use |
|------|--------|-----------|-------------|
| **Compose, API-only** | No | OPA + gateway + worker in Docker | Default dev/demo over HTTP |
| **Compose + stream** | Redpanda (Kafka API) | Same + broker; gateway produces traces, worker consumes/produces evidence | Async / Phase 2 path |
| **Compose + obs** | No extra broker | Adds OTel collector + Jaeger; OTLP from gateway/worker | Traces / correlation evidence |
| **Local processes** | Optional host OPA | `go run` gateway (`8081`) + worker (`9090`) | Fast iteration without building images |
| **Kind** | No (current manifests) | OPA + gateway + worker + Jaeger + OTel in-cluster | K8s-shaped smoke test |
| **MCP** | Optional | Gateway binary in stdio mode | AI agent trust middleware (Cursor, Claude Code, VS Code) |

## Compose: API-only (no Kafka)

Do **not** set `COMPOSE_PROFILES=stream`. Leave `KAFKA_BOOTSTRAP_SERVERS` unset (default in `infra/compose/docker-compose.yml`).

```sh
./scripts/compose_up.sh
./scripts/demo_compose.sh
./scripts/compose_down.sh
```

Flow: `POST /decide` → decision trace JSON → `POST /execute` with that JSON → evidence JSON.

## Compose: streaming

```sh
COMPOSE_PROFILES=stream ./scripts/compose_up.sh
./scripts/demo_stream.sh
./scripts/compose_down.sh
```

Inside Compose, services use **`redpanda:9092`**. On the host, Kafka clients use **`127.0.0.1:19092`** (see `infra/compose/README.md`). Do **not** set `KAFKA_BOOTSTRAP_SERVERS=127.0.0.1:19092` for gateway/worker containers.

## Compose: traces (Jaeger)

```sh
COMPOSE_PROFILES=obs ./scripts/compose_up.sh
```

Open http://127.0.0.1:16686 — search by service (`steering-gateway`, `steering-worker`) or tags `steering.correlation_id`, `policy.decision` (e.g. `deny`). With **`COMPOSE_PROFILES=stream,obs`**, Kafka carries W3C trace context in record headers so gateway and worker spans share a trace. Details: [`observability/README.md`](../observability/README.md).

## Ports (loopback)

| Port | Service |
|------|---------|
| 8080 | Gateway HTTP |
| 9090 | Worker HTTP |
| 8181 | OPA |
| 19092 | Kafka API (only with **stream** profile) |
| 9644 | Redpanda admin (stream profile) |
| 4318 | OTLP HTTP to collector (**obs** profile) |
| 16686 | Jaeger UI (**obs** profile) |

## Health checks

```sh
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:9090/healthz
curl -fsS http://127.0.0.1:8181/health
```

## Tests

| Command | Needs |
|---------|--------|
| `make test` | `go`, `curl`; `opa` optional; **no** Docker broker |
| `make test-stream` | Docker; pulls/starts Redpanda; full streaming smoke |
| `make test-obs` | Docker; stream + Jaeger; asserts traces in Jaeger API |

## Troubleshooting

- **`address already in use` on 8080 / 9090** — stop Compose (`./scripts/compose_down.sh`) or stop other listeners; `demo_local.sh` uses gateway **8081** on purpose to avoid clashing with Compose **8080**.
- **`503` on `POST /decide` with streaming** — Kafka publish failed (broker not ready, wrong bootstrap inside containers, or network). Confirm `COMPOSE_PROFILES=stream` and that you did not override `KAFKA_BOOTSTRAP_SERVERS` with a host-only address for services.
- **Stale shell env after streaming** — `unset KAFKA_BOOTSTRAP_SERVERS` before a fresh API-only `compose_up`.
- **Kind vs streaming** — `make kind-demo` is HTTP-only today; streaming parity is not deployed in Kind yet (see `docs/roadmap.md`).

## MCP mode (AI agent trust middleware)

The gateway binary embeds an MCP (Model Context Protocol) server. When launched with `--mcp`, it speaks JSON-RPC over stdio and exposes three tools to any MCP-compatible AI client.

### Build

```sh
cd services/gateway
go build -o gateway .
```

### Tools exposed

| Tool | Purpose | Required inputs |
|------|---------|----------------|
| `request_work_order` | Request task-level authorization (pre-flight) | `action`, `resource` |
| `submit_evidence` | Report task completion/failure (post-flight) | `correlation_id`, `action`, `result` |
| `get_policy_status` | Check policy engine health | *(none)* |

### Quick test (pipe JSON-RPC over stdin)

```sh
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' \
  | ALLOW_LOCAL_POLICY_FALLBACK=true ./gateway --mcp 2>/dev/null
```

### Configure Cursor

Add to `~/.cursor/mcp.json`:

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

### Configure Claude Code

Add to `~/.claude/claude_desktop_config.json`:

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

### Configure VS Code Copilot

Add to `.vscode/settings.json`:

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

### Environment variables (MCP mode)

| Variable | Default | Purpose |
|----------|---------|--------|
| `ALLOW_LOCAL_POLICY_FALLBACK` | `false` | Enable built-in policy rules (no OPA needed) |
| `OPA_URL` | *(unset)* | Point to an OPA instance for real policy evaluation |
| `KAFKA_BOOTSTRAP_SERVERS` | *(unset)* | Publish decision traces and evidence to Kafka |
| `POLICY_BUNDLE_HASH` | `local-dev` | Reported in traces and policy status |

### Workflow

1. Agent calls `request_work_order` with `action: "build"`, `resource: "wiki"` → gets `correlation_id` + `decision: "allow"`.
2. Agent executes the task freely (e.g. inference against the modelbase).
3. Agent calls `submit_evidence` with the `correlation_id` and `result: "ok"` → evidence recorded.
4. Both events are stitchable by `correlation_id` for audit export.

---

## Related docs

- Roadmap and phases: `docs/roadmap.md`
- Compose env and topics: `infra/compose/README.md`
- Script index: `scripts/README.md`
- MCP protocol: https://modelcontextprotocol.io

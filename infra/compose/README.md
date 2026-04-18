# Docker Compose (primary local stack)

OPA, gateway, and worker run as containers with pinned images and a fixed Compose project name (`steering` via `name:` in `docker-compose.yml`).

## Default (core loop only)

From repo root:

```sh
./scripts/compose_up.sh
```

- Gateway: `http://127.0.0.1:8080` (`POST /decide`)
- Worker: `http://127.0.0.1:9090` (`POST /execute`)
- OPA: `http://127.0.0.1:8181` (for debugging)

`compose_up.sh` sets `POLICY_BUNDLE_HASH` from `policies/opa/rego/decision.rego` (same idea as `scripts/kind_deploy.sh`).

## Stream profile (Kafka / Redpanda)

Single-node Redpanda with **internal** and **external** Kafka listeners so processes **inside** the Compose network use `redpanda:9092`, and tools on the **host** use `127.0.0.1:19092` (metadata advertises `127.0.0.1:19092` for the external listener).

```sh
COMPOSE_PROFILES=stream ./scripts/compose_up.sh
```

When the stream profile is active, `compose_up.sh` sets **`KAFKA_BOOTSTRAP_SERVERS=redpanda:9092`** for gateway and worker (do not point containers at `127.0.0.1:19092`).

- Kafka API (host): `127.0.0.1:19092`
- Kafka API (containers): `redpanda:9092`
- Admin API: `127.0.0.1:9644`

Topics (created automatically on first use; `compose_up` also runs `rpk topic create` when available):

- `decision.trace.v1` — gateway produces after each successful `/decide`
- `decision.evidence.v1` — worker produces after consuming a trace

Env (override via shell before `compose up` if needed):

| Variable | Default | Service |
|----------|---------|---------|
| `KAFKA_TOPIC_DECISION_TRACE` | `decision.trace.v1` | gateway, worker |
| `KAFKA_TOPIC_EVIDENCE` | `decision.evidence.v1` | worker |
| `KAFKA_CONSUMER_GROUP` | `steering-worker` | worker |

End-to-end demo (host reads evidence via `tools/kafka-read-one`):

```sh
COMPOSE_PROFILES=stream ./scripts/compose_up.sh
./scripts/demo_stream.sh
```

## Stop / reset

```sh
./scripts/compose_down.sh
```

Remove containers and named volumes (e.g. Redpanda data):

```sh
./scripts/compose_down.sh --volumes
```

## Hygiene

- Bind ports to loopback only (`127.0.0.1`) so nothing listens on all interfaces by default.
- Use one stack at a time: avoid running this alongside another process bound to `8080`/`9090`/`8181`/`19092`.

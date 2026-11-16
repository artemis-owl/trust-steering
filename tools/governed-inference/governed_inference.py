#!/usr/bin/env python3
"""A consumer that makes governed inference calls through steering's MCP tools.

It asks steering for a work order for a route off the machine (default
anthropic/claude-haiku-5-5) for a request of the declared data classification. If policy
allows it (e.g. --classification internal), the consumer itself makes one call to the
Anthropic Messages API (--offsite-url, key from ANTHROPIC_API_KEY only, billed to the key's
owner) and reports it with submit_evidence; the evidence carries token counts, never the
prompt, the reply or the key. On any other decision nothing is called.

Unless --offsite-only is given, it then asks for a work order for the local route and, on
allow, calls the local OpenAI-compatible server and reports that call the same way.
--no-call rehearses both: work orders and evidence (result=skipped), no model call.
Steering never sees the inference traffic.

Standard library only. The gateway binary is spoken to over stdio (JSON-RPC, MCP).
"""
import argparse, json, os, subprocess, sys, time, urllib.error, urllib.request

ANTHROPIC_VERSION = "2026-10-21"


def call_offsite(url, route, prompt, key):
    """One Messages API call. Returns (result, summary); never the prompt, reply or key."""
    model = route.removeprefix("anthropic/")
    body = json.dumps({"model": model, "max_tokens": 1024,
                       "messages": [{"role": "user", "content": prompt}]}).encode()
    req = urllib.request.Request(url.rstrip("/") + "/v1/messages", data=body, method="POST",
                                 headers={"Content-Type": "application/json",
                                          "x-api-key": key,
                                          "anthropic-version": ANTHROPIC_VERSION})
    t0 = time.monotonic()
    try:
        with urllib.request.urlopen(req, timeout=300) as r:
            reply = json.load(r)
    except urllib.error.HTTPError as e:
        return "error", f"route={route} http_status={e.code}"
    ms = int((time.monotonic() - t0) * 1000)
    usage = reply.get("usage", {})
    stop = reply.get("stop_reason")
    summary = (f"route={route} model={reply.get('model', model)} "
               f"input_tokens={usage.get('input_tokens')} "
               f"output_tokens={usage.get('output_tokens')} latency_ms={ms} stop_reason={stop}")
    return ("error" if stop == "refusal" else "ok"), summary


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--gateway", required=True, help="path to the steering gateway binary")
    ap.add_argument("--offsite-route", default="anthropic/claude-haiku-5-5")
    ap.add_argument("--classification", default="confidential")
    ap.add_argument("--model-url", default="http://127.0.0.1:8000/v1")
    ap.add_argument("--model", default="local")
    ap.add_argument("--prompt", default="Reply with the single word: governed")
    ap.add_argument("--offsite-url", default="https://api.anthropic.com",
                    help="base URL of the Anthropic Messages API")
    ap.add_argument("--offsite-only", action="store_true",
                    help="ask only for the off-machine work order, print the result and exit")
    ap.add_argument("--no-call", action="store_true", help="rehearsal: skip the model call and report result=skipped")
    a = ap.parse_args()

    gw = subprocess.Popen([a.gateway, "--mcp"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
    ids = iter(range(1, 1000))

    def rpc(method, params=None, notify=False):
        msg = {"jsonrpc": "2.0", "method": method}
        if params is not None:
            msg["params"] = params
        if not notify:
            msg["id"] = next(ids)
        gw.stdin.write(json.dumps(msg) + "\n"); gw.stdin.flush()
        if notify:
            return None
        while True:
            line = gw.stdout.readline()
            if not line:
                sys.exit("gateway closed the connection")
            resp = json.loads(line)
            if resp.get("id") == msg["id"]:
                return resp

    def tool(name, args):
        resp = rpc("tools/call", {"name": name, "arguments": args})
        if "error" in resp:
            sys.exit(f"{name}: {resp['error']}")
        res = resp["result"]
        text = "".join(c.get("text", "") for c in res.get("content", []))
        if res.get("isError"):
            sys.exit(f"{name}: {text}")
        return json.loads(text)

    rpc("initialize", {"protocolVersion": "2024-11-05", "capabilities": {},
                       "clientInfo": {"name": "governed-inference-consumer", "version": "0.1"}})
    rpc("notifications/initialized", notify=True)

    attrs = json.dumps({"data_classification": a.classification})
    out = {}

    context = f"one chat completion on {a.classification} data"
    offsite = tool("request_work_order", {"action": "inference", "resource": a.offsite_route,
                                          "environment": "dev", "attributes": attrs,
                                          "context": context})
    out["offsite"] = {"route": a.offsite_route, **offsite, "call_made": False}

    if offsite.get("decision") == "allow":
        key = os.environ.get("ANTHROPIC_API_KEY", "")
        if a.no_call:
            result, summary = "skipped", "rehearsal: no model call made"
        elif not key:
            result, summary = "skipped", "no ANTHROPIC_API_KEY in the environment: no model call made"
        else:
            result, summary = call_offsite(a.offsite_url, a.offsite_route, a.prompt, key)
            out["offsite"]["call_made"] = True
        out["offsite_evidence"] = tool("submit_evidence", {"correlation_id": offsite["correlation_id"],
                                                           "action": "inference", "result": result,
                                                           "summary": summary})
        out["offsite_evidence"]["summary"] = summary

    if a.offsite_only:
        gw.stdin.close(); gw.wait(timeout=10)
        print(json.dumps(out, indent=2))
        return

    allowed = tool("request_work_order", {"action": "inference", "resource": "local",
                                          "environment": "dev", "attributes": attrs,
                                          "context": context})
    out["local"] = {"route": "local", **allowed}

    permitted = allowed.get("decision") == "allow"
    if permitted:
        if a.no_call:
            result, summary = "skipped", "rehearsal: no model call made"
        else:
            body = json.dumps({"model": a.model, "max_tokens": 16,
                               "messages": [{"role": "user", "content": a.prompt}]}).encode()
            req = urllib.request.Request(a.model_url.rstrip("/") + "/chat/completions", data=body,
                                         headers={"Content-Type": "application/json"})
            t0 = time.monotonic()
            with urllib.request.urlopen(req, timeout=300) as r:
                reply = json.load(r)
            ms = int((time.monotonic() - t0) * 1000)
            usage = reply.get("usage", {})
            result = "ok"
            summary = (f"route=local model={reply.get('model', a.model)} "
                       f"prompt_tokens={usage.get('prompt_tokens')} "
                       f"completion_tokens={usage.get('completion_tokens')} latency_ms={ms}")
            out["local"]["call_made"] = True
        out["evidence"] = tool("submit_evidence", {"correlation_id": allowed["correlation_id"],
                                                   "action": "inference", "result": result,
                                                   "summary": summary})
        out["evidence"]["summary"] = summary
    else:
        out["local"]["call_made"] = False

    gw.stdin.close(); gw.wait(timeout=10)
    print(json.dumps(out, indent=2))

if __name__ == "__main__":
    main()

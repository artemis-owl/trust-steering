#!/usr/bin/env python3
"""A consumer that makes one governed inference call through steering's MCP tools.

It asks steering for a work order twice for the same confidential request: once for a
route off the machine (expected: deny, and no call is made) and once for the local route
(expected: allow). On allow, the consumer itself calls the local OpenAI-compatible server
and reports the call with submit_evidence. Steering never sees the inference traffic.

Standard library only. The gateway binary is spoken to over stdio (JSON-RPC, MCP).
"""
import argparse, json, subprocess, sys, time, urllib.request

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--gateway", required=True, help="path to the steering gateway binary")
    ap.add_argument("--offsite-route", default="anthropic/claude-haiku-5-5")
    ap.add_argument("--classification", default="confidential")
    ap.add_argument("--model-url", default="http://127.0.0.1:8000/v1")
    ap.add_argument("--model", default="local")
    ap.add_argument("--prompt", default="Reply with the single word: governed")
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

    denied = tool("request_work_order", {"action": "inference", "resource": a.offsite_route,
                                         "environment": "dev", "attributes": attrs,
                                         "context": "one chat completion on confidential data"})
    out["offsite"] = {"route": a.offsite_route, **denied, "call_made": False}

    allowed = tool("request_work_order", {"action": "inference", "resource": "local",
                                          "environment": "dev", "attributes": attrs,
                                          "context": "one chat completion on confidential data"})
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

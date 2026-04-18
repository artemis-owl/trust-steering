package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHandleDecideUsesOPA(t *testing.T) {
	opa := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/data/steering/decision/decision" {
			t.Fatalf("unexpected OPA path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"allow":true,"rationale":[{"code":"ALLOW_DEV","message":"Allowed by OPA."}]}}`))
	}))
	defer opa.Close()

	t.Setenv("OPA_URL", opa.URL)
	t.Setenv("POLICY_BUNDLE_HASH", "sha256:test-policy")
	t.Setenv("POLICY_BUNDLE_VERSION", "0.1.0")
	t.Setenv("ALLOW_LOCAL_POLICY_FALLBACK", "")

	body := `{
		"subject":{"type":"user","id":"user:alice","attributes":{"role":"developer"}},
		"request":{"action":"deploy","resource":"service:gateway","environment":"dev","attributes":{"change_ticket":"CHG-0001"}},
		"data_classification":"internal"
	}`

	req := httptest.NewRequest(http.MethodPost, "/decide", strings.NewReader(body))
	rec := httptest.NewRecorder()

	newHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var trace DecisionTrace
	if err := json.NewDecoder(rec.Body).Decode(&trace); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if trace.Policy.Engine != "opa" {
		t.Fatalf("expected OPA engine, got %q", trace.Policy.Engine)
	}
	if trace.Policy.BundleHash != "sha256:test-policy" {
		t.Fatalf("unexpected bundle hash: %q", trace.Policy.BundleHash)
	}
	if trace.Policy.Decision != "allow" {
		t.Fatalf("unexpected decision: %q", trace.Policy.Decision)
	}
	if trace.CorrelationID == "" || trace.TraceID == "" || trace.SpanID == nil || *trace.SpanID == "" {
		t.Fatalf("expected generated correlation and trace identifiers")
	}
}

func TestHandleDecideRequiresOPAOrFallback(t *testing.T) {
	t.Setenv("OPA_URL", "")
	t.Setenv("ALLOW_LOCAL_POLICY_FALLBACK", "")

	body := `{
		"subject":{"type":"user","id":"user:alice","attributes":{"role":"developer"}},
		"request":{"action":"deploy","resource":"service:gateway","environment":"dev"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/decide", strings.NewReader(body))
	rec := httptest.NewRecorder()

	newHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDecideAllowsFallbackWhenEnabled(t *testing.T) {
	t.Setenv("OPA_URL", "")
	t.Setenv("ALLOW_LOCAL_POLICY_FALLBACK", "true")
	t.Setenv("POLICY_BUNDLE_HASH", "local-dev")

	body := `{
		"subject":{"type":"user","id":"user:alice","attributes":{"role":"developer"}},
		"request":{"action":"deploy","resource":"service:gateway","environment":"dev"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/decide", strings.NewReader(body))
	rec := httptest.NewRecorder()

	newHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var trace DecisionTrace
	if err := json.NewDecoder(rec.Body).Decode(&trace); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if trace.Policy.Engine != "local" {
		t.Fatalf("expected local fallback, got %q", trace.Policy.Engine)
	}
}

func TestHandleDecideRecordsSpansWithPolicyAttribute(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	opa := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/data/steering/decision/decision" {
			t.Fatalf("unexpected OPA path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"allow":false,"rationale":[{"code":"DENY","message":"no."}]}}`))
	}))
	defer opa.Close()

	t.Setenv("OPA_URL", opa.URL)
	t.Setenv("POLICY_BUNDLE_HASH", "sha256:test-policy")
	t.Setenv("POLICY_BUNDLE_VERSION", "0.1.0")
	t.Setenv("ALLOW_LOCAL_POLICY_FALLBACK", "")

	body := `{
		"subject":{"type":"user","id":"user:alice","attributes":{"role":"developer"}},
		"request":{"action":"deploy","resource":"service:gateway","environment":"dev"},
		"data_classification":"internal"
	}`

	req := httptest.NewRequest(http.MethodPost, "/decide", strings.NewReader(body))
	rec := httptest.NewRecorder()

	otelHandler("steering-gateway", newHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	ended := sr.Ended()
	if len(ended) == 0 {
		t.Fatal("expected at least one ended span")
	}
	var sawPolicy bool
	for _, sp := range ended {
		for _, kv := range sp.Attributes() {
			if string(kv.Key) == "policy.decision" {
				sawPolicy = true
			}
		}
	}
	if !sawPolicy {
		t.Fatalf("expected policy.decision on a span, got %d spans", len(ended))
	}
}

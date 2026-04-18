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

func TestHandleExecuteAllowProducesEvidence(t *testing.T) {
	body := `{
		"schema_version":"0.1.0",
		"event_id":"evt-1",
		"event_time":"2026-03-07T00:00:00Z",
		"correlation_id":"corr-123",
		"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736",
		"span_id":"00f067aa0ba902b7",
		"subject":{"type":"user","id":"user:alice","attributes":{"role":"developer"}},
		"request":{"action":"deploy","resource":"service:gateway","environment":"dev"},
		"policy":{"engine":"opa","bundle_hash":"sha256:test-policy","decision":"allow"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/execute", strings.NewReader(body))
	rec := httptest.NewRecorder()

	newHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var event EvidenceEvent
	if err := json.NewDecoder(rec.Body).Decode(&event); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if event.EvidenceType != "action.executed" || event.Result != "ok" {
		t.Fatalf("unexpected evidence outcome: %#v", event)
	}
	if event.CorrelationID != "corr-123" || event.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("expected trace identifiers to be preserved")
	}
}

func TestHandleExecuteRejectsIncompleteTrace(t *testing.T) {
	body := `{
		"correlation_id":"corr-123",
		"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736",
		"request":{"action":"deploy","resource":"service:gateway","environment":"dev"},
		"policy":{"decision":"allow"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/execute", strings.NewReader(body))
	rec := httptest.NewRecorder()

	newHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleExecuteRecordsSpanAttributes(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	body := `{
		"schema_version":"0.1.0",
		"event_id":"evt-1",
		"event_time":"2026-03-07T00:00:00Z",
		"correlation_id":"corr-123",
		"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736",
		"span_id":"00f067aa0ba902b7",
		"subject":{"type":"user","id":"user:alice","attributes":{"role":"developer"}},
		"request":{"action":"deploy","resource":"service:gateway","environment":"dev"},
		"policy":{"engine":"opa","bundle_hash":"sha256:test-policy","decision":"allow"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/execute", strings.NewReader(body))
	rec := httptest.NewRecorder()

	otelHandler("steering-worker", newHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	ended := sr.Ended()
	if len(ended) == 0 {
		t.Fatal("expected at least one ended span")
	}
	var sawCorr bool
	for _, sp := range ended {
		for _, kv := range sp.Attributes() {
			if string(kv.Key) == "steering.correlation_id" {
				sawCorr = true
			}
		}
	}
	if !sawCorr {
		t.Fatalf("expected steering.correlation_id on a span, got %d spans", len(ended))
	}
}

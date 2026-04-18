package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

type Subject struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type Request struct {
	Action      string            `json:"action"`
	Resource    string            `json:"resource"`
	Environment string            `json:"environment"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

type PolicyDecision struct {
	Engine        string  `json:"engine"`
	BundleVersion *string `json:"bundle_version,omitempty"`
	BundleHash    string  `json:"bundle_hash"`
	Decision      string  `json:"decision"`
}

type DecisionTrace struct {
	SchemaVersion string         `json:"schema_version"`
	EventID       string         `json:"event_id"`
	EventTime     string         `json:"event_time"`
	CorrelationID string         `json:"correlation_id"`
	TraceID       string         `json:"trace_id"`
	SpanID        *string        `json:"span_id,omitempty"`
	Subject       Subject        `json:"subject"`
	Request       Request        `json:"request"`
	Policy        PolicyDecision `json:"policy"`
}

type EvidenceEvent struct {
	SchemaVersion string            `json:"schema_version"`
	EventID       string            `json:"event_id"`
	EventTime     string            `json:"event_time"`
	CorrelationID string            `json:"correlation_id"`
	TraceID       string            `json:"trace_id"`
	SpanID        *string           `json:"span_id,omitempty"`
	EvidenceType  string            `json:"evidence_type"`
	Action        string            `json:"action"`
	Result        string            `json:"result"`
	Details       map[string]string `json:"details,omitempty"`
	Workload      WorkloadIdentity  `json:"workload"`
}

type WorkloadIdentity struct {
	ServiceName string  `json:"service_name"`
	Version     *string `json:"version,omitempty"`
	ImageDigest *string `json:"image_digest,omitempty"`
}

func main() {
	ctx := context.Background()

	shutdown, err := initOTel(ctx, envOr("OTEL_SERVICE_NAME", "steering-worker"))
	if err != nil {
		panic("otel: " + err.Error())
	}
	defer func() { _ = shutdown(context.Background()) }()

	kafkaCl, err := newKafkaWorkerClient()
	if err != nil {
		panic("kafka: " + err.Error())
	}
	if kafkaCl != nil {
		defer kafkaCl.Close()
		go runKafkaConsumer(ctx, kafkaCl)
	}

	addr := ":" + envOr("PORT", "9090")
	server := &http.Server{
		Addr:              addr,
		Handler:           otelHandler(envOr("OTEL_SERVICE_NAME", "steering-worker"), newHandler()),
		ReadHeaderTimeout: 5 * time.Second,
	}
	_ = server.ListenAndServe()
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/execute", handleExecute)
	return mux
}

func handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var trace DecisionTrace
	if err := json.NewDecoder(r.Body).Decode(&trace); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	annotateExecuteSpan(r.Context(), trace.CorrelationID, trace.Policy.Decision)

	evidence, err := traceToEvidence(trace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, evidence)
}

func traceToEvidence(trace DecisionTrace) (EvidenceEvent, error) {
	if err := validateTrace(trace); err != nil {
		return EvidenceEvent{}, err
	}

	decision := strings.ToLower(trace.Policy.Decision)
	result := "skipped"
	evidenceType := "action.skipped"
	if decision == "allow" {
		result = "ok"
		evidenceType = "action.executed"
	}

	details := map[string]string{
		"policy_decision": decision,
		"resource":        trace.Request.Resource,
		"environment":     trace.Request.Environment,
	}

	return EvidenceEvent{
		SchemaVersion: envOr("EVIDENCE_SCHEMA_VERSION", "0.1.0"),
		EventID:       randomHex(16),
		EventTime:     time.Now().UTC().Format(time.RFC3339Nano),
		CorrelationID: trace.CorrelationID,
		TraceID:       trace.TraceID,
		SpanID:        trace.SpanID,
		EvidenceType:  evidenceType,
		Action:        trace.Request.Action,
		Result:        result,
		Details:       details,
		Workload:      workloadIdentity(),
	}, nil
}

func validateTrace(trace DecisionTrace) error {
	if trace.SchemaVersion == "" || trace.EventID == "" || trace.EventTime == "" {
		return errors.New("schema_version, event_id, and event_time are required")
	}
	if trace.CorrelationID == "" || trace.TraceID == "" {
		return errors.New("correlation_id and trace_id are required")
	}
	if trace.Subject.Type == "" || trace.Subject.ID == "" {
		return errors.New("subject.type and subject.id are required")
	}
	if trace.Request.Action == "" || trace.Request.Resource == "" || trace.Request.Environment == "" {
		return errors.New("request.action, request.resource, request.environment are required")
	}
	if trace.Policy.Engine == "" || trace.Policy.BundleHash == "" || trace.Policy.Decision == "" {
		return errors.New("policy.engine, policy.bundle_hash, and policy.decision are required")
	}
	switch strings.ToLower(trace.Policy.Decision) {
	case "allow", "deny":
	default:
		return errors.New("policy.decision must be allow or deny")
	}
	return nil
}

func workloadIdentity() WorkloadIdentity {
	service := envOr("SERVICE_NAME", "worker")
	version := envOr("SERVICE_VERSION", "0.1.0")
	imageDigest := os.Getenv("IMAGE_DIGEST")

	var versionPtr *string
	if version != "" {
		versionPtr = &version
	}
	var digestPtr *string
	if imageDigest != "" {
		digestPtr = &imageDigest
	}

	return WorkloadIdentity{
		ServiceName: service,
		Version:     versionPtr,
		ImageDigest: digestPtr,
	}
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(payload)
}

func randomHex(bytesLen int) string {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func envOr(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

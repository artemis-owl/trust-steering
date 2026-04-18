package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var errPolicyUnavailable = errors.New("policy engine unavailable")

var opaHTTPClient = &http.Client{
	Transport: otelhttp.NewTransport(http.DefaultTransport),
}

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

type RationaleItem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PolicyDecision struct {
	Engine        string          `json:"engine"`
	BundleVersion *string         `json:"bundle_version,omitempty"`
	BundleHash    string          `json:"bundle_hash"`
	Decision      string          `json:"decision"`
	Rationale     []RationaleItem `json:"rationale,omitempty"`
}

type DecisionTrace struct {
	SchemaVersion      string         `json:"schema_version"`
	EventID            string         `json:"event_id"`
	EventTime          string         `json:"event_time"`
	CorrelationID      string         `json:"correlation_id"`
	TraceID            string         `json:"trace_id"`
	SpanID             *string        `json:"span_id,omitempty"`
	Subject            Subject        `json:"subject"`
	Request            Request        `json:"request"`
	Policy             PolicyDecision `json:"policy"`
	DataClassification string         `json:"data_classification"`
	PiiFlags           []string       `json:"pii_flags,omitempty"`
	EvidenceRefs       []string       `json:"evidence_refs,omitempty"`
}

type DecisionRequest struct {
	CorrelationID      string   `json:"correlation_id,omitempty"`
	TraceID            string   `json:"trace_id,omitempty"`
	SpanID             string   `json:"span_id,omitempty"`
	Subject            Subject  `json:"subject"`
	Request            Request  `json:"request"`
	DataClassification string   `json:"data_classification,omitempty"`
	PiiFlags           []string `json:"pii_flags,omitempty"`
}

type opaQuery struct {
	Input any `json:"input"`
}

type opaResponse struct {
	Result struct {
		Allow     bool            `json:"allow"`
		Rationale []RationaleItem `json:"rationale"`
	} `json:"result"`
}

var kafkaClient *kgo.Client

func isMCPMode() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--mcp" {
			return true
		}
	}
	return false
}

func main() {
	ctx := context.Background()
	shutdown, err := initOTel(ctx, envOr("OTEL_SERVICE_NAME", "steering-gateway"))
	if err != nil {
		panic("otel: " + err.Error())
	}
	defer func() { _ = shutdown(context.Background()) }()

	cl, err := newKafkaProducer()
	if err != nil {
		panic("kafka producer: " + err.Error())
	}
	kafkaClient = cl
	if kafkaClient != nil {
		defer kafkaClient.Close()
	}

	if isMCPMode() {
		// MCP mode: speak JSON-RPC over stdio
		mcpServer := newMCPServer()
		if err := server.ServeStdio(mcpServer); err != nil {
			panic("mcp server: " + err.Error())
		}
		return
	}

	// HTTP mode: standard gateway server
	addr := ":" + envOr("PORT", "8080")
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           otelHandler(envOr("OTEL_SERVICE_NAME", "steering-gateway"), newHandler()),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/decide", handleDecide)
	return mux
}

func handleDecide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req DecisionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := validateRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	policyDecision, err := evaluatePolicy(r.Context(), req.Subject, req.Request)
	if err != nil {
		if errors.Is(err, errPolicyUnavailable) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "policy evaluation failed", http.StatusBadGateway)
		return
	}

	correlationID := req.CorrelationID
	if correlationID == "" {
		correlationID = "corr-" + randomHex(8)
	}

	traceID, spanID := traceIDsForDecisionTrace(r.Context(), req.TraceID, req.SpanID)
	annotateDecideSpan(r.Context(), correlationID, policyDecision.Decision)

	dataClassification := req.DataClassification
	if dataClassification == "" {
		dataClassification = "internal"
	}

	decisionTrace := DecisionTrace{
		SchemaVersion:      envOr("SCHEMA_VERSION", "0.1.0"),
		EventID:            randomHex(16),
		EventTime:          time.Now().UTC().Format(time.RFC3339Nano),
		CorrelationID:      correlationID,
		TraceID:            traceID,
		SpanID:             &spanID,
		Subject:            req.Subject,
		Request:            req.Request,
		Policy:             policyDecision,
		DataClassification: dataClassification,
		PiiFlags:           req.PiiFlags,
		EvidenceRefs:       []string{},
	}

	if kafkaClient != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := publishDecisionTrace(ctx, kafkaClient, decisionTrace, correlationID); err != nil {
			http.Error(w, "failed to publish decision trace", http.StatusServiceUnavailable)
			return
		}
	}

	writeJSON(w, decisionTrace)
}

func validateRequest(req DecisionRequest) error {
	if req.Subject.Type == "" || req.Subject.ID == "" {
		return errors.New("subject.type and subject.id are required")
	}
	if req.Request.Action == "" || req.Request.Resource == "" || req.Request.Environment == "" {
		return errors.New("request.action, request.resource, request.environment are required")
	}
	return nil
}

func evaluatePolicy(ctx context.Context, subject Subject, request Request) (PolicyDecision, error) {
	opaURL := strings.TrimRight(os.Getenv("OPA_URL"), "/")
	if opaURL != "" {
		return evaluatePolicyOPA(ctx, opaURL, subject, request)
	}
	if localPolicyFallbackEnabled() {
		return evaluatePolicyLocal(subject, request), nil
	}
	return PolicyDecision{}, errPolicyUnavailable
}

func evaluatePolicyOPA(ctx context.Context, opaURL string, subject Subject, request Request) (PolicyDecision, error) {
	input := map[string]any{
		"subject": subject,
		"request": request,
	}
	body, _ := json.Marshal(opaQuery{Input: input})
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, opaURL+"/v1/data/steering/decision/decision", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := opaHTTPClient.Do(req)
	if err != nil {
		return PolicyDecision{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return PolicyDecision{}, errors.New("opa error")
	}

	var opaResp opaResponse
	if err := json.NewDecoder(resp.Body).Decode(&opaResp); err != nil {
		return PolicyDecision{}, err
	}

	decision := "deny"
	if opaResp.Result.Allow {
		decision = "allow"
	}

	bundleVersion := envOr("POLICY_BUNDLE_VERSION", "")
	var bundleVersionPtr *string
	if bundleVersion != "" {
		bundleVersionPtr = &bundleVersion
	}

	return PolicyDecision{
		Engine:        "opa",
		BundleVersion: bundleVersionPtr,
		BundleHash:    envOr("POLICY_BUNDLE_HASH", "local-dev"),
		Decision:      decision,
		Rationale:     opaResp.Result.Rationale,
	}, nil
}

func evaluatePolicyLocal(subject Subject, request Request) PolicyDecision {
	allow := false
	rationale := []RationaleItem{{Code: "DEFAULT_DENY", Message: "Denied by default."}}

	if request.Action == "build" {
		allow = true
		rationale = []RationaleItem{{Code: "ALLOW_BUILD", Message: "Local builds are unrestricted."}}
	} else if request.Action == "deploy" && request.Environment == "dev" && request.Attributes["tests_passed"] == "true" {
		allow = true
		rationale = []RationaleItem{{Code: "ALLOW_DEPLOY_DEV", Message: "Dev deploy permitted because tests passed."}}
	} else if request.Action == "deploy" && request.Environment == "prod" && request.Attributes["human_approved"] == "true" {
		allow = true
		rationale = []RationaleItem{{Code: "ALLOW_DEPLOY_PROD", Message: "Prod deploy permitted by explicit human approval."}}
	}

	decision := "deny"
	if allow {
		decision = "allow"
	}

	bundleVersion := envOr("POLICY_BUNDLE_VERSION", "")
	var bundleVersionPtr *string
	if bundleVersion != "" {
		bundleVersionPtr = &bundleVersion
	}

	return PolicyDecision{
		Engine:        "local",
		BundleVersion: bundleVersionPtr,
		BundleHash:    envOr("POLICY_BUNDLE_HASH", "local-dev"),
		Decision:      decision,
		Rationale:     rationale,
	}
}

func isAllowedAction(action string) bool {
	switch strings.ToLower(action) {
	case "deploy", "read", "inference", "build", "refactor":
		return true
	default:
		return false
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

func localPolicyFallbackEnabled() bool {
	return strings.EqualFold(os.Getenv("ALLOW_LOCAL_POLICY_FALLBACK"), "true")
}

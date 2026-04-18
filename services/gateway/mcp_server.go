package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// EvidenceEvent matches schemas/evidence.avsc
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
	Details       map[string]string `json:"details"`
	Workload      WorkloadIdentity  `json:"workload"`
}

type WorkloadIdentity struct {
	ServiceName string  `json:"service_name"`
	Version     *string `json:"version,omitempty"`
	ImageDigest *string `json:"image_digest,omitempty"`
}

func newMCPServer() *server.MCPServer {
	s := server.NewMCPServer(
		"steering",
		"0.1.0",
		server.WithToolCapabilities(false),
	)

	// --- Tool: request_work_order ---
	requestWorkOrder := mcp.NewTool("request_work_order",
		mcp.WithDescription(
			"Request authorization to execute a task. Call this BEFORE starting work. "+
				"Returns a correlation_id to carry through the task and use when submitting evidence.",
		),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Task action, e.g. build, deploy, inference, refactor"),
		),
		mcp.WithString("resource",
			mcp.Required(),
			mcp.Description("Target resource, e.g. wiki, cluster, modelbase, repository"),
		),
		mcp.WithString("environment",
			mcp.Description("Environment: dev, staging, or prod. Defaults to dev."),
		),
		mcp.WithString("context",
			mcp.Description("Free-text description of what the agent plans to do"),
		),
		mcp.WithString("attributes",
			mcp.Description("Optional structured JSON string of key-value attributes (e.g. {\"tests_passed\":\"true\"})"),
		),
	)

	s.AddTool(requestWorkOrder, handleRequestWorkOrder)

	// --- Tool: submit_evidence ---
	submitEvidence := mcp.NewTool("submit_evidence",
		mcp.WithDescription(
			"Report task completion or failure for audit. Call this AFTER finishing work. "+
				"Ties to a previous work order via the correlation_id returned by request_work_order.",
		),
		mcp.WithString("correlation_id",
			mcp.Required(),
			mcp.Description("The correlation_id returned by request_work_order"),
		),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action that was executed, e.g. build, deploy"),
		),
		mcp.WithString("result",
			mcp.Required(),
			mcp.Description("Outcome: ok, error, or skipped"),
		),
		mcp.WithString("summary",
			mcp.Description("Brief summary of what happened during the task"),
		),
	)

	s.AddTool(submitEvidence, handleSubmitEvidence)

	// --- Tool: get_policy_status ---
	getPolicyStatus := mcp.NewTool("get_policy_status",
		mcp.WithDescription(
			"Check if the steering policy engine is available and what policy bundle is loaded.",
		),
	)

	s.AddTool(getPolicyStatus, handleGetPolicyStatus)

	return s
}

func handleRequestWorkOrder(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, span := otel.Tracer("steering.mcp").Start(ctx, "mcp.request_work_order")
	defer span.End()

	action, err := request.RequireString("action")
	if err != nil {
		return mcp.NewToolResultError("action is required"), nil
	}
	resource, err := request.RequireString("resource")
	if err != nil {
		return mcp.NewToolResultError("resource is required"), nil
	}

	environment := "dev"
	if env, err := request.RequireString("environment"); err == nil && env != "" {
		environment = env
	}

	taskContext := ""
	if c, err := request.RequireString("context"); err == nil {
		taskContext = c
	}

	attributesJSON := ""
	if a, err := request.RequireString("attributes"); err == nil {
		attributesJSON = a
	}

	subject := Subject{
		Type:       "agent",
		ID:         "mcp-client",
		Attributes: map[string]string{"role": "developer"},
	}

	reqObj := Request{
		Action:      action,
		Resource:    resource,
		Environment: environment,
	}
	attrs := make(map[string]string)
	if taskContext != "" {
		attrs["context"] = taskContext
	}
	if attributesJSON != "" {
		var parsed map[string]string
		if err := json.Unmarshal([]byte(attributesJSON), &parsed); err == nil {
			for k, v := range parsed {
				attrs[k] = v
			}
		}
	}
	if len(attrs) > 0 {
		reqObj.Attributes = attrs
	}

	policyDecision, err := evaluatePolicy(ctx, subject, reqObj)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Policy evaluation failed: %v", err)), nil
	}

	correlationID := "corr-" + randomHex(8)
	traceID := randomHex(16)
	spanID := randomHex(8)

	decisionTrace := DecisionTrace{
		SchemaVersion:      envOr("SCHEMA_VERSION", "0.1.0"),
		EventID:            randomHex(16),
		EventTime:          time.Now().UTC().Format(time.RFC3339Nano),
		CorrelationID:      correlationID,
		TraceID:            traceID,
		SpanID:             &spanID,
		Subject:            subject,
		Request:            reqObj,
		Policy:             policyDecision,
		DataClassification: "internal",
		PiiFlags:           []string{},
		EvidenceRefs:       []string{},
	}

	// Publish to Kafka if available
	if kafkaClient != nil {
		pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = publishDecisionTrace(pubCtx, kafkaClient, decisionTrace, correlationID)
	}

	// Build response
	result := map[string]any{
		"correlation_id": correlationID,
		"decision":       policyDecision.Decision,
		"rationale":      policyDecision.Rationale,
		"event_id":       decisionTrace.EventID,
		"trace_id":       traceID,
	}

	out, _ := json.MarshalIndent(result, "", "  ")

	span.SetAttributes(
		attribute.String("steering.correlation_id", correlationID),
		attribute.String("policy.decision", policyDecision.Decision),
		attribute.String("request.action", action),
		attribute.String("request.resource", resource),
		attribute.String("request.environment", environment),
		attribute.String("request.context", taskContext),
	)

	return mcp.NewToolResultText(string(out)), nil
}

func handleSubmitEvidence(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, span := otel.Tracer("steering.mcp").Start(ctx, "mcp.submit_evidence")
	defer span.End()

	correlationID, err := request.RequireString("correlation_id")
	if err != nil {
		return mcp.NewToolResultError("correlation_id is required"), nil
	}
	action, err := request.RequireString("action")
	if err != nil {
		return mcp.NewToolResultError("action is required"), nil
	}
	result, err := request.RequireString("result")
	if err != nil {
		return mcp.NewToolResultError("result is required"), nil
	}

	summary := ""
	if s, err := request.RequireString("summary"); err == nil {
		summary = s
	}

	details := map[string]string{}
	if summary != "" {
		details["summary"] = summary
	}

	evidence := EvidenceEvent{
		SchemaVersion: envOr("SCHEMA_VERSION", "0.1.0"),
		EventID:       randomHex(16),
		EventTime:     time.Now().UTC().Format(time.RFC3339Nano),
		CorrelationID: correlationID,
		TraceID:       randomHex(16),
		EvidenceType:  "action.executed",
		Action:        action,
		Result:        result,
		Details:       details,
		Workload: WorkloadIdentity{
			ServiceName: "mcp-client",
		},
	}

	// Publish to Kafka if available
	if kafkaClient != nil {
		payload, _ := json.Marshal(evidence)
		pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = publishEvidence(pubCtx, kafkaClient, payload, correlationID)
	}

	out, _ := json.MarshalIndent(map[string]any{
		"event_id":       evidence.EventID,
		"correlation_id": correlationID,
		"status":         "recorded",
		"result":         result,
	}, "", "  ")

	span.SetAttributes(
		attribute.String("steering.correlation_id", correlationID),
		attribute.String("action.name", action),
		attribute.String("action.result", result),
		attribute.String("evidence.summary", summary),
	)

	return mcp.NewToolResultText(string(out)), nil
}

func handleGetPolicyStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	engine := "none"
	if envOr("OPA_URL", "") != "" {
		engine = "opa"
	} else if localPolicyFallbackEnabled() {
		engine = "local-fallback"
	}

	status := map[string]any{
		"gateway_healthy": true,
		"policy_engine":   engine,
		"bundle_hash":     envOr("POLICY_BUNDLE_HASH", "local-dev"),
		"kafka_connected": kafkaClient != nil,
	}

	out, _ := json.MarshalIndent(status, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

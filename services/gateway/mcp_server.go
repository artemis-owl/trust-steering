package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
		mcp.WithString("correlation_id",
			mcp.Description(
				"Only for retrying a work order that came back decision=deny with approval_status=pending: "+
					"pass the correlation_id from that response after a human has granted the approval out-of-band.",
			),
		),
		mcp.WithString("attributes",
			mcp.Description(
				"Optional structured JSON string of key-value attributes (e.g. {\"tests_passed\":\"true\"}). "+
					"Trust-reserved keys (role, agent_id, human_approved) are rejected: identity comes from the "+
					"gateway's own configuration and approvals must originate out-of-band.",
			),
		),
	)

	s.AddTool(requestWorkOrder, handleRequestWorkOrder)

	// --- Tool: submit_evidence ---
	submitEvidence := mcp.NewTool("submit_evidence",
		mcp.WithDescription(
			"Report task completion or failure for audit. Call this AFTER finishing work. "+
				"Ties to a previous work order via the correlation_id returned by request_work_order. "+
				"Rejected unless that work order was issued by this gateway, decided allow, and "+
				"covers the reported action.",
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

// isTrustReservedAttribute reports whether a client-supplied attribute key
// would let the governed agent assert its own identity or approval state.
func isTrustReservedAttribute(key string) bool {
	switch key {
	case "role", "agent_id", "human_approved":
		return true
	}
	return false
}

// traceClassifications are the data_classification values the decision
// trace schema admits (schemas/decision_trace.avsc).
var traceClassifications = map[string]bool{"public": true, "internal": true, "confidential": true, "restricted": true}

// traceDataClassification is the caller's declared data_classification when
// it is a schema value, else "internal".
func traceDataClassification(attrs map[string]string) string {
	if c := attrs["data_classification"]; traceClassifications[c] {
		return c
	}
	return "internal"
}

// mcpTraceObserver, when set (tests only), sees each decision trace built by
// request_work_order.
var mcpTraceObserver func(DecisionTrace)

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

	// Subject identity and role come from the gateway's own configuration.
	// The MCP client is the governed party and must not be able to assert
	// its own identity, role, or approval state.
	subject := Subject{
		Type:       "agent",
		ID:         envOr("MCP_AGENT_ID", "mcp-client"),
		Attributes: map[string]string{"role": envOr("MCP_SUBJECT_ROLE", "developer")},
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
		if err := json.Unmarshal([]byte(attributesJSON), &parsed); err != nil {
			return mcp.NewToolResultError("attributes must be a JSON object of string values"), nil
		}
		for k, v := range parsed {
			if isTrustReservedAttribute(k) {
				return mcp.NewToolResultError(fmt.Sprintf("attribute %q is trust-reserved and cannot be self-attested", k)), nil
			}
			attrs[k] = v
		}
	}
	if len(attrs) > 0 {
		reqObj.Attributes = attrs
	}

	correlationID := "corr-" + randomHex(8)
	if c, err := request.RequireString("correlation_id"); err == nil && c != "" {
		if !validCorrelationID(c) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid correlation_id %q", c)), nil
		}
		correlationID = c
	}

	// Approval state only enters policy input via the server-side store; the
	// agent cannot assert it (human_approved is trust-reserved above).
	approval := applyApprovalGate(correlationID, &reqObj)

	policyDecision, err := evaluatePolicy(ctx, subject, reqObj)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Policy evaluation failed: %v", err)), nil
	}

	approvalStatus := ""
	if policyDecision.Decision == "deny" && approval == nil && requiresHumanApproval(reqObj) {
		if status, item := approvalDenyFeedback(correlationID, reqObj); status != "" {
			approvalStatus = status
			policyDecision.Rationale = append(policyDecision.Rationale, item)
		}
	}

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
		DataClassification: traceDataClassification(reqObj.Attributes),
		PiiFlags:           []string{},
		EvidenceRefs:       []string{},
	}

	if mcpTraceObserver != nil {
		mcpTraceObserver(decisionTrace)
	}

	// When a broker is configured, failure to persist the trace must fail the
	// work order — same contract as the HTTP /decide path.
	if kafkaClient != nil {
		pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := publishDecisionTrace(pubCtx, kafkaClient, decisionTrace, correlationID); err != nil {
			span.RecordError(err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to persist decision trace, work order not authorized: %v", err)), nil
		}
	}

	// The store is what lets submit_evidence link back to this work order;
	// if it cannot be written the grant would be unlinkable, so fail closed.
	if err := recordWorkOrder(decisionTrace); err != nil {
		span.RecordError(err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to record work order, not authorized: %v", err)), nil
	}

	// Build response
	result := map[string]any{
		"correlation_id": correlationID,
		"decision":       policyDecision.Decision,
		"rationale":      policyDecision.Rationale,
		"event_id":       decisionTrace.EventID,
		"trace_id":       traceID,
	}
	if approvalStatus != "" {
		result["approval_status"] = approvalStatus
		switch approvalStatus {
		case "pending":
			result["approval_instructions"] = fmt.Sprintf(
				"A human must grant this out-of-band: ./scripts/approve.sh grant %s — then retry request_work_order with this correlation_id. Do not attempt to approve it yourself.",
				correlationID,
			)
		case "expired":
			result["approval_instructions"] = fmt.Sprintf(
				"The human grant expired before it was used. A human can re-grant it: ./scripts/approve.sh grant %s — then retry promptly with this correlation_id.",
				correlationID,
			)
		default:
			result["approval_instructions"] = "This work order cannot be revived. Request a new work order (fresh correlation_id) for this task."
		}
	}
	if approval != nil {
		result["approval_status"] = "consumed"
		result["approver"] = approval.Approver
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

	// Evidence must link to a previously issued, allowed work order covering
	// the claimed action. Fail-closed: unverifiable submissions are rejected
	// before anything is published.
	workOrder, err := validateEvidenceLink(correlationID, action)
	if err != nil {
		span.RecordError(err)
		return mcp.NewToolResultError(fmt.Sprintf("evidence rejected: %v", err)), nil
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

	// When a broker is configured, evidence must be durable before we report it recorded.
	if kafkaClient != nil {
		payload, err := json.Marshal(evidence)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to encode evidence: %v", err)), nil
		}
		pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := publishEvidence(pubCtx, kafkaClient, payload, correlationID); err != nil {
			span.RecordError(err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to persist evidence event: %v", err)), nil
		}
	}

	// The evidence is durable; closure bookkeeping failure must not unrecord it.
	if err := closeWorkOrder(workOrder, result); err != nil {
		span.RecordError(err)
		fmt.Fprintf(os.Stderr, "workorders: record closure of %s: %v\n", correlationID, err)
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

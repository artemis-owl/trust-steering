package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func linkageEnv(t *testing.T) {
	t.Helper()
	t.Setenv("WORKORDERS_DIR", t.TempDir())
	t.Setenv("APPROVALS_DIR", t.TempDir())
	t.Setenv("OPA_URL", "")
	t.Setenv("ALLOW_LOCAL_POLICY_FALLBACK", "true")
}

// issueWorkOrder requests a work order over the MCP handler and returns the
// parsed result (correlation_id, decision, ...).
func issueWorkOrder(t *testing.T, args map[string]any) map[string]any {
	t.Helper()
	res := callTool(t, handleRequestWorkOrder, args)
	if res.IsError {
		t.Fatalf("request_work_order failed: %s", toolText(t, res))
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(toolText(t, res)), &parsed); err != nil {
		t.Fatalf("parse work order result: %v", err)
	}
	return parsed
}

func TestSubmitEvidenceRequiresIssuedWorkOrder(t *testing.T) {
	linkageEnv(t)

	res := callTool(t, handleSubmitEvidence, map[string]any{
		"correlation_id": "corr-" + randomHex(8),
		"action":         "build",
		"result":         "ok",
	})
	if !res.IsError {
		t.Fatal("evidence with an unissued correlation_id must be rejected")
	}
	if !strings.Contains(toolText(t, res), "no issued work order") {
		t.Fatalf("expected unknown-work-order rejection, got: %s", toolText(t, res))
	}
}

func TestSubmitEvidenceRejectsMalformedCorrelationID(t *testing.T) {
	linkageEnv(t)

	for _, id := range []string{"corr-XYZ", "../escape", "task-1", "corr-" + randomHex(8) + "/../x"} {
		res := callTool(t, handleSubmitEvidence, map[string]any{
			"correlation_id": id,
			"action":         "build",
			"result":         "ok",
		})
		if !res.IsError {
			t.Fatalf("evidence with correlation_id %q must be rejected", id)
		}
		if !strings.Contains(toolText(t, res), "invalid correlation_id") {
			t.Fatalf("expected invalid-correlation_id rejection for %q, got: %s", id, toolText(t, res))
		}
	}
}

func TestSubmitEvidenceAcceptsAndClosesIssuedWorkOrder(t *testing.T) {
	linkageEnv(t)

	issued := issueWorkOrder(t, map[string]any{
		"action":   "build",
		"resource": "repository",
	})
	if issued["decision"] != "allow" {
		t.Fatalf("expected allow, got %+v", issued)
	}
	correlationID := issued["correlation_id"].(string)

	w, err := readWorkOrder(correlationID)
	if err != nil {
		t.Fatalf("work order not recorded at issuance: %v", err)
	}
	if w.Decision != "allow" || w.EvidenceCount != 0 {
		t.Fatalf("expected open allowed work order, got %+v", w)
	}

	res := callTool(t, handleSubmitEvidence, map[string]any{
		"correlation_id": correlationID,
		"action":         "build",
		"result":         "ok",
		"summary":        "built the repository",
	})
	if res.IsError {
		t.Fatalf("evidence for an issued, allowed work order must be accepted: %s", toolText(t, res))
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(toolText(t, res)), &parsed); err != nil {
		t.Fatalf("parse evidence result: %v", err)
	}
	if parsed["status"] != "recorded" {
		t.Fatalf("expected recorded status, got %+v", parsed)
	}

	w, err = readWorkOrder(correlationID)
	if err != nil {
		t.Fatalf("read work order after evidence: %v", err)
	}
	if w.EvidenceCount != 1 || w.LastResult != "ok" || w.FirstEvidenceAt == "" {
		t.Fatalf("expected closed work order with one evidence event, got %+v", w)
	}
}

func TestSubmitEvidenceRejectsDeniedWorkOrder(t *testing.T) {
	linkageEnv(t)

	issued := issueWorkOrder(t, map[string]any{
		"action":      "deploy",
		"resource":    "service:gateway",
		"environment": "prod",
	})
	if issued["decision"] != "deny" {
		t.Fatalf("expected prod deploy to be denied without approval, got %+v", issued)
	}
	correlationID := issued["correlation_id"].(string)

	res := callTool(t, handleSubmitEvidence, map[string]any{
		"correlation_id": correlationID,
		"action":         "deploy",
		"result":         "ok",
	})
	if !res.IsError {
		t.Fatal("evidence must not attach to a denied work order")
	}
	if !strings.Contains(toolText(t, res), "denied") && !strings.Contains(toolText(t, res), "deny") {
		t.Fatalf("expected denied-work-order rejection, got: %s", toolText(t, res))
	}
}

func TestSubmitEvidenceRejectsActionMismatch(t *testing.T) {
	linkageEnv(t)

	issued := issueWorkOrder(t, map[string]any{
		"action":   "build",
		"resource": "repository",
	})
	correlationID := issued["correlation_id"].(string)

	res := callTool(t, handleSubmitEvidence, map[string]any{
		"correlation_id": correlationID,
		"action":         "deploy",
		"result":         "ok",
	})
	if !res.IsError {
		t.Fatal("evidence claiming an action the work order did not authorize must be rejected")
	}
	if !strings.Contains(toolText(t, res), "authorized action") {
		t.Fatalf("expected action-mismatch rejection, got: %s", toolText(t, res))
	}
}

func TestHandleDecideRecordsWorkOrder(t *testing.T) {
	linkageEnv(t)

	trace, code, body := decideProd(t, "", nil)
	if code != 200 {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}
	w, err := readWorkOrder(trace.CorrelationID)
	if err != nil {
		t.Fatalf("expected /decide to record the work order: %v", err)
	}
	if w.Decision != "deny" || w.Action != "deploy" || w.Environment != "prod" {
		t.Fatalf("unexpected recorded work order: %+v", w)
	}
}

func TestApprovalRetryUpdatesWorkOrderDecision(t *testing.T) {
	linkageEnv(t)

	issued := issueWorkOrder(t, map[string]any{
		"action":      "deploy",
		"resource":    "service:gateway",
		"environment": "prod",
	})
	correlationID := issued["correlation_id"].(string)
	if w, err := readWorkOrder(correlationID); err != nil || w.Decision != "deny" {
		t.Fatalf("expected recorded deny before grant, got %+v err=%v", w, err)
	}

	grantForTest(t, correlationID, "user:will", time.Minute)

	retried := issueWorkOrder(t, map[string]any{
		"action":         "deploy",
		"resource":       "service:gateway",
		"environment":    "prod",
		"correlation_id": correlationID,
	})
	if retried["decision"] != "allow" {
		t.Fatalf("expected allow after grant, got %+v", retried)
	}
	if w, err := readWorkOrder(correlationID); err != nil || w.Decision != "allow" {
		t.Fatalf("expected work order updated to allow, got %+v err=%v", w, err)
	}

	res := callTool(t, handleSubmitEvidence, map[string]any{
		"correlation_id": correlationID,
		"action":         "deploy",
		"result":         "ok",
	})
	if res.IsError {
		t.Fatalf("evidence for the granted work order must be accepted: %s", toolText(t, res))
	}
}

func TestRequestWorkOrderInferenceTraceCarriesDeclaredClassification(t *testing.T) {
	linkageEnv(t)

	var traces []DecisionTrace
	mcpTraceObserver = func(tr DecisionTrace) { traces = append(traces, tr) }
	t.Cleanup(func() { mcpTraceObserver = nil })

	issueWorkOrder(t, map[string]any{
		"action":     "inference",
		"resource":   "local",
		"attributes": `{"data_classification":"confidential"}`,
	})
	if len(traces) != 1 {
		t.Fatalf("expected one decision trace, got %d", len(traces))
	}
	if got := traces[0].DataClassification; got != "confidential" {
		t.Fatalf("trace data_classification = %q, want %q", got, "confidential")
	}
}

func TestSubmitEvidenceJoinsTheDecisionTrace(t *testing.T) {
	linkageEnv(t)

	var traces []DecisionTrace
	var evidence []EvidenceEvent
	mcpTraceObserver = func(tr DecisionTrace) { traces = append(traces, tr) }
	mcpEvidenceObserver = func(ev EvidenceEvent) { evidence = append(evidence, ev) }
	t.Cleanup(func() {
		mcpTraceObserver = nil
		mcpEvidenceObserver = nil
	})

	issued := issueWorkOrder(t, map[string]any{
		"action":     "inference",
		"resource":   "local",
		"attributes": `{"data_classification":"confidential"}`,
	})
	if issued["decision"] != "allow" {
		t.Fatalf("expected allow, got %+v", issued)
	}
	res := callTool(t, handleSubmitEvidence, map[string]any{
		"correlation_id": issued["correlation_id"],
		"action":         "inference",
		"result":         "ok",
	})
	if res.IsError {
		t.Fatalf("submit_evidence failed: %s", toolText(t, res))
	}
	if len(traces) != 1 || len(evidence) != 1 {
		t.Fatalf("expected one trace and one evidence event, got %d and %d", len(traces), len(evidence))
	}
	if evidence[0].TraceID != traces[0].TraceID {
		t.Fatalf("evidence trace_id = %q, want decision trace's %q", evidence[0].TraceID, traces[0].TraceID)
	}
	if got := evidence[0].Details["policy_decision"]; got != "allow" {
		t.Fatalf("evidence details.policy_decision = %q, want %q", got, "allow")
	}
}

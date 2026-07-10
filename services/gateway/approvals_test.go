package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func grantForTest(t *testing.T, correlationID, approver string, ttl time.Duration) {
	t.Helper()
	a, err := readApproval(correlationID)
	if err != nil {
		t.Fatalf("read approval %s: %v", correlationID, err)
	}
	if a.Status != "pending" {
		t.Fatalf("expected pending record, got %q", a.Status)
	}
	now := time.Now().UTC()
	a.Status = "approved"
	a.Approver = approver
	a.ApprovedAt = now.Format(time.RFC3339)
	a.ExpiresAt = now.Add(ttl).Format(time.RFC3339)
	if err := writeApproval(approvalPath(correlationID), a); err != nil {
		t.Fatalf("write approval: %v", err)
	}
}

func prodDeployRequest() Request {
	return Request{Action: "deploy", Resource: "service:gateway", Environment: "prod"}
}

func TestSteeringRootAnchorsConfigOutOfTree(t *testing.T) {
	root := t.TempDir()
	t.Setenv("STEERING_ROOT", root)
	t.Setenv("APPROVALS_DIR", "")
	t.Setenv("ROLES_CONFIG_PATH", "")

	if got, want := approvalsDir(), filepath.Join(root, "out", "approvals"); got != want {
		t.Fatalf("approvalsDir() = %q, want %q", got, want)
	}

	rolesPath := filepath.Join(root, "policies", "opa", "rego")
	if err := os.MkdirAll(rolesPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rolesPath, "roles.json"), []byte(`{"roles":{"tester":["ship"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !actionAllowedForRole("tester", "ship") {
		t.Fatal("expected roles.json to resolve via STEERING_ROOT")
	}
	if actionAllowedForRole("developer", "build") {
		t.Fatal("expected STEERING_ROOT roles to take precedence over relative candidates")
	}
}

func TestApprovalConsumeIsSingleUseAndMatched(t *testing.T) {
	t.Setenv("APPROVALS_DIR", t.TempDir())
	req := prodDeployRequest()
	correlationID := "corr-" + randomHex(8)

	if got := consumeApproval(correlationID, req); got != nil {
		t.Fatal("expected no approval before any record exists")
	}

	if err := recordPendingApproval(correlationID, req); err != nil {
		t.Fatalf("record pending: %v", err)
	}
	if got := consumeApproval(correlationID, req); got != nil {
		t.Fatal("pending record must not be consumable")
	}

	grantForTest(t, correlationID, "user:will", time.Minute)

	mismatched := req
	mismatched.Environment = "dev"
	if got := consumeApproval(correlationID, mismatched); got != nil {
		t.Fatal("approval must not apply to a different request")
	}

	first := consumeApproval(correlationID, req)
	if first == nil || first.Approver != "user:will" {
		t.Fatalf("expected consumable approval by user:will, got %+v", first)
	}
	if second := consumeApproval(correlationID, req); second != nil {
		t.Fatal("approval must be single-use")
	}
}

func TestApprovalConsumeRejectsExpired(t *testing.T) {
	t.Setenv("APPROVALS_DIR", t.TempDir())
	req := prodDeployRequest()
	correlationID := "corr-" + randomHex(8)

	if err := recordPendingApproval(correlationID, req); err != nil {
		t.Fatalf("record pending: %v", err)
	}
	grantForTest(t, correlationID, "user:will", -time.Minute)

	if got := consumeApproval(correlationID, req); got != nil {
		t.Fatal("expired approval must not be consumable")
	}
}

func TestApprovalStoreRejectsUnsafeCorrelationIDs(t *testing.T) {
	t.Setenv("APPROVALS_DIR", t.TempDir())
	req := prodDeployRequest()
	for _, id := range []string{"../escape", "corr-XYZ", "corr-" + randomHex(8) + "/../x", ""} {
		if err := recordPendingApproval(id, req); err == nil {
			t.Fatalf("expected rejection of correlation_id %q", id)
		}
		if got := consumeApproval(id, req); got != nil {
			t.Fatalf("expected no consume for correlation_id %q", id)
		}
	}
}

func decideProd(t *testing.T, correlationID string, attrs map[string]string) (DecisionTrace, int, string) {
	t.Helper()
	payload := map[string]any{
		"subject": map[string]any{"type": "user", "id": "user:alice", "attributes": map[string]string{"role": "developer"}},
		"request": map[string]any{"action": "deploy", "resource": "service:gateway", "environment": "prod", "attributes": attrs},
	}
	if correlationID != "" {
		payload["correlation_id"] = correlationID
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/decide", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return DecisionTrace{}, rec.Code, rec.Body.String()
	}
	var trace DecisionTrace
	if err := json.NewDecoder(rec.Body).Decode(&trace); err != nil {
		t.Fatalf("decode trace: %v", err)
	}
	return trace, rec.Code, ""
}

func TestHandleDecideIgnoresCallerSuppliedApproval(t *testing.T) {
	t.Setenv("APPROVALS_DIR", t.TempDir())
	t.Setenv("OPA_URL", "")
	t.Setenv("ALLOW_LOCAL_POLICY_FALLBACK", "true")

	trace, code, body := decideProd(t, "", map[string]string{"human_approved": "true", "approver": "user:mallory"})
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}
	if trace.Policy.Decision != "deny" {
		t.Fatalf("caller-supplied human_approved must not authorize prod deploy, got %q", trace.Policy.Decision)
	}
	if _, ok := trace.Request.Attributes["human_approved"]; ok {
		t.Fatal("human_approved must be stripped from the recorded trace")
	}
	var pending bool
	for _, item := range trace.Policy.Rationale {
		if item.Code == "APPROVAL_PENDING" {
			pending = true
		}
	}
	if !pending {
		t.Fatalf("expected APPROVAL_PENDING rationale, got %+v", trace.Policy.Rationale)
	}
	if a, err := readApproval(trace.CorrelationID); err != nil || a.Status != "pending" {
		t.Fatalf("expected pending approval record for %s, got %+v err=%v", trace.CorrelationID, a, err)
	}
}

func TestHandleDecideDenyThenGrantThenAllowOnce(t *testing.T) {
	t.Setenv("APPROVALS_DIR", t.TempDir())
	t.Setenv("OPA_URL", "")
	t.Setenv("ALLOW_LOCAL_POLICY_FALLBACK", "true")

	first, _, _ := decideProd(t, "", nil)
	if first.Policy.Decision != "deny" {
		t.Fatalf("expected initial deny, got %q", first.Policy.Decision)
	}

	grantForTest(t, first.CorrelationID, "user:will", time.Minute)

	second, _, _ := decideProd(t, first.CorrelationID, nil)
	if second.Policy.Decision != "allow" {
		t.Fatalf("expected allow after human grant, got %q (%+v)", second.Policy.Decision, second.Policy.Rationale)
	}
	if second.Request.Attributes["approver"] != "user:will" {
		t.Fatalf("expected approver recorded in trace, got %+v", second.Request.Attributes)
	}

	third, _, _ := decideProd(t, first.CorrelationID, nil)
	if third.Policy.Decision != "deny" {
		t.Fatalf("approval must be single-use, got %q on replay", third.Policy.Decision)
	}
}

func callTool(t *testing.T, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "request_work_order", Arguments: args}}
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("tool handler error: %v", err)
	}
	return res
}

func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("empty tool result content")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("unexpected content type %T", res.Content[0])
	}
	return tc.Text
}

func TestMCPRejectsTrustReservedAttributes(t *testing.T) {
	t.Setenv("APPROVALS_DIR", t.TempDir())
	t.Setenv("OPA_URL", "")
	t.Setenv("ALLOW_LOCAL_POLICY_FALLBACK", "true")

	for _, reserved := range []string{"role", "agent_id", "human_approved"} {
		attrs, _ := json.Marshal(map[string]string{reserved: "sneaky"})
		res := callTool(t, handleRequestWorkOrder, map[string]any{
			"action":     "deploy",
			"resource":   "service:gateway",
			"attributes": string(attrs),
		})
		if !res.IsError {
			t.Fatalf("expected error result for reserved attribute %q", reserved)
		}
		if !strings.Contains(toolText(t, res), "trust-reserved") {
			t.Fatalf("expected trust-reserved rejection for %q, got: %s", reserved, toolText(t, res))
		}
	}
}

func TestMCPApprovalFlowDenyGrantAllow(t *testing.T) {
	t.Setenv("APPROVALS_DIR", t.TempDir())
	t.Setenv("OPA_URL", "")
	t.Setenv("ALLOW_LOCAL_POLICY_FALLBACK", "true")

	res := callTool(t, handleRequestWorkOrder, map[string]any{
		"action":      "deploy",
		"resource":    "service:gateway",
		"environment": "prod",
	})
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", toolText(t, res))
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(toolText(t, res)), &first); err != nil {
		t.Fatalf("parse tool result: %v", err)
	}
	if first["decision"] != "deny" || first["approval_status"] != "pending" {
		t.Fatalf("expected deny with pending approval, got %+v", first)
	}
	correlationID := first["correlation_id"].(string)

	grantForTest(t, correlationID, "user:will", time.Minute)

	res = callTool(t, handleRequestWorkOrder, map[string]any{
		"action":         "deploy",
		"resource":       "service:gateway",
		"environment":    "prod",
		"correlation_id": correlationID,
	})
	var second map[string]any
	if err := json.Unmarshal([]byte(toolText(t, res)), &second); err != nil {
		t.Fatalf("parse tool result: %v", err)
	}
	if second["decision"] != "allow" || second["approver"] != "user:will" {
		t.Fatalf("expected allow approved by user:will after grant, got %+v", second)
	}
}

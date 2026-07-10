package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// The approval store is the server-side source of truth for human approvals.
// The governed agent can never assert approval state: the gateway records a
// pending approval when policy requires one, a human grants it out-of-band
// (tools/approve), and the gateway consumes it — single use — on the retry.
//
// Records live as one JSON file per correlation_id under APPROVALS_DIR so the
// HTTP gateway and the agent-spawned MCP process share state without a broker.

type Approval struct {
	CorrelationID string `json:"correlation_id"`
	Action        string `json:"action"`
	Resource      string `json:"resource"`
	Environment   string `json:"environment"`
	Status        string `json:"status"` // pending | approved | denied | consumed
	RequestedAt   string `json:"requested_at"`
	Approver      string `json:"approver,omitempty"`
	ApprovedAt    string `json:"approved_at,omitempty"`
	ExpiresAt     string `json:"expires_at,omitempty"`
	ConsumedAt    string `json:"consumed_at,omitempty"`
}

// correlationIDPattern matches gateway-minted IDs. Store operations refuse any
// other shape so client-supplied IDs cannot traverse the filesystem.
var correlationIDPattern = regexp.MustCompile(`^corr-[0-9a-f]{16}$`)

func approvalsDir() string {
	if d := os.Getenv("APPROVALS_DIR"); d != "" {
		return d
	}
	// Anchor to STEERING_ROOT when set so the agent-spawned MCP process
	// (whose cwd is the governed repo) and the human's approve CLI (run from
	// the steering repo) always share one store.
	if root := os.Getenv("STEERING_ROOT"); root != "" {
		return filepath.Join(root, "out", "approvals")
	}
	return "out/approvals"
}

func approvalPath(correlationID string) string {
	return filepath.Join(approvalsDir(), correlationID+".json")
}

func validCorrelationID(correlationID string) bool {
	return correlationIDPattern.MatchString(correlationID)
}

// requiresHumanApproval reports whether a request is gated on out-of-band
// human approval. Mirrors the prod-deploy gate in decision.rego.
func requiresHumanApproval(req Request) bool {
	return req.Action == "deploy" && req.Environment == "prod"
}

func readApproval(correlationID string) (Approval, error) {
	var a Approval
	data, err := os.ReadFile(approvalPath(correlationID))
	if err != nil {
		return a, err
	}
	if err := json.Unmarshal(data, &a); err != nil {
		return a, err
	}
	return a, nil
}

func writeApproval(path string, a Approval) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// recordPendingApproval writes a pending record for a request that policy
// gated on human approval. Existing records are never clobbered.
func recordPendingApproval(correlationID string, req Request) error {
	if !validCorrelationID(correlationID) {
		return fmt.Errorf("invalid correlation_id %q", correlationID)
	}
	if _, err := os.Stat(approvalPath(correlationID)); err == nil {
		return nil
	}
	return writeApproval(approvalPath(correlationID), Approval{
		CorrelationID: correlationID,
		Action:        req.Action,
		Resource:      req.Resource,
		Environment:   req.Environment,
		Status:        "pending",
		RequestedAt:   time.Now().UTC().Format(time.RFC3339),
	})
}

// consumeApproval returns the approval for correlationID if it is granted,
// unexpired, and matches the request — atomically marking it consumed so it
// can authorize exactly one decision. Returns nil when no usable approval
// exists (the caller then proceeds without approval attributes).
func consumeApproval(correlationID string, req Request) *Approval {
	if !validCorrelationID(correlationID) {
		return nil
	}
	a, err := readApproval(correlationID)
	if err != nil {
		return nil
	}
	if a.Status != "approved" || a.Approver == "" {
		return nil
	}
	if a.Action != req.Action || a.Resource != req.Resource || a.Environment != req.Environment {
		return nil
	}
	if a.ExpiresAt != "" {
		exp, err := time.Parse(time.RFC3339, a.ExpiresAt)
		if err != nil || time.Now().After(exp) {
			return nil
		}
	}
	// Claim by atomic rename: exactly one consumer wins.
	consumedPath := approvalPath(correlationID) + ".consumed"
	if err := os.Rename(approvalPath(correlationID), consumedPath); err != nil {
		return nil
	}
	a.Status = "consumed"
	a.ConsumedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeApproval(consumedPath, a); err != nil && !errors.Is(err, os.ErrNotExist) {
		// The claim already succeeded; a bookkeeping failure must not
		// invalidate the human's grant.
		fmt.Fprintf(os.Stderr, "approvals: record consumption of %s: %v\n", correlationID, err)
	}
	return &a
}

// applyApprovalGate enforces that approval state only ever enters policy input
// from the server-side store. It strips any caller-supplied approval
// attributes, then injects verified ones when a granted approval is consumed.
func applyApprovalGate(correlationID string, req *Request) *Approval {
	delete(req.Attributes, "human_approved")
	delete(req.Attributes, "approver")
	if correlationID == "" || !requiresHumanApproval(*req) {
		return nil
	}
	approval := consumeApproval(correlationID, *req)
	if approval == nil {
		return nil
	}
	if req.Attributes == nil {
		req.Attributes = map[string]string{}
	}
	req.Attributes["human_approved"] = "true"
	req.Attributes["approver"] = approval.Approver
	return approval
}

// approvalPendingRationale is appended by the gateway when it records a
// pending approval, telling the caller how the grant happens out-of-band.
func approvalPendingRationale(correlationID string) RationaleItem {
	return RationaleItem{
		Code:    "APPROVAL_PENDING",
		Message: fmt.Sprintf("Human approval required and now pending for %s. A human can grant it with: ./scripts/approve.sh grant %s", correlationID, correlationID),
	}
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// The work-order store is the server-side record of issued work orders, the
// forward half of evidence–grant linkage: submit_evidence only accepts a
// correlation_id that names a previously issued, allowed work order. Without
// it, evidence could be minted for work that was never authorized.
//
// Records live as one JSON file per correlation_id under WORKORDERS_DIR
// (defaulting next to the approvals store) so the HTTP gateway and the
// agent-spawned MCP process share state without a broker. The Kafka trace
// topic remains the durable audit record; this store is the gateway's
// enforcement index over it, and tools/grant-reconcile is the topic-level
// check that covers history from before this store existed.

type WorkOrder struct {
	CorrelationID string `json:"correlation_id"`
	Action        string `json:"action"`
	Resource      string `json:"resource"`
	Environment   string `json:"environment"`
	Decision      string `json:"decision"` // allow | deny
	IssuedAt      string `json:"issued_at"`
	EventID       string `json:"event_id"`
	// Closure bookkeeping, written when evidence is accepted. A work order
	// with Decision "allow" and EvidenceCount 0 is issued-and-never-closed —
	// exactly the state reconciliation must surface, never silently drop.
	EvidenceCount   int    `json:"evidence_count,omitempty"`
	FirstEvidenceAt string `json:"first_evidence_at,omitempty"`
	LastEvidenceAt  string `json:"last_evidence_at,omitempty"`
	LastResult      string `json:"last_result,omitempty"`
}

func workOrdersDir() string {
	if d := os.Getenv("WORKORDERS_DIR"); d != "" {
		return d
	}
	if root := os.Getenv("STEERING_ROOT"); root != "" {
		return filepath.Join(root, "out", "workorders")
	}
	return "out/workorders"
}

func workOrderPath(correlationID string) string {
	return filepath.Join(workOrdersDir(), correlationID+".json")
}

func readWorkOrder(correlationID string) (WorkOrder, error) {
	var w WorkOrder
	data, err := os.ReadFile(workOrderPath(correlationID))
	if err != nil {
		return w, err
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return w, err
	}
	return w, nil
}

func writeWorkOrder(w WorkOrder) error {
	path := workOrderPath(w.CorrelationID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// recordWorkOrder stores the decision behind a published trace. Called on
// every decision, allow and deny, so evidence rejection can report the true
// state (denied vs unknown). A retry on the same correlation_id (the
// deny → human grant → allow approval flow) updates the decision in place;
// closure bookkeeping is preserved.
func recordWorkOrder(trace DecisionTrace) error {
	if !validCorrelationID(trace.CorrelationID) {
		// Caller-supplied IDs outside the gateway-minted shape are not
		// storable (path safety); they simply never become linkable.
		return nil
	}
	w, err := readWorkOrder(trace.CorrelationID)
	if err != nil {
		w = WorkOrder{CorrelationID: trace.CorrelationID}
	}
	w.Action = trace.Request.Action
	w.Resource = trace.Request.Resource
	w.Environment = trace.Request.Environment
	w.Decision = trace.Policy.Decision
	w.IssuedAt = trace.EventTime
	w.EventID = trace.EventID
	return writeWorkOrder(w)
}

var (
	errWorkOrderUnknown = errors.New("no issued work order")
	errWorkOrderDenied  = errors.New("work order was denied")
)

// validateEvidenceLink checks that evidence may attach to correlationID:
// the work order must exist, be allowed, and cover the claimed action.
// Fail-closed: any state that cannot be positively verified is a rejection.
func validateEvidenceLink(correlationID, action string) (WorkOrder, error) {
	var w WorkOrder
	if !validCorrelationID(correlationID) {
		return w, fmt.Errorf("invalid correlation_id %q: evidence must use the correlation_id returned by request_work_order", correlationID)
	}
	w, err := readWorkOrder(correlationID)
	if err != nil {
		return w, fmt.Errorf("%w for correlation_id %s: call request_work_order before submitting evidence", errWorkOrderUnknown, correlationID)
	}
	if w.Decision != "allow" {
		return w, fmt.Errorf("%w: correlation_id %s carries decision %q — evidence can only attach to an allowed work order", errWorkOrderDenied, correlationID, w.Decision)
	}
	if action != w.Action {
		return w, fmt.Errorf("work order %s authorized action %q but evidence reports action %q", correlationID, w.Action, action)
	}
	return w, nil
}

// closeWorkOrder records accepted evidence against the work order. The
// evidence is already durable when this runs; a bookkeeping failure is
// logged by the caller, never surfaced as an evidence failure.
func closeWorkOrder(w WorkOrder, result string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	w.EvidenceCount++
	if w.FirstEvidenceAt == "" {
		w.FirstEvidenceAt = now
	}
	w.LastEvidenceAt = now
	w.LastResult = result
	return writeWorkOrder(w)
}

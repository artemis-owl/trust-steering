package main

import (
	"fmt"
	"testing"
)

func trace(corr, eventTime, decision, action, taskContext string) TraceRecord {
	var t TraceRecord
	t.CorrelationID = corr
	t.EventTime = eventTime
	t.Subject.ID = "kestrel-team-agent"
	t.Request.Action = action
	t.Request.Resource = "repository"
	t.Request.Environment = "dev"
	if taskContext != "" {
		t.Request.Attributes = map[string]string{"context": taskContext}
	}
	t.Policy.Decision = decision
	return t
}

func evidenceEvent(corr, eventTime, action, result string) EvidenceRecord {
	var e EvidenceRecord
	e.CorrelationID = corr
	e.EventTime = eventTime
	e.EvidenceType = "action.executed"
	e.Action = action
	e.Result = result
	e.Workload.ServiceName = "mcp-client"
	return e
}

// TestReconcileDogfoodShape reproduces the first real Trust-in-the-Loop
// verification (kestrel-team run-1785159637, diagnosed 2026-07-29): six work
// orders granted for a five-task plan — task 1 granted twice with identical
// task text (corr-1f83be18839abb94 at 13:40:40Z, corr-4ea18174e92634df at
// 13:46:32Z) — and five evidence events closing only the last five grants.
// The first grant of task 1 is an allowed action with no recorded outcome and
// must be surfaced, not silently absent.
func TestReconcileDogfoodShape(t *testing.T) {
	task1 := "Fix permanent-429 fast-fail in the retry classifier"
	traces := []TraceRecord{
		trace("corr-1f83be18839abb94", "2026-07-27T13:40:40Z", "allow", "execute", task1),
		trace("corr-4ea18174e92634df", "2026-07-27T13:46:32Z", "allow", "execute", task1),
	}
	evidence := []EvidenceRecord{
		evidenceEvent("corr-4ea18174e92634df", "2026-07-27T13:47:30Z", "execute", "ok"),
	}
	for i := 2; i <= 5; i++ {
		corr := fmt.Sprintf("corr-%016x", i)
		issued := fmt.Sprintf("2026-07-27T13:5%d:00Z", i)
		traces = append(traces, trace(corr, issued, "allow", "execute", fmt.Sprintf("Task %d", i)))
		evidence = append(evidence, evidenceEvent(corr, fmt.Sprintf("2026-07-27T13:5%d:59Z", i), "execute", "ok"))
	}

	report := reconcile(traces, evidence)

	if report.Grants != 6 || report.ClosedGrants != 5 {
		t.Fatalf("expected 6 grants with 5 closed, got %d/%d", report.Grants, report.ClosedGrants)
	}
	if len(report.UnclosedGrants) != 1 {
		t.Fatalf("expected exactly one unclosed grant, got %+v", report.UnclosedGrants)
	}
	unclosed := report.UnclosedGrants[0]
	if unclosed.CorrelationID != "corr-1f83be18839abb94" {
		t.Fatalf("expected corr-1f83be18839abb94 surfaced as unclosed, got %s", unclosed.CorrelationID)
	}
	if len(report.UnlinkedEvidence) != 0 {
		t.Fatalf("all five evidence events link to grants; got unlinked %+v", report.UnlinkedEvidence)
	}
	if !report.HasFindings() {
		t.Fatal("an unclosed grant is a finding")
	}

	if len(report.DuplicateGrants) != 1 {
		t.Fatalf("expected the double-granted task detected, got %+v", report.DuplicateGrants)
	}
	dup := report.DuplicateGrants[0]
	if len(dup.Correlations) != 2 || dup.Correlations[0] != "corr-1f83be18839abb94" || dup.UnclosedCount != 1 {
		t.Fatalf("expected duplicate group locating the silent first grant, got %+v", dup)
	}
}

func TestReconcileEvidenceWithoutGrant(t *testing.T) {
	report := reconcile(
		[]TraceRecord{trace("corr-00000000000000aa", "2026-07-27T10:00:00Z", "deny", "deploy", "")},
		[]EvidenceRecord{
			evidenceEvent("corr-00000000000000aa", "2026-07-27T10:01:00Z", "deploy", "ok"),
			evidenceEvent("corr-00000000000000bb", "2026-07-27T10:02:00Z", "build", "ok"),
		},
	)

	if len(report.UnlinkedEvidence) != 2 {
		t.Fatalf("expected both evidence events flagged, got %+v", report.UnlinkedEvidence)
	}
	byCorr := map[string]UnlinkedEvidence{}
	for _, e := range report.UnlinkedEvidence {
		byCorr[e.CorrelationID] = e
	}
	if e := byCorr["corr-00000000000000aa"]; e.Reason != "work order was denied but evidence claims execution" {
		t.Fatalf("denied-but-executed must be named, got %+v", e)
	}
	if e := byCorr["corr-00000000000000bb"]; e.Reason == "" {
		t.Fatalf("orphan evidence must carry a reason, got %+v", e)
	}
	if !report.HasFindings() {
		t.Fatal("unlinked evidence is a finding")
	}
}

func TestReconcileSkippedEvidenceForDenyIsLegitimate(t *testing.T) {
	e := evidenceEvent("corr-00000000000000cc", "2026-07-27T10:01:00Z", "deploy", "skipped")
	e.EvidenceType = "action.skipped"
	e.Workload.ServiceName = "worker"

	report := reconcile(
		[]TraceRecord{trace("corr-00000000000000cc", "2026-07-27T10:00:00Z", "deny", "deploy", "")},
		[]EvidenceRecord{e},
	)

	if report.HasFindings() {
		t.Fatalf("the worker's action.skipped for a denied trace records non-execution and is not a finding: %+v", report)
	}
	if report.Grants != 0 {
		t.Fatalf("a denied trace is not a grant, got %d", report.Grants)
	}
}

func TestReconcileApprovalRetryIsOneGrant(t *testing.T) {
	report := reconcile(
		[]TraceRecord{
			trace("corr-00000000000000dd", "2026-07-27T10:00:00Z", "deny", "deploy", "ship it"),
			trace("corr-00000000000000dd", "2026-07-27T10:05:00Z", "allow", "deploy", "ship it"),
		},
		[]EvidenceRecord{evidenceEvent("corr-00000000000000dd", "2026-07-27T10:06:00Z", "deploy", "ok")},
	)

	if report.Grants != 1 || report.ClosedGrants != 1 || report.HasFindings() {
		t.Fatalf("deny→grant→allow on one correlation_id is a single closed grant, got %+v", report)
	}
}

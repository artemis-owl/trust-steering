// grant-reconcile joins the decision-trace and evidence topics by
// correlation_id and reports both directions of evidence–grant linkage:
//
//   - unclosed grants: allowed work orders with no evidence event — an
//     authorized action whose outcome is absent from the audit trail
//     (the gap diagnosed 2026-07-29 from dogfood run-1785159637);
//   - unlinked evidence: evidence events that no allowed work order covers
//     (the direction the gateway now rejects at submit_evidence).
//
// The gateway's work-order store enforces linkage going forward; this tool is
// the topic-level check that also covers history from before the store
// existed. Exit codes: 0 clean, 2 findings present, 1 operational error.
//
// Env: KAFKA_BOOTSTRAP_SERVERS (required); KAFKA_TOPIC_DECISION_TRACE and
// KAFKA_TOPIC_EVIDENCE (defaults decision.trace.v1 / decision.evidence.v1);
// TIMEOUT_SEC (default 60). JSON report on stdout, summary on stderr.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

type TraceRecord struct {
	CorrelationID string `json:"correlation_id"`
	EventTime     string `json:"event_time"`
	Subject       struct {
		ID string `json:"id"`
	} `json:"subject"`
	Request struct {
		Action      string            `json:"action"`
		Resource    string            `json:"resource"`
		Environment string            `json:"environment"`
		Attributes  map[string]string `json:"attributes"`
	} `json:"request"`
	Policy struct {
		Decision string `json:"decision"`
	} `json:"policy"`
}

type EvidenceRecord struct {
	CorrelationID string `json:"correlation_id"`
	EventTime     string `json:"event_time"`
	EvidenceType  string `json:"evidence_type"`
	Action        string `json:"action"`
	Result        string `json:"result"`
	Workload      struct {
		ServiceName string `json:"service_name"`
	} `json:"workload"`
}

type Grant struct {
	CorrelationID string `json:"correlation_id"`
	IssuedAt      string `json:"issued_at"`
	SubjectID     string `json:"subject_id"`
	Action        string `json:"action"`
	Resource      string `json:"resource"`
	Environment   string `json:"environment"`
	Context       string `json:"context,omitempty"`
	EvidenceCount int    `json:"evidence_count"`
}

type UnlinkedEvidence struct {
	CorrelationID string `json:"correlation_id"`
	EventTime     string `json:"event_time"`
	EvidenceType  string `json:"evidence_type"`
	Action        string `json:"action"`
	Result        string `json:"result"`
	ServiceName   string `json:"service_name"`
	Reason        string `json:"reason"`
}

type DuplicateGrantGroup struct {
	Action        string   `json:"action"`
	Resource      string   `json:"resource"`
	Environment   string   `json:"environment"`
	Context       string   `json:"context"`
	Correlations  []string `json:"correlation_ids"`
	UnclosedCount int      `json:"unclosed_count"`
}

type Report struct {
	TracesRead       int                   `json:"traces_read"`
	EvidenceRead     int                   `json:"evidence_read"`
	Grants           int                   `json:"grants"`
	ClosedGrants     int                   `json:"closed_grants"`
	UnclosedGrants   []Grant               `json:"unclosed_grants"`
	UnlinkedEvidence []UnlinkedEvidence    `json:"unlinked_evidence"`
	DuplicateGrants  []DuplicateGrantGroup `json:"duplicate_grants,omitempty"`
}

func (r Report) HasFindings() bool {
	return len(r.UnclosedGrants) > 0 || len(r.UnlinkedEvidence) > 0
}

// reconcile joins traces and evidence by correlation_id. Pure so the dogfood
// regression fixture can exercise it without a broker.
func reconcile(traces []TraceRecord, evidence []EvidenceRecord) Report {
	report := Report{
		TracesRead:       len(traces),
		EvidenceRead:     len(evidence),
		UnclosedGrants:   []Grant{},
		UnlinkedEvidence: []UnlinkedEvidence{},
	}

	// A correlation_id is a grant when any of its traces decided allow — the
	// deny → human grant → allow approval flow reuses one correlation_id.
	grants := map[string]Grant{}
	denied := map[string]bool{}
	for _, t := range traces {
		if strings.EqualFold(t.Policy.Decision, "allow") {
			g, seen := grants[t.CorrelationID]
			if !seen || t.EventTime > g.IssuedAt {
				grants[t.CorrelationID] = Grant{
					CorrelationID: t.CorrelationID,
					IssuedAt:      t.EventTime,
					SubjectID:     t.Subject.ID,
					Action:        t.Request.Action,
					Resource:      t.Request.Resource,
					Environment:   t.Request.Environment,
					Context:       t.Request.Attributes["context"],
					EvidenceCount: g.EvidenceCount,
				}
			}
		} else {
			denied[t.CorrelationID] = true
		}
	}

	evidenceByCorr := map[string][]EvidenceRecord{}
	for _, e := range evidence {
		evidenceByCorr[e.CorrelationID] = append(evidenceByCorr[e.CorrelationID], e)
	}

	for corr, events := range evidenceByCorr {
		if g, ok := grants[corr]; ok {
			g.EvidenceCount = len(events)
			grants[corr] = g
			continue
		}
		for _, e := range events {
			reason := "no work order on the trace topic (never issued, or issued beyond topic retention)"
			if denied[corr] {
				if strings.EqualFold(e.Result, "skipped") {
					// The worker records action.skipped for denied traces:
					// evidence of non-execution, a legitimate link.
					continue
				}
				reason = "work order was denied but evidence claims execution"
			}
			report.UnlinkedEvidence = append(report.UnlinkedEvidence, UnlinkedEvidence{
				CorrelationID: e.CorrelationID,
				EventTime:     e.EventTime,
				EvidenceType:  e.EvidenceType,
				Action:        e.Action,
				Result:        e.Result,
				ServiceName:   e.Workload.ServiceName,
				Reason:        reason,
			})
		}
	}

	byTask := map[string][]Grant{}
	for _, g := range grants {
		report.Grants++
		if g.EvidenceCount > 0 {
			report.ClosedGrants++
		} else {
			report.UnclosedGrants = append(report.UnclosedGrants, g)
		}
		if g.Context != "" {
			key := g.Action + "\x00" + g.Resource + "\x00" + g.Environment + "\x00" + g.Context
			byTask[key] = append(byTask[key], g)
		}
	}

	// Identical task text granted more than once is the dogfood signature:
	// a re-request after a grant went unused. Informational on its own; with
	// an unclosed member it locates the silent grant.
	for _, group := range byTask {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].IssuedAt < group[j].IssuedAt })
		dup := DuplicateGrantGroup{
			Action:      group[0].Action,
			Resource:    group[0].Resource,
			Environment: group[0].Environment,
			Context:     group[0].Context,
		}
		for _, g := range group {
			dup.Correlations = append(dup.Correlations, g.CorrelationID)
			if g.EvidenceCount == 0 {
				dup.UnclosedCount++
			}
		}
		report.DuplicateGrants = append(report.DuplicateGrants, dup)
	}

	sort.Slice(report.UnclosedGrants, func(i, j int) bool {
		return report.UnclosedGrants[i].IssuedAt < report.UnclosedGrants[j].IssuedAt
	})
	sort.Slice(report.UnlinkedEvidence, func(i, j int) bool {
		return report.UnlinkedEvidence[i].EventTime < report.UnlinkedEvidence[j].EventTime
	})
	sort.Slice(report.DuplicateGrants, func(i, j int) bool {
		return report.DuplicateGrants[i].Context < report.DuplicateGrants[j].Context
	})
	return report
}

func main() {
	bootstrap := strings.TrimSpace(os.Getenv("KAFKA_BOOTSTRAP_SERVERS"))
	if bootstrap == "" {
		fmt.Fprintln(os.Stderr, "KAFKA_BOOTSTRAP_SERVERS is required")
		os.Exit(1)
	}
	traceTopic := envOr("KAFKA_TOPIC_DECISION_TRACE", "decision.trace.v1")
	evidenceTopic := envOr("KAFKA_TOPIC_EVIDENCE", "decision.evidence.v1")
	deadlineSec := 60
	if s := os.Getenv("TIMEOUT_SEC"); s != "" {
		_, _ = fmt.Sscanf(s, "%d", &deadlineSec)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(deadlineSec)*time.Second)
	defer cancel()

	traceRaw, evidenceRaw, err := readTopics(ctx, splitBrokers(bootstrap), traceTopic, evidenceTopic)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var traces []TraceRecord
	for _, raw := range traceRaw {
		var t TraceRecord
		if err := json.Unmarshal(raw, &t); err != nil || t.CorrelationID == "" {
			continue
		}
		traces = append(traces, t)
	}
	var evidence []EvidenceRecord
	for _, raw := range evidenceRaw {
		var e EvidenceRecord
		if err := json.Unmarshal(raw, &e); err != nil || e.CorrelationID == "" {
			continue
		}
		evidence = append(evidence, e)
	}

	report := reconcile(traces, evidence)

	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))

	fmt.Fprintf(os.Stderr, "grant-reconcile: %d traces, %d evidence events; %d grants (%d closed, %d unclosed); %d unlinked evidence events\n",
		report.TracesRead, report.EvidenceRead, report.Grants, report.ClosedGrants,
		len(report.UnclosedGrants), len(report.UnlinkedEvidence))
	for _, g := range report.UnclosedGrants {
		fmt.Fprintf(os.Stderr, "  UNCLOSED %s issued %s: %s %s (%s) — allowed, no evidence recorded\n",
			g.CorrelationID, g.IssuedAt, g.Action, g.Resource, g.Environment)
	}
	for _, e := range report.UnlinkedEvidence {
		fmt.Fprintf(os.Stderr, "  UNLINKED %s at %s: %s %s result=%s — %s\n",
			e.CorrelationID, e.EventTime, e.EvidenceType, e.Action, e.Result, e.Reason)
	}

	if report.HasFindings() {
		os.Exit(2)
	}
}

// readTopics consumes both topics from the beginning up to the end offsets
// observed at startup, so the run is deterministic over the topic contents.
func readTopics(ctx context.Context, brokers []string, topics ...string) (traceRaw, evidenceRaw [][]byte, err error) {
	adm, err := kadm.NewOptClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return nil, nil, fmt.Errorf("admin client: %w", err)
	}
	defer adm.Close()

	ends, err := adm.ListEndOffsets(ctx, topics...)
	if err != nil {
		return nil, nil, fmt.Errorf("list end offsets: %w", err)
	}
	remaining := map[string]map[int32]int64{}
	total := int64(0)
	starts, err := adm.ListStartOffsets(ctx, topics...)
	if err != nil {
		return nil, nil, fmt.Errorf("list start offsets: %w", err)
	}
	ends.Each(func(o kadm.ListedOffset) {
		start := int64(0)
		if s, ok := starts.Lookup(o.Topic, o.Partition); ok {
			start = s.Offset
		}
		if o.Offset > start {
			if remaining[o.Topic] == nil {
				remaining[o.Topic] = map[int32]int64{}
			}
			remaining[o.Topic][o.Partition] = o.Offset
			total += o.Offset - start
		}
	})
	if total == 0 {
		return nil, nil, nil
	}

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("consumer: %w", err)
	}
	defer cl.Close()

	read := int64(0)
	for read < total {
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("timed out after reading %d/%d records", read, total)
		}
		fetches := cl.PollFetches(ctx)
		if fetchErr := fetches.Err0(); fetchErr != nil && ctx.Err() == nil {
			return nil, nil, fmt.Errorf("fetch: %w", fetchErr)
		}
		fetches.EachRecord(func(r *kgo.Record) {
			endsForTopic := remaining[r.Topic]
			if endsForTopic == nil || r.Offset >= endsForTopic[r.Partition] {
				return
			}
			read++
			value := append([]byte(nil), r.Value...)
			if r.Topic == topics[0] {
				traceRaw = append(traceRaw, value)
			} else {
				evidenceRaw = append(evidenceRaw, value)
			}
		})
	}
	return traceRaw, evidenceRaw, nil
}

func splitBrokers(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envOr(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

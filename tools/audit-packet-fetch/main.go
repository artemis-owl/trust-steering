// audit-packet-fetch reads decision.trace and decision.evidence topics until it finds
// JSON records matching CORRELATION_ID (by body field), then writes them to output paths.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	decisionOut := flag.String("decision-out", "", "write decision trace JSON here")
	evidenceOut := flag.String("evidence-out", "", "write evidence JSON here")
	corrFlag := flag.String("correlation-id", "", "required correlation_id (or env CORRELATION_ID)")
	flag.Parse()

	want := strings.TrimSpace(*corrFlag)
	if want == "" {
		want = strings.TrimSpace(os.Getenv("CORRELATION_ID"))
	}
	if want == "" {
		fmt.Fprintln(os.Stderr, "correlation-id flag or CORRELATION_ID is required")
		os.Exit(2)
	}
	if *decisionOut == "" || *evidenceOut == "" {
		fmt.Fprintln(os.Stderr, "-decision-out and -evidence-out are required")
		os.Exit(2)
	}

	bootstrap := strings.TrimSpace(os.Getenv("KAFKA_BOOTSTRAP_SERVERS"))
	if bootstrap == "" {
		fmt.Fprintln(os.Stderr, "KAFKA_BOOTSTRAP_SERVERS is required")
		os.Exit(1)
	}
	traceTopic := envOr("KAFKA_TOPIC_DECISION_TRACE", "decision.trace.v1")
	evidenceTopic := envOr("KAFKA_TOPIC_EVIDENCE", "decision.evidence.v1")
	deadlineSec := 90
	if s := os.Getenv("TIMEOUT_SEC"); s != "" {
		_, _ = fmt.Sscanf(s, "%d", &deadlineSec)
	}

	brokers := splitBrokers(bootstrap)
	group := fmt.Sprintf("audit-packet-fetch-%d", time.Now().UnixNano())

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(traceTopic, evidenceTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(deadlineSec)*time.Second)
	defer cancel()

	var decisionBytes, evidenceBytes []byte

	// A denied work order executed by its consumer has no evidence: the
	// consumer was refused and the worker leaves it alone. Its evidence file
	// is written empty.
	for decisionBytes == nil || (evidenceBytes == nil && evidenceExpected(decisionBytes)) {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "timeout waiting for decision trace and evidence for correlation_id")
			os.Exit(1)
		}
		fetches := cl.PollFetches(ctx)
		if err := fetches.Err(); err != nil && ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, fetches.Err())
		}
		fetches.EachRecord(func(r *kgo.Record) {
			if !jsonCorrelationMatch(r.Value, want) {
				return
			}
			switch r.Topic {
			case traceTopic:
				if decisionBytes == nil {
					decisionBytes = append([]byte(nil), r.Value...)
				}
			case evidenceTopic:
				if evidenceBytes == nil {
					evidenceBytes = append([]byte(nil), r.Value...)
				}
			}
		})
	}

	if err := os.WriteFile(*decisionOut, decisionBytes, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*evidenceOut, evidenceBytes, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// evidenceExpected reports whether a decision trace will be followed by an
// evidence record. It is false only for a denied work order whose consumer
// executes it (request.attributes.executor == "consumer"); input that does not
// parse is treated as expecting evidence.
func evidenceExpected(decision []byte) bool {
	var tr struct {
		Request struct {
			Attributes map[string]string `json:"attributes"`
		} `json:"request"`
		Policy struct {
			Decision string `json:"decision"`
		} `json:"policy"`
	}
	if err := json.Unmarshal(decision, &tr); err != nil {
		return true
	}
	denied := strings.ToLower(strings.TrimSpace(tr.Policy.Decision)) == "deny"
	return !(denied && tr.Request.Attributes["executor"] == "consumer")
}

func jsonCorrelationMatch(raw []byte, want string) bool {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	c, _ := m["correlation_id"].(string)
	return c == want
}

func envOr(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
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

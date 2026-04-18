// kafka-read-one polls a Kafka topic until it sees a record (optionally matching correlation_id) or times out.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	bootstrap := strings.TrimSpace(os.Getenv("KAFKA_BOOTSTRAP_SERVERS"))
	if bootstrap == "" {
		fmt.Fprintln(os.Stderr, "KAFKA_BOOTSTRAP_SERVERS is required")
		os.Exit(1)
	}
	topic := envOr("KAFKA_TOPIC", "decision.evidence.v1")
	matchCorr := strings.TrimSpace(os.Getenv("CORRELATION_ID"))
	deadlineSec := 45
	if s := os.Getenv("TIMEOUT_SEC"); s != "" {
		_, _ = fmt.Sscanf(s, "%d", &deadlineSec)
	}

	brokers := splitBrokers(bootstrap)
	group := fmt.Sprintf("kafka-read-one-%d", time.Now().UnixNano())

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(deadlineSec)*time.Second)
	defer cancel()

	for {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "timeout waiting for record")
			os.Exit(1)
		}
		fetches := cl.PollFetches(ctx)
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "timeout waiting for record")
			os.Exit(1)
		}
		fetches.EachRecord(func(r *kgo.Record) {
			if matchCorr == "" {
				fmt.Println(string(r.Value))
				os.Exit(0)
			}
			var payload map[string]any
			if err := json.Unmarshal(r.Value, &payload); err != nil {
				return
			}
			c, _ := payload["correlation_id"].(string)
			if c == matchCorr {
				fmt.Println(string(r.Value))
				os.Exit(0)
			}
		})
	}
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

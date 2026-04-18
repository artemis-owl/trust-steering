package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
)

type kafkaHeaderCarrier []kgo.RecordHeader

func (c *kafkaHeaderCarrier) Get(key string) string {
	for _, h := range *c {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaHeaderCarrier) Set(key, value string) {
	*c = append(*c, kgo.RecordHeader{Key: key, Value: []byte(value)})
}

func (c *kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(*c))
	for _, h := range *c {
		keys = append(keys, h.Key)
	}
	return keys
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

func newKafkaProducer() (*kgo.Client, error) {
	bootstrap := strings.TrimSpace(os.Getenv("KAFKA_BOOTSTRAP_SERVERS"))
	if bootstrap == "" {
		return nil, nil
	}
	brokers := splitBrokers(bootstrap)
	if len(brokers) == 0 {
		return nil, nil
	}
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, err
	}
	return cl, nil
}

func kafkaDecisionTraceTopic() string {
	return envOr("KAFKA_TOPIC_DECISION_TRACE", "decision.trace.v1")
}

// publishDecisionTrace writes the trace to Kafka. When the broker is configured, failure to
// publish must fail the request so we do not imply execution without a durable trace.
func publishDecisionTrace(ctx context.Context, cl *kgo.Client, trace DecisionTrace, correlationID string) error {
	payload, err := json.Marshal(trace)
	if err != nil {
		return err
	}
	var headers kafkaHeaderCarrier
	otel.GetTextMapPropagator().Inject(ctx, &headers)
	res := cl.ProduceSync(ctx, &kgo.Record{
		Topic:   kafkaDecisionTraceTopic(),
		Key:     []byte(correlationID),
		Value:   payload,
		Headers: headers,
	})
	return res.FirstErr()
}

func kafkaEvidenceTopic() string {
	return envOr("KAFKA_TOPIC_EVIDENCE", "decision.evidence.v1")
}

// publishEvidence writes an evidence event to Kafka, keyed by correlation_id.
func publishEvidence(ctx context.Context, cl *kgo.Client, payload []byte, correlationID string) error {
	var headers kafkaHeaderCarrier
	otel.GetTextMapPropagator().Inject(ctx, &headers)
	res := cl.ProduceSync(ctx, &kgo.Record{
		Topic:   kafkaEvidenceTopic(),
		Key:     []byte(correlationID),
		Value:   payload,
		Headers: headers,
	})
	return res.FirstErr()
}

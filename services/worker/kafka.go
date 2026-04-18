package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

func kafkaTraceTopic() string {
	return envOr("KAFKA_TOPIC_DECISION_TRACE", "decision.trace.v1")
}

func kafkaEvidenceTopic() string {
	return envOr("KAFKA_TOPIC_EVIDENCE", "decision.evidence.v1")
}

func kafkaConsumerGroup() string {
	return envOr("KAFKA_CONSUMER_GROUP", "steering-worker")
}

func newKafkaWorkerClient() (*kgo.Client, error) {
	bootstrap := strings.TrimSpace(os.Getenv("KAFKA_BOOTSTRAP_SERVERS"))
	if bootstrap == "" {
		return nil, nil
	}
	brokers := splitBrokers(bootstrap)
	if len(brokers) == 0 {
		return nil, errors.New("KAFKA_BOOTSTRAP_SERVERS has no broker addresses")
	}
	return kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(kafkaConsumerGroup()),
		kgo.ConsumeTopics(kafkaTraceTopic()),
		kgo.AllowAutoTopicCreation(),
	)
}

func runKafkaConsumer(ctx context.Context, cl *kgo.Client) {
	for {
		if ctx.Err() != nil {
			return
		}
		fetches := cl.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}
		fetches.EachError(func(_ string, _ int32, err error) {
			log.Printf("kafka fetch error: %v", err)
		})
		fetches.EachRecord(func(r *kgo.Record) {
			carrier := kafkaHeaderCarrier(append([]kgo.RecordHeader(nil), r.Headers...))
			procCtx := otel.GetTextMapPropagator().Extract(ctx, &carrier)
			procCtx, span := otel.Tracer("steering-worker").Start(procCtx, "kafka.process_decision_trace")
			defer span.End()

			var trace DecisionTrace
			if err := json.Unmarshal(r.Value, &trace); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "unmarshal trace")
				log.Printf("kafka: skip invalid trace record: %v", err)
				return
			}
			span.SetAttributes(
				attribute.String("steering.correlation_id", trace.CorrelationID),
				attribute.String("policy.decision", strings.ToLower(trace.Policy.Decision)),
			)

			ev, err := traceToEvidence(trace)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "trace to evidence")
				log.Printf("kafka: skip trace %s: %v", trace.CorrelationID, err)
				return
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "marshal evidence")
				log.Printf("kafka: marshal evidence: %v", err)
				return
			}
			pctx, cancel := context.WithTimeout(procCtx, 10*time.Second)
			var evidenceHeaders kafkaHeaderCarrier
			otel.GetTextMapPropagator().Inject(pctx, &evidenceHeaders)
			res := cl.ProduceSync(pctx, &kgo.Record{
				Topic:   kafkaEvidenceTopic(),
				Key:     []byte(trace.CorrelationID),
				Value:   payload,
				Headers: evidenceHeaders,
			})
			cancel()
			if err := res.FirstErr(); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "produce evidence")
				log.Printf("kafka: produce evidence: %v", err)
			}
		})
	}
}

package testutil

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

type Kafka struct {
	brokers []string
	client  *kafka.Client
	writer  *kafka.Writer
}

func NewKafka(t testing.TB) *Kafka {
	t.Helper()
	addresses := os.Getenv("TEST_KAFKA_BROKERS")
	if addresses == "" {
		t.Fatal("TEST_KAFKA_BROKERS must point to the integration Kafka brokers")
	}
	brokers := strings.Split(addresses, ",")
	address := kafka.TCP(brokers...)
	transport := &kafka.Transport{}
	writer := &kafka.Writer{
		Addr: address, Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll,
		BatchTimeout: 10 * time.Millisecond, Transport: transport,
	}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Errorf("close Kafka writer: %v", err)
		}
		transport.CloseIdleConnections()
	})
	return &Kafka{
		brokers: brokers, writer: writer,
		client: &kafka.Client{Addr: address, Timeout: 5 * time.Second, Transport: transport},
	}
}

func (k *Kafka) PublishJSON(t testing.TB, ctx context.Context, topic, key string, event any) {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("encode Kafka event for %s: %v", topic, err)
	}
	if err := k.writer.WriteMessages(ctx, kafka.Message{Topic: topic, Key: []byte(key), Value: body}); err != nil {
		t.Fatalf("publish Kafka event to %s: %v", topic, err)
	}
}

type topicPartition struct {
	id     int
	next   int64
	reader *kafka.Reader
}

type TopicReader[Event any] struct {
	kafka      *Kafka
	topic      string
	partitions []topicPartition
}

// ObserveTopic starts at the current end of every partition without joining a consumer group.
func ObserveTopic[Event any](t testing.TB, ctx context.Context, k *Kafka, topic string) *TopicReader[Event] {
	t.Helper()
	metadata, err := k.client.Metadata(ctx, &kafka.MetadataRequest{Topics: []string{topic}})
	if err != nil {
		t.Fatalf("read Kafka metadata for %s: %v", topic, err)
	}
	if len(metadata.Topics) != 1 {
		t.Fatalf("Kafka metadata for %s: got %d topics, want one", topic, len(metadata.Topics))
	}
	info := metadata.Topics[0]
	if info.Error != nil || len(info.Partitions) == 0 {
		t.Fatalf("Kafka topic %s is unavailable: %d partitions, error %v", topic, len(info.Partitions), info.Error)
	}
	reader := &TopicReader[Event]{kafka: k, topic: topic}
	for _, partition := range info.Partitions {
		reader.partitions = append(reader.partitions, topicPartition{id: partition.ID})
	}
	ends := reader.endOffsets(t, ctx)
	for i := range reader.partitions {
		partition := &reader.partitions[i]
		partition.next = ends[partition.id]
		partition.reader = kafka.NewReader(kafka.ReaderConfig{
			Brokers: k.brokers, Topic: topic, Partition: partition.id,
			MaxWait: 250 * time.Millisecond,
		})
		t.Cleanup(func() {
			if err := partition.reader.Close(); err != nil {
				t.Errorf("close Kafka reader for %s partition %d: %v", topic, partition.id, err)
			}
		})
		if err := partition.reader.SetOffset(partition.next); err != nil {
			t.Fatalf("set Kafka offset for %s partition %d: %v", topic, partition.id, err)
		}
	}
	return reader
}

// ReadAvailable reads records up to a fresh snapshot of the partition end offsets.
func (r *TopicReader[Event]) ReadAvailable(t testing.TB, ctx context.Context) []Event {
	t.Helper()
	ends := r.endOffsets(t, ctx)
	var events []Event
	for i := range r.partitions {
		partition := &r.partitions[i]
		for partition.next < ends[partition.id] {
			msg, err := partition.reader.FetchMessage(ctx)
			if err != nil {
				t.Fatalf("read Kafka topic %s partition %d offset %d: %v", r.topic, partition.id, partition.next, err)
			}
			if msg.Offset != partition.next {
				t.Fatalf("Kafka topic %s partition %d: got offset %d, want %d", r.topic, partition.id, msg.Offset, partition.next)
			}
			var event Event
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				t.Fatalf("decode Kafka topic %s partition %d offset %d: %v", r.topic, partition.id, msg.Offset, err)
			}
			events = append(events, event)
			partition.next++
		}
	}
	return events
}

func (r *TopicReader[Event]) endOffsets(t testing.TB, ctx context.Context) map[int]int64 {
	t.Helper()
	requests := make([]kafka.OffsetRequest, 0, len(r.partitions))
	for _, partition := range r.partitions {
		requests = append(requests, kafka.LastOffsetOf(partition.id))
	}
	response, err := r.kafka.client.ListOffsets(ctx, &kafka.ListOffsetsRequest{
		Topics: map[string][]kafka.OffsetRequest{r.topic: requests},
	})
	if err != nil {
		t.Fatalf("read Kafka end offsets for %s: %v", r.topic, err)
	}
	ends := make(map[int]int64, len(r.partitions))
	for _, partition := range response.Topics[r.topic] {
		if partition.Error != nil || partition.LastOffset < 0 {
			t.Fatalf("read Kafka end offset for %s partition %d: offset %d, error %v",
				r.topic, partition.Partition, partition.LastOffset, partition.Error)
		}
		ends[partition.Partition] = partition.LastOffset
	}
	if len(ends) != len(r.partitions) {
		t.Fatalf("Kafka end offsets for %s: got %d partitions, want %d", r.topic, len(ends), len(r.partitions))
	}
	return ends
}

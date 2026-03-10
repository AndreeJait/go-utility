// Package kafkaw implements the brokerw interfaces for Apache Kafka
// using the modern segmentio/kafka-go library.
package kafkaw

import (
	"context"
	"errors"

	"github.com/AndreeJait/go-utility/v2/brokerw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/segmentio/kafka-go"
)

// kafkaProducer implements brokerw.Producer for Kafka.
type kafkaProducer struct {
	writer *kafka.Writer
}

// NewProducer initializes a highly concurrent Kafka producer.
// It uses a Hash balancer to ensure messages with the same Key are routed to the same partition.
func NewProducer(brokers []string) brokerw.Producer {
	w := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Balancer: &kafka.Hash{},
		Async:    false, // Set too false to guarantee delivery before returning
	}
	return &kafkaProducer{writer: w}
}

// Send publishes a single message to a Kafka topic.
func (p *kafkaProducer) Send(ctx context.Context, topic string, key, payload []byte) error {
	msg := kafka.Message{
		Topic: topic,
		Key:   key,
		Value: payload,
	}
	return p.writer.WriteMessages(ctx, msg)
}

// BulkSend publishes multiple messages in a single network request.
func (p *kafkaProducer) BulkSend(ctx context.Context, topic string, keys, payloads [][]byte) error {
	if len(keys) != len(payloads) {
		return errors.New("kafkaw: keys and payloads slices must have the same length")
	}

	msgs := make([]kafka.Message, len(payloads))
	for i := range payloads {
		msgs[i] = kafka.Message{
			Topic: topic,
			Key:   keys[i],
			Value: payloads[i],
		}
	}
	return p.writer.WriteMessages(ctx, msgs...)
}

// Close gracefully shuts down the Kafka writer, flushing any pending messages.
func (p *kafkaProducer) Close() error {
	return p.writer.Close()
}

// kafkaConsumer implements brokerw.Consumer for Kafka.
type kafkaConsumer struct {
	brokers []string
	groupID string
	readers []*kafka.Reader
}

// NewConsumer initializes a Kafka consumer that utilizes Consumer Groups
// for automatic load balancing and partition assignment.
func NewConsumer(brokers []string, groupID string) brokerw.Consumer {
	return &kafkaConsumer{
		brokers: brokers,
		groupID: groupID,
	}
}

// Consume subscribes to a topic and starts a background goroutine to process messages.
// It uses manual offset commits (Explicit Ack) to guarantee at-least-once delivery.
func (c *kafkaConsumer) Consume(ctx context.Context, topic string, handlers ...brokerw.Handler) error {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: c.brokers,
		GroupID: c.groupID,
		Topic:   topic,
	})
	c.readers = append(c.readers, r)

	logw.Infof("Kafka Consumer started for topic: %s | Group: %s", topic, c.groupID)

	go func() {
		for {
			m, err := r.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					logw.Infof("Kafka Consumer shutting down for topic: %s", topic)
					return
				}
				logw.Errorf("kafkaw: failed to fetch message: %v", err)
				continue
			}

			stdMsg := &brokerw.Message{
				Topic:   m.Topic,
				Key:     m.Key,
				Payload: m.Value,
			}

			// Execute the middleware chain
			if err := brokerw.ExecuteHandlers(ctx, stdMsg, handlers...); err != nil {
				logw.Errorf("kafkaw: handler failed for topic %s: %v", topic, err)
				// Note: Skipping CommitMessages causes Kafka to retry this message later
				continue
			}

			// Manual Ack: Only commit the offset if all handlers succeeded
			if err := r.CommitMessages(ctx, m); err != nil {
				logw.Errorf("kafkaw: failed to commit message offset: %v", err)
			}
		}
	}()

	return nil
}

// Close gracefully terminates all active Kafka readers.
func (c *kafkaConsumer) Close() error {
	var errs []error
	for _, r := range c.readers {
		if err := r.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.New("kafkaw: failed to close one or more readers")
	}
	return nil
}

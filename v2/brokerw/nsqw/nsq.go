// Package nsqw implements the brokerw interfaces for NSQ, a real-time distributed messaging platform.
package nsqw

import (
	"context"

	"github.com/AndreeJait/go-utility/v2/brokerw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/nsqio/go-nsq"
)

// nsqProducer implements brokerw.Producer for NSQ.
type nsqProducer struct {
	producer *nsq.Producer
}

// NewProducer initializes an NSQ producer.
// The address should point to a specific nsqd instance (e.g., "127.0.0.1:4150").
func NewProducer(nsqdAddr string) (brokerw.Producer, error) {
	config := nsq.NewConfig()
	p, err := nsq.NewProducer(nsqdAddr, config)
	if err != nil {
		return nil, err
	}
	return &nsqProducer{producer: p}, nil
}

// Send publishes a single message to an NSQ topic.
// Note: NSQ does not utilize routing keys. The key parameter is ignored.
func (p *nsqProducer) Send(ctx context.Context, topic string, key, payload []byte) error {
	// NSQ's standard Go client doesn't take context directly in Publish,
	// but it executes very quickly over TCP.
	return p.producer.Publish(topic, payload)
}

// BulkSend leverages NSQ's native MultiPublish for high-throughput batching.
func (p *nsqProducer) BulkSend(ctx context.Context, topic string, keys, payloads [][]byte) error {
	return p.producer.MultiPublish(topic, payloads)
}

// Close gracefully stops the NSQ producer and closes the TCP connection.
func (p *nsqProducer) Close() error {
	p.producer.Stop()
	return nil
}

// nsqConsumer implements brokerw.Consumer for NSQ.
type nsqConsumer struct {
	lookupdAddrs []string
	channel      string
	consumers    []*nsq.Consumer
}

// NewConsumer initializes an NSQ consumer using nsqlookupd for dynamic discovery.
// The 'channel' acts as a consumer group; every consumer in the same channel
// shares the message load for a topic.
func NewConsumer(lookupdAddrs []string, channel string) brokerw.Consumer {
	return &nsqConsumer{
		lookupdAddrs: lookupdAddrs,
		channel:      channel,
	}
}

// Consume subscribes to a topic and registers the middleware handlers.
// NSQ natively handles Auto-Ack (if handler returns nil) and Auto-Requeue (if error).
func (c *nsqConsumer) Consume(ctx context.Context, topic string, handlers ...brokerw.Handler) error {
	config := nsq.NewConfig()

	// Create a new consumer for the specific topic and channel
	q, err := nsq.NewConsumer(topic, c.channel, config)
	if err != nil {
		return err
	}

	logw.Infof("NSQ Consumer started for topic: %s | Channel: %s", topic, c.channel)

	// Register the handler. go-nsq automatically manages concurrency.
	q.AddHandler(nsq.HandlerFunc(func(m *nsq.Message) error {
		stdMsg := &brokerw.Message{
			Topic:   topic,
			Payload: m.Body,
			// NSQ doesn't use keys, so we leave it empty.
		}

		// Execute the middleware chain
		if err := brokerw.ExecuteHandlers(ctx, stdMsg, handlers...); err != nil {
			logw.Errorf("nsqw: handler failed for topic %s: %v", topic, err)
			return err // Returning an error tells NSQ to Nack and Requeue the message
		}

		return nil // Returning nil tells NSQ to Ack the message
	}))

	// Connect to the lookup daemons to discover nsqd nodes dynamically
	if err := q.ConnectToNSQLookupds(c.lookupdAddrs); err != nil {
		return err
	}

	c.consumers = append(c.consumers, q)

	// Listen for context cancellation to gracefully stop this specific consumer
	go func() {
		<-ctx.Done()
		logw.Infof("Context canceled, stopping NSQ consumer for topic: %s", topic)
		q.Stop()
	}()

	return nil
}

// Close gracefully terminates all tracked NSQ consumers.
func (c *nsqConsumer) Close() error {
	for _, q := range c.consumers {
		q.Stop()
		// Wait for the consumer to fully drain and disconnect
		<-q.StopChan
	}
	return nil
}

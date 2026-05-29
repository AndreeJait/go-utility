// Package nsqw implements the brokerw interfaces for NSQ, a real-time distributed messaging platform.
package nsqw

import (
	"context"
	"fmt"
	"time"

	"github.com/AndreeJait/go-utility/v2/brokerw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/nsqio/go-nsq"
)

// ProducerConfig holds configuration for the NSQ producer with retry support.
type ProducerConfig struct {
	// NSQdAddr is the address of the nsqd instance (e.g., "127.0.0.1:4150").
	NSQdAddr string
	// MaxRetries is the maximum number of connection attempts. 0 means try once (no retry). Default: 0.
	MaxRetries int
	// RetryDelay is the initial delay between retries. Each subsequent retry doubles the delay. Default: 1s.
	RetryDelay time.Duration
}

// nsqProducer implements brokerw.Producer for NSQ.
type nsqProducer struct {
	producer *nsq.Producer
}

// NewProducer initializes an NSQ producer.
// The address should point to a specific nsqd instance (e.g., "127.0.0.1:4150").
func NewProducer(nsqdAddr string) (brokerw.Producer, error) {
	return NewProducerWithRetry(ProducerConfig{
		NSQdAddr:   nsqdAddr,
		MaxRetries: 0,
	})
}

// NewProducerWithRetry initializes an NSQ producer with retry support.
// If MaxRetries > 0, connection failures will be retried with exponential backoff.
func NewProducerWithRetry(cfg ProducerConfig) (brokerw.Producer, error) {
	if cfg.NSQdAddr == "" {
		return nil, fmt.Errorf("nsqw: nsqd address is required")
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 1 * time.Second
	}

	config := nsq.NewConfig()
	p, err := nsq.NewProducer(cfg.NSQdAddr, config)
	if err != nil {
		return nil, fmt.Errorf("nsqw: failed to create producer: %w", err)
	}

	// Attempt initial connection with retries
	attempts := cfg.MaxRetries + 1 // +1 for the initial attempt
	delay := cfg.RetryDelay

	for attempt := 1; attempt <= attempts; attempt++ {
		err := p.Ping()
		if err == nil {
			logw.Infof("nsqw: producer connected to %s", cfg.NSQdAddr)
			return &nsqProducer{producer: p}, nil
		}

		if attempt < attempts {
			logw.Warningf("nsqw: producer connection attempt %d/%d to %s failed: %v, retrying in %v", attempt, attempts, cfg.NSQdAddr, err, delay)
			time.Sleep(delay)
			delay *= 2 // exponential backoff
		} else {
			// Last attempt failed, return the producer anyway — NSQ will
			// attempt reconnection on publish. This matches go-nsq behavior.
			logw.Warningf("nsqw: producer could not ping %s after %d attempts: %v (will retry on publish)", cfg.NSQdAddr, attempts, err)
			return &nsqProducer{producer: p}, nil
		}
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
	nsqdAddr     string
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

// NewConsumerDirect initializes an NSQ consumer that connects directly to nsqd.
// This is useful when nsqlookupd broadcast address isn't resolvable from the client.
// The 'channel' acts as a consumer group; every consumer in the same channel
// shares the message load for a topic.
func NewConsumerDirect(nsqdAddr, channel string) brokerw.Consumer {
	return &nsqConsumer{
		nsqdAddr: nsqdAddr,
		channel:  channel,
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

	// Connect either directly to nsqd or via lookupd
	if c.nsqdAddr != "" {
		// Direct connection to nsqd
		if err := q.ConnectToNSQD(c.nsqdAddr); err != nil {
			return err
		}
		logw.Infof("nsqw: connected directly to nsqd %s", c.nsqdAddr)
	} else if len(c.lookupdAddrs) > 0 {
		// Connect to lookupd for dynamic discovery
		if err := q.ConnectToNSQLookupds(c.lookupdAddrs); err != nil {
			return err
		}
		logw.Infof("nsqw: querying nsqlookupd %v", c.lookupdAddrs)
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
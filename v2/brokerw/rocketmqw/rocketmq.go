// Package rocketmqw implements the brokerw interfaces for Apache RocketMQ.
package rocketmqw

import (
	"context"
	"errors"
	"fmt"

	"github.com/AndreeJait/go-utility/v2/brokerw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

// rocketProducer implements brokerw.Producer for RocketMQ.
type rocketProducer struct {
	producer rocketmq.Producer
}

// NewProducer initializes a synchronous RocketMQ producer.
func NewProducer(nameServers []string, groupName string) (brokerw.Producer, error) {
	p, err := rocketmq.NewProducer(
		producer.WithNameServer(nameServers),
		producer.WithGroupName(groupName),
		producer.WithRetry(2),
	)
	if err != nil {
		return nil, fmt.Errorf("rocketmqw: failed to create producer: %w", err)
	}

	if err := p.Start(); err != nil {
		return nil, fmt.Errorf("rocketmqw: failed to start producer: %w", err)
	}

	return &rocketProducer{producer: p}, nil
}

// Send publishes a single message to RocketMQ.
// If a key is provided, it is attached to the message for trace/indexing purposes.
func (p *rocketProducer) Send(ctx context.Context, topic string, key, payload []byte) error {
	msg := primitive.NewMessage(topic, payload)
	if len(key) > 0 {
		msg.WithKeys([]string{string(key)})
	}

	res, err := p.producer.SendSync(ctx, msg)
	if err != nil {
		return err
	}
	if res.Status != primitive.SendOK {
		return fmt.Errorf("rocketmqw: send failed with status: %v", res.Status)
	}
	return nil
}

// BulkSend dispatches a batch of messages in a single network transmission.
func (p *rocketProducer) BulkSend(ctx context.Context, topic string, keys, payloads [][]byte) error {
	if len(keys) != len(payloads) {
		return errors.New("rocketmqw: keys and payloads slices must have identical lengths")
	}

	var msgs []*primitive.Message
	for i := range payloads {
		msg := primitive.NewMessage(topic, payloads[i])
		if len(keys[i]) > 0 {
			msg.WithKeys([]string{string(keys[i])})
		}
		msgs = append(msgs, msg)
	}

	res, err := p.producer.SendSync(ctx, msgs...)
	if err != nil {
		return err
	}
	if res.Status != primitive.SendOK {
		return fmt.Errorf("rocketmqw: bulk send failed with status: %v", res.Status)
	}
	return nil
}

// Close shuts down the RocketMQ producer cleanly.
func (p *rocketProducer) Close() error {
	return p.producer.Shutdown()
}

// rocketConsumer implements brokerw.Consumer for RocketMQ.
type rocketConsumer struct {
	consumer rocketmq.PushConsumer
}

// NewConsumer initializes a Push-style RocketMQ consumer for a specific group.
func NewConsumer(nameServers []string, groupName string) (brokerw.Consumer, error) {
	c, err := rocketmq.NewPushConsumer(
		consumer.WithNameServer(nameServers),
		consumer.WithGroupName(groupName),
		consumer.WithConsumeFromWhere(consumer.ConsumeFromLastOffset),
	)
	if err != nil {
		return nil, fmt.Errorf("rocketmqw: failed to create consumer: %w", err)
	}

	return &rocketConsumer{consumer: c}, nil
}

// Consume subscribes to a topic and utilizes the push callback model to process events.
func (c *rocketConsumer) Consume(ctx context.Context, topic string, handlers ...brokerw.Handler) error {
	err := c.consumer.Subscribe(topic, consumer.MessageSelector{}, func(cCtx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, m := range msgs {
			stdMsg := &brokerw.Message{
				Topic:   m.Topic,
				Key:     []byte(m.GetKeys()), // Retrieve attached keys
				Payload: m.Body,
			}

			// Execute Middleware Chain
			if err := brokerw.ExecuteHandlers(cCtx, stdMsg, handlers...); err != nil {
				logw.Errorf("rocketmqw: handler failed for topic %s: %v", topic, err)
				// Nack: Tells RocketMQ to retry this message later according to its delay levels
				return consumer.ConsumeRetryLater, err
			}
		}
		// Ack: All messages processed successfully
		return consumer.ConsumeSuccess, nil
	})

	if err != nil {
		return fmt.Errorf("rocketmqw: failed to subscribe to topic: %w", err)
	}

	logw.Infof("RocketMQ Consumer started for topic: %s", topic)

	if err := c.consumer.Start(); err != nil {
		return fmt.Errorf("rocketmqw: failed to start consumer: %w", err)
	}

	// Wait for context cancellation to trigger a shutdown
	go func() {
		<-ctx.Done()
		logw.Infof("Context canceled, shutting down RocketMQ consumer for topic: %s", topic)
		_ = c.Close()
	}()

	return nil
}

// Close gracefully stops the push consumer and commits final offsets.
func (c *rocketConsumer) Close() error {
	return c.consumer.Shutdown()
}

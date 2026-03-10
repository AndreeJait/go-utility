// Package rabbitmqw implements the brokerw interfaces for RabbitMQ (AMQP 0.9.1).
package rabbitmqw

import (
	"context"
	"fmt"

	"github.com/AndreeJait/go-utility/v2/brokerw"
	"github.com/AndreeJait/go-utility/v2/logw"
	amqp "github.com/rabbitmq/amqp091-go"
)

// rabbitProducer implements brokerw.Producer for RabbitMQ.
type rabbitProducer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewProducer initializes a persistent RabbitMQ publisher.
func NewProducer(amqpURL string) (brokerw.Producer, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmqw: failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmqw: failed to open a channel: %w", err)
	}

	return &rabbitProducer{conn: conn, channel: ch}, nil
}

// Send publishes a message to an exchange.
// If exchange is an empty string, it routes directly to the queue matching the routing key.
func (p *rabbitProducer) Send(ctx context.Context, exchange string, routingKey []byte, payload []byte) error {
	return p.channel.PublishWithContext(ctx,
		exchange,
		string(routingKey),
		false, // Mandatory
		false, // Immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent, // Require messages to be saved to disk
			ContentType:  "application/octet-stream",
			Body:         payload,
		})
}

// BulkSend iterates and publishes multiple messages efficiently over the same TCP channel.
func (p *rabbitProducer) BulkSend(ctx context.Context, exchange string, routingKeys, payloads [][]byte) error {
	if len(routingKeys) != len(payloads) {
		return fmt.Errorf("rabbitmqw: keys and payloads slices must have identical lengths")
	}

	for i, payload := range payloads {
		if err := p.Send(ctx, exchange, routingKeys[i], payload); err != nil {
			return fmt.Errorf("rabbitmqw: failed to send message at index %d: %w", i, err)
		}
	}
	return nil
}

// Close gracefully tears down the channel and the AMQP connection.
func (p *rabbitProducer) Close() error {
	p.channel.Close()
	return p.conn.Close()
}

// rabbitConsumer implements brokerw.Consumer for RabbitMQ.
type rabbitConsumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewConsumer initializes a RabbitMQ subscriber with a basic Quality of Service (QoS)
// prefetch limit to prevent overwhelming the worker.
func NewConsumer(amqpURL string) (brokerw.Consumer, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmqw: failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmqw: failed to open a channel: %w", err)
	}

	// Prefetch up to 100 unacknowledged messages per consumer to optimize throughput
	_ = ch.Qos(100, 0, false)

	return &rabbitConsumer{conn: conn, channel: ch}, nil
}

// Consume starts a background goroutine to process messages from a specified queue.
// It requires explicit manual acknowledgments based on handler success.
func (c *rabbitConsumer) Consume(ctx context.Context, queueName string, handlers ...brokerw.Handler) error {
	msgs, err := c.channel.Consume(
		queueName,
		"",    // Consumer tag
		false, // Auto-Ack disabled (requires manual Ack/Nack)
		false, // Exclusive
		false, // No-local
		false, // No-wait
		nil,   // Args
	)
	if err != nil {
		return fmt.Errorf("rabbitmqw: failed to register consumer: %w", err)
	}

	logw.Infof("RabbitMQ Consumer listening on queue: %s", queueName)

	go func() {
		for {
			select {
			case d, ok := <-msgs:
				if !ok {
					logw.Infof("RabbitMQ Consumer channel closed for queue: %s", queueName)
					return
				}

				stdMsg := &brokerw.Message{
					Topic:   queueName,
					Key:     []byte(d.RoutingKey),
					Payload: d.Body,
				}

				// Execute Middleware Chain
				if err := brokerw.ExecuteHandlers(ctx, stdMsg, handlers...); err != nil {
					logw.Errorf("rabbitmqw: handler failed for queue %s: %v", queueName, err)
					// Nack and explicitly requeue the message so it can be retried
					_ = d.Nack(false, true)
					continue
				}

				// Manual Ack: Successfully processed
				_ = d.Ack(false)

			case <-ctx.Done():
				logw.Infof("Context canceled, stopping RabbitMQ consumer for queue: %s", queueName)
				return
			}
		}
	}()

	return nil
}

// Close gracefully terminates the channel and connection, preventing new messages
// from being pushed while allowing existing ones to finish processing.
func (c *rabbitConsumer) Close() error {
	c.channel.Close()
	return c.conn.Close()
}

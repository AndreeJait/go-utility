// Package brokerw provides a unified, strategy-based interface for message brokers.
// It abstracts away the underlying implementations (Kafka, RabbitMQ, NSQ, etc.)
// while supporting robust middleware chaining, single sends, and bulk sends.
package brokerw

import "context"

// Message represents a standardized event payload received from any broker.
// This abstraction ensures that consumer logic does not depend on broker-specific types.
type Message struct {
	Topic   string
	Key     []byte            // Optional: Used by brokers like Kafka for partition routing.
	Payload []byte            // The raw event data (usually JSON or Protobuf).
	Headers map[string]string // Optional metadata headers.
}

// Handler defines the signature for processing an incoming message.
// Handlers can be chained together like HTTP middleware.
// If a Handler returns an error, the consumer wrapper will automatically Nack/Retry the message.
// If it returns nil, the message proceeds to the next handler or gets Acked.
type Handler func(ctx context.Context, msg *Message) error

// Producer defines the contract for publishing events to a message broker.
type Producer interface {
	// Send publishes a single message to the specified topic or exchange.
	Send(ctx context.Context, topic string, key, payload []byte) error

	// BulkSend publishes multiple messages efficiently in a single batch or network request.
	// The lengths of keys and payloads slices must be identical.
	BulkSend(ctx context.Context, topic string, keys, payloads [][]byte) error

	// Close safely terminates the producer connection.
	// This should be registered with the graceful shutdown utility.
	Close() error
}

// Consumer defines the contract for subscribing to events from a message broker.
type Consumer interface {
	// Consume subscribes to a topic/queue and processes incoming messages.
	// It accepts a variadic list of Handlers, allowing for middleware chaining
	// (e.g., RequestID Injection -> Logging -> Validation -> Business Logic).
	Consume(ctx context.Context, topic string, handlers ...Handler) error

	// Close safely stops the consumer from receiving new messages and closes connections.
	Close() error
}

// ExecuteHandlers is a global utility used internally by broker implementations
// to run the middleware chain sequentially. It halts execution and returns an error
// immediately if any handler in the chain fails.
func ExecuteHandlers(ctx context.Context, msg *Message, handlers ...Handler) error {
	for _, h := range handlers {
		if err := h(ctx, msg); err != nil {
			return err // Halts the chain and triggers a Nack/Retry
		}
	}
	return nil // Triggers an Ack
}

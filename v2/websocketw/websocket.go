// Package websocketw provides a unified, strategy-based interface for WebSocket servers and clients.
// It abstracts away the underlying implementations (Gorilla, Nhooyr, etc.)
// while supporting robust middleware chaining, event routing, and room-based broadcasting.
package websocketw

import (
	"context"
	"net/http"
)

// EventType identifies the kind of WebSocket message being sent or received.
// Applications define their own event constants (e.g., "chat.message", "order.updated").
type EventType string

// Message represents a standardized WebSocket event payload.
// All inbound and outbound messages are normalized to this structure.
type Message struct {
	Event   EventType         `json:"event"`   // The event type (e.g., "chat.message")
	Payload []byte            `json:"payload"` // The raw event data (usually JSON or binary)
	Headers map[string]string `json:"headers"` // Optional metadata headers
}

// ClientInfo holds metadata about a connected WebSocket client.
type ClientInfo struct {
	ID       string         // Unique connection identifier (auto-generated UUID)
	UserID   string         // Authenticated user ID (from authw.Result, empty if no auth)
	Username string         // Authenticated username
	Roles    []string       // Authenticated user roles
	Data     map[string]any // Arbitrary connection-scoped metadata
}

// Handler defines the signature for processing an incoming WebSocket message.
// Handlers can be chained together like HTTP middleware.
// If a Handler returns an error, the chain halts.
type Handler func(ctx context.Context, msg *Message) error

// ConnectHandler is called when a client successfully connects.
type ConnectHandler func(ctx context.Context, client *ClientInfo) error

// DisconnectHandler is called when a client disconnects.
type DisconnectHandler func(ctx context.Context, client *ClientInfo)

// ClientConnectHandler is called when the WebSocket client successfully connects
// to an external server.
type ClientConnectHandler func(ctx context.Context) error

// ClientDisconnectHandler is called when the WebSocket client disconnects from
// the external server.
type ClientDisconnectHandler func(ctx context.Context)

// Server defines the contract for a WebSocket server.
// Implementations must also satisfy http.Handler via ServeHTTP,
// allowing them to be mounted on any HTTP router (Echo, Gin, Mux, net/http).
type Server interface {
	// ServeHTTP handles the HTTP-to-WebSocket upgrade request.
	// This makes Server implement http.Handler, so it can be mounted directly
	// on any router:
	//   Echo:  e.GET("/ws", echo.Wrap(srv))
	//   Gin:   r.GET("/ws", gin.WrapH(srv))
	//   Mux:   r.Handle("/ws", srv)
	ServeHTTP(w http.ResponseWriter, r *http.Request)

	// Send delivers a message to a specific connected client by client ID.
	Send(ctx context.Context, clientID string, msg *Message) error

	// Broadcast sends a message to all currently connected clients.
	Broadcast(ctx context.Context, msg *Message) error

	// BroadcastToRoom sends a message to all clients in a specific room/channel.
	BroadcastToRoom(ctx context.Context, room string, msg *Message) error

	// JoinRoom adds a client to a room/channel for targeted broadcasting.
	JoinRoom(ctx context.Context, clientID string, room string) error

	// LeaveRoom removes a client from a room/channel.
	LeaveRoom(ctx context.Context, clientID string, room string) error

	// OnEvent registers handlers for a specific event type.
	// When an inbound message has a matching event, these handlers are invoked.
	// Multiple calls for the same event append to the handler chain (middleware stacking).
	OnEvent(event EventType, handlers ...Handler)

	// OnConnect registers a handler invoked when a client successfully connects.
	OnConnect(handler ConnectHandler)

	// OnDisconnect registers a handler invoked when a client disconnects.
	OnDisconnect(handler DisconnectHandler)

	// Close gracefully shuts down the WebSocket server and disconnects all clients.
	// This should be registered with gracefulw for clean shutdown:
	//   gracefulw.Register("WebSocket", ws.Close)
	Close() error
}

// Client defines the contract for a WebSocket client that connects
// to an external WebSocket server, listens for events, and publishes messages.
type Client interface {
	// Connect establishes a WebSocket connection to the server and starts
	// listening for incoming messages. It blocks until the context is canceled
	// or a fatal connection error occurs.
	// If auto-reconnect is enabled, it will attempt to reconnect on disconnection.
	Connect(ctx context.Context) error

	// Send publishes a message to the connected WebSocket server.
	Send(ctx context.Context, msg *Message) error

	// OnEvent registers handlers for a specific event type received from the server.
	// When an inbound message has a matching event, these handlers are invoked.
	// Multiple calls for the same event append to the handler chain (middleware stacking).
	OnEvent(event EventType, handlers ...Handler)

	// OnConnect registers a handler invoked when the client successfully connects
	// to the server (also called on reconnection).
	OnConnect(handler ClientConnectHandler)

	// OnDisconnect registers a handler invoked when the client disconnects from the server.
	OnDisconnect(handler ClientDisconnectHandler)

	// Close gracefully closes the WebSocket connection.
	// This should be registered with gracefulw for clean shutdown:
	//   gracefulw.Register("WSClient", c.Close)
	Close() error
}

// ExecuteHandlers is a global utility used internally by WebSocket implementations
// to run the middleware chain sequentially. It halts execution and returns an error
// immediately if any handler in the chain fails.
func ExecuteHandlers(ctx context.Context, msg *Message, handlers ...Handler) error {
	for _, h := range handlers {
		if err := h(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}
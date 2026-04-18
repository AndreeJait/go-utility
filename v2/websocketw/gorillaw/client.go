package gorillaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/AndreeJait/go-utility/v2/websocketw"
	"github.com/gorilla/websocket"
)

// Compile-time interface compliance check.
var _ websocketw.Client = (*wsClient)(nil)

// ClientConfig holds the configuration options for the WebSocket client.
type ClientConfig struct {
	// URL is the WebSocket server URL to connect to (e.g., "ws://localhost:8080/ws").
	URL string

	// Header is optional HTTP headers sent during the handshake (e.g., for auth tokens).
	Header http.Header

	// AutoReconnect enables automatic reconnection when the connection drops.
	// Default: false.
	AutoReconnect bool

	// ReconnectInterval is the delay between reconnection attempts.
	// Default: 3 seconds.
	ReconnectInterval time.Duration

	// MaxReconnectAttempts limits reconnection tries. 0 = unlimited.
	MaxReconnectAttempts int

	// DebugMode enables verbose logging of client operations.
	DebugMode bool
}

type wsClient struct {
	cfg          *ClientConfig
	conn         *websocket.Conn
	connMu       sync.Mutex
	eventMap     map[websocketw.EventType][]websocketw.Handler
	onConnect    websocketw.ClientConnectHandler
	onDisconnect websocketw.ClientDisconnectHandler
	mu           sync.RWMutex
	done         chan struct{}
}

// NewClient creates a new gorilla/websocket-based WebSocket client.
func NewClient(cfg *ClientConfig) websocketw.Client {
	if cfg == nil {
		cfg = &ClientConfig{}
	}
	if cfg.ReconnectInterval == 0 {
		cfg.ReconnectInterval = 3 * time.Second
	}
	return &wsClient{
		cfg:      cfg,
		eventMap: make(map[websocketw.EventType][]websocketw.Handler),
		done:     make(chan struct{}),
	}
}

// Connect establishes a WebSocket connection and starts listening for messages.
// It blocks until the context is canceled or a fatal error occurs.
func (c *wsClient) Connect(ctx context.Context) error {
	return c.connectLoop(ctx)
}

func (c *wsClient) connectLoop(ctx context.Context) error {
	attempts := 0

	for {
		select {
		case <-c.done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := c.dial(ctx)
		if err != nil {
			logw.CtxErrorf(ctx, "[WS-CLIENT] dial failed: %v", err)

			if !c.cfg.AutoReconnect {
				return fmt.Errorf("gorillaw: connect failed: %w", err)
			}

			attempts++
			if c.cfg.MaxReconnectAttempts > 0 && attempts > c.cfg.MaxReconnectAttempts {
				return fmt.Errorf("gorillaw: max reconnect attempts (%d) reached", c.cfg.MaxReconnectAttempts)
			}

			logw.CtxInfof(ctx, "[WS-CLIENT] reconnecting in %v (attempt %d)...", c.cfg.ReconnectInterval, attempts)

			select {
			case <-time.After(c.cfg.ReconnectInterval):
				continue
			case <-ctx.Done():
				return ctx.Err()
			case <-c.done:
				return nil
			}
		}

		// Reset attempt counter on successful connection
		attempts = 0

		// Invoke onConnect handler
		if c.onConnect != nil {
			if err := c.onConnect(ctx); err != nil {
				logw.CtxErrorf(ctx, "[WS-CLIENT] onConnect handler failed: %v", err)
			}
		}

		// Read messages until disconnection
		readErr := c.readMessages(ctx, conn)

		// Invoke onDisconnect handler
		if c.onDisconnect != nil {
			c.onDisconnect(ctx)
		}

		// If not auto-reconnect, return the error
		if !c.cfg.AutoReconnect {
			if readErr != nil {
				return readErr
			}
			return nil
		}

		// Wait before reconnecting
		logw.CtxInfof(ctx, "[WS-CLIENT] disconnected, reconnecting in %v...", c.cfg.ReconnectInterval)
		select {
		case <-time.After(c.cfg.ReconnectInterval):
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
			return nil
		}
	}
}

func (c *wsClient) dial(ctx context.Context) (*websocket.Conn, error) {
	u, err := url.Parse(c.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("gorillaw: invalid URL: %w", err)
	}

	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else if u.Scheme == "http" {
		u.Scheme = "ws"
	}

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, u.String(), c.cfg.Header)
	if err != nil {
		return nil, err
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	logw.CtxInfof(ctx, "[WS-CLIENT] connected to %s", u.String())
	return conn, nil
}

func (c *wsClient) readMessages(ctx context.Context, conn *websocket.Conn) error {
	for {
		select {
		case <-c.done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logw.CtxErrorf(ctx, "[WS-CLIENT] unexpected close: %v", err)
				return err
			}
			logw.CtxInfof(ctx, "[WS-CLIENT] connection closed: %v", err)
			return nil
		}

		if c.cfg.DebugMode {
			logw.CtxInfof(ctx, "[WS-CLIENT] received message: %s", string(message))
		}

		var msg websocketw.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			logw.CtxErrorf(ctx, "[WS-CLIENT] failed to unmarshal message: %v", err)
			continue
		}

		// Look up registered handlers for this event type
		c.mu.RLock()
		handlers, ok := c.eventMap[msg.Event]
		c.mu.RUnlock()

		if !ok || len(handlers) == 0 {
			logw.CtxInfof(ctx, "[WS-CLIENT] no handler registered for event '%s'", msg.Event)
			continue
		}

		// Execute the handler chain
		if err := websocketw.ExecuteHandlers(ctx, &msg, handlers...); err != nil {
			logw.CtxErrorf(ctx, "[WS-CLIENT] handler chain failed for event '%s': %v", msg.Event, err)
		}
	}
}

// Send publishes a message to the connected WebSocket server.
func (c *wsClient) Send(ctx context.Context, msg *websocketw.Message) error {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()

	if conn == nil {
		return fmt.Errorf("gorillaw: not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("gorillaw: failed to marshal message: %w", err)
	}

	c.connMu.Lock()
	defer c.connMu.Unlock()

	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("gorillaw: write failed: %w", err)
	}

	if c.cfg.DebugMode {
		logw.CtxInfof(ctx, "[WS-CLIENT] sent message: event=%s", msg.Event)
	}

	return nil
}

// OnEvent registers handlers for a specific event type received from the server.
func (c *wsClient) OnEvent(event websocketw.EventType, handlers ...websocketw.Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventMap[event] = append(c.eventMap[event], handlers...)
}

// OnConnect registers a handler invoked when the client successfully connects.
func (c *wsClient) OnConnect(handler websocketw.ClientConnectHandler) {
	c.onConnect = handler
}

// OnDisconnect registers a handler invoked when the client disconnects.
func (c *wsClient) OnDisconnect(handler websocketw.ClientDisconnectHandler) {
	c.onDisconnect = handler
}

// Close gracefully closes the WebSocket connection.
func (c *wsClient) Close() error {
	close(c.done)

	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		err := c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client shutting down"))
		c.conn.Close()
		c.conn = nil
		logw.Info("[WS-CLIENT] closed")
		return err
	}

	return nil
}
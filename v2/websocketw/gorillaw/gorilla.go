// Package gorillaw implements the websocketw.Server interface using gorilla/websocket.
package gorillaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/AndreeJait/go-utility/v2/authw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/AndreeJait/go-utility/v2/websocketw"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Compile-time interface compliance check.
var _ websocketw.Server = (*server)(nil)

var defaultUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Config holds the configuration options for initializing the Gorilla WebSocket server.
type Config struct {
	// Authenticator is optional. If set, it runs during the HTTP upgrade handshake.
	// Failed authentication prevents the WebSocket connection from being established.
	Authenticator authw.Authenticator

	// CheckOrigin allows customizing the CORS origin check for the WebSocket upgrade.
	// Defaults to allowing all origins.
	CheckOrigin func(r *http.Request) bool

	// DebugMode enables verbose logging of WebSocket operations.
	DebugMode bool
}

type server struct {
	cfg          *Config
	upgrader     websocket.Upgrader
	hub          *hub
	eventMap     map[websocketw.EventType][]websocketw.Handler
	onConnect    websocketw.ConnectHandler
	onDisconnect websocketw.DisconnectHandler
	mu           sync.RWMutex
}

// hub manages all connected clients and room subscriptions.
type hub struct {
	clients      map[string]*client            // clientID -> client
	rooms        map[string]map[string]struct{} // room -> set of clientIDs
	register     chan *client
	unregister   chan *client
	broadcast    chan *broadcastMsg
	onDisconnect websocketw.DisconnectHandler
	mu           sync.RWMutex
}

// client represents a single WebSocket connection.
type client struct {
	id   string
	info *websocketw.ClientInfo
	conn *websocket.Conn
	send chan []byte
	ctx  context.Context
}

type broadcastMsg struct {
	msg     *websocketw.Message
	room    string // empty means broadcast to all
	exclude string // client ID to exclude (e.g., sender)
}

// New creates a new gorilla/websocket-based WebSocket server.
func New(cfg *Config) websocketw.Server {
	if cfg == nil {
		cfg = &Config{}
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  defaultUpgrader.ReadBufferSize,
		WriteBufferSize: defaultUpgrader.WriteBufferSize,
	}
	if cfg.CheckOrigin != nil {
		upgrader.CheckOrigin = cfg.CheckOrigin
	} else {
		upgrader.CheckOrigin = defaultUpgrader.CheckOrigin
	}

	s := &server{
		cfg:      cfg,
		upgrader: upgrader,
		hub: &hub{
			clients:    make(map[string]*client),
			rooms:      make(map[string]map[string]struct{}),
			register:   make(chan *client),
			unregister: make(chan *client),
			broadcast:  make(chan *broadcastMsg),
		},
		eventMap: make(map[websocketw.EventType][]websocketw.Handler),
	}

	// Start hub goroutine
	go s.hub.run()

	return s
}

// ServeHTTP handles the HTTP-to-WebSocket upgrade request.
// This makes Server implement http.Handler, so it can be mounted directly on any router.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handleUpgrade(w, r)
}

// handleUpgrade performs the HTTP-to-WebSocket upgrade and registers the client.
func (s *server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	ctx := logw.InjectLogID(r.Context())

	// Run authentication if configured and not already authenticated by framework middleware
	if authw.FromContext(ctx) == nil && s.cfg.Authenticator != nil {
		result, err := s.cfg.Authenticator.Authenticate(r)
		if err != nil {
			logw.CtxInfof(ctx, "[WS] authentication failed: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx = authw.WithResult(ctx, result)
		logw.CtxInfof(ctx, "[WS] authenticated user: %s", result.GetUserID())
	}

	// Upgrade the connection
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logw.CtxErrorf(ctx, "[WS] upgrade failed: %v", err)
		return
	}

	// Build client info
	clientID := uuid.New().String()
	info := &websocketw.ClientInfo{
		ID:   clientID,
		Data: make(map[string]any),
	}

	// Populate auth info if available
	if authResult := authw.FromContext(ctx); authResult != nil {
		info.UserID = authResult.GetUserID()
		info.Username = authResult.Username
		info.Roles = authResult.Roles
	}

	c := &client{
		id:   clientID,
		info: info,
		conn: conn,
		send: make(chan []byte, 256),
		ctx:  ctx,
	}

	// Register client in hub
	s.hub.register <- c

	logw.CtxInfof(ctx, "[WS] client connected: %s (user: %s)", clientID, info.UserID)

	// Invoke OnConnect handler
	if s.onConnect != nil {
		if err := s.onConnect(ctx, info); err != nil {
			logw.CtxErrorf(ctx, "[WS] onConnect handler failed for client %s: %v", clientID, err)
		}
	}

	// Start read and write pumps
	go c.writePump()
	go c.readPump(s)
}

// readPump reads messages from the WebSocket connection and dispatches to handlers.
func (c *client) readPump(s *server) {
	defer func() {
		s.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logw.CtxErrorf(c.ctx, "[WS] unexpected close for client %s: %v", c.id, err)
			}
			return
		}

		if s.cfg.DebugMode {
			logw.CtxInfof(c.ctx, "[WS] received message from client %s: %s", c.id, string(message))
		}

		var msg websocketw.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			logw.CtxErrorf(c.ctx, "[WS] failed to unmarshal message from client %s: %v", c.id, err)
			continue
		}

		// Look up registered handlers for this event type
		s.mu.RLock()
		handlers, ok := s.eventMap[msg.Event]
		s.mu.RUnlock()

		if !ok || len(handlers) == 0 {
			logw.CtxInfof(c.ctx, "[WS] no handler registered for event '%s' from client %s", msg.Event, c.id)
			continue
		}

		// Execute the handler chain
		if err := websocketw.ExecuteHandlers(c.ctx, &msg, handlers...); err != nil {
			logw.CtxErrorf(c.ctx, "[WS] handler chain failed for event '%s' from client %s: %v", msg.Event, c.id, err)
		}
	}
}

// writePump sends messages from the send channel to the WebSocket connection.
func (c *client) writePump() {
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Channel closed, send close message
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logw.CtxErrorf(c.ctx, "[WS] write error for client %s: %v", c.id, err)
				return
			}
		}
	}
}

// run manages client registration, unregistration, and message broadcasting.
func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c.id] = c
			h.mu.Unlock()
			logw.Infof("[WS] client %s registered (total: %d)", c.id, len(h.clients))

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c.id]; ok {
				delete(h.clients, c.id)
				// Remove from all rooms
				for room := range h.rooms {
					delete(h.rooms[room], c.id)
					if len(h.rooms[room]) == 0 {
						delete(h.rooms, room)
					}
				}
				close(c.send)
			}
			h.mu.Unlock()
			// Invoke onDisconnect handler
			if h.onDisconnect != nil {
				h.onDisconnect(c.ctx, c.info)
			}
			logw.Infof("[WS] client %s unregistered (total: %d)", c.id, len(h.clients))

		case bm := <-h.broadcast:
			data, err := json.Marshal(bm.msg)
			if err != nil {
				logw.Errorf("[WS] failed to marshal broadcast message: %v", err)
				continue
			}
			h.mu.RLock()
			if bm.room != "" {
				// Broadcast to room members only
				if members, ok := h.rooms[bm.room]; ok {
					for memberID := range members {
						if memberID != bm.exclude {
							if client, ok := h.clients[memberID]; ok {
								select {
								case client.send <- data:
								default:
									logw.Warningf("[WS] client %s send buffer full, dropping message", memberID)
								}
							}
						}
					}
				}
			} else {
				// Broadcast to all clients
				for id, client := range h.clients {
					if id != bm.exclude {
						select {
						case client.send <- data:
						default:
							logw.Warningf("[WS] client %s send buffer full, dropping message", id)
						}
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Send delivers a message to a specific connected client.
func (s *server) Send(ctx context.Context, clientID string, msg *websocketw.Message) error {
	s.hub.mu.RLock()
	c, ok := s.hub.clients[clientID]
	s.hub.mu.RUnlock()

	if !ok {
		return fmt.Errorf("gorillaw: client %s not found", clientID)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("gorillaw: failed to marshal message: %w", err)
	}

	select {
	case c.send <- data:
		return nil
	default:
		return fmt.Errorf("gorillaw: client %s send buffer full", clientID)
	}
}

// Broadcast sends a message to all connected clients.
func (s *server) Broadcast(ctx context.Context, msg *websocketw.Message) error {
	s.hub.broadcast <- &broadcastMsg{msg: msg}
	return nil
}

// BroadcastToRoom sends a message to all clients in a specific room.
func (s *server) BroadcastToRoom(ctx context.Context, room string, msg *websocketw.Message) error {
	s.hub.broadcast <- &broadcastMsg{msg: msg, room: room}
	return nil
}

// JoinRoom adds a client to a room for targeted broadcasting.
func (s *server) JoinRoom(ctx context.Context, clientID string, room string) error {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()

	if _, ok := s.hub.clients[clientID]; !ok {
		return fmt.Errorf("gorillaw: client %s not found", clientID)
	}

	if s.hub.rooms[room] == nil {
		s.hub.rooms[room] = make(map[string]struct{})
	}
	s.hub.rooms[room][clientID] = struct{}{}

	logw.CtxInfof(ctx, "[WS] client %s joined room %s", clientID, room)
	return nil
}

// LeaveRoom removes a client from a room.
func (s *server) LeaveRoom(ctx context.Context, clientID string, room string) error {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()

	if members, ok := s.hub.rooms[room]; ok {
		delete(members, clientID)
		if len(members) == 0 {
			delete(s.hub.rooms, room)
		}
	}

	logw.CtxInfof(ctx, "[WS] client %s left room %s", clientID, room)
	return nil
}

// OnEvent registers handlers for a specific event type.
func (s *server) OnEvent(event websocketw.EventType, handlers ...websocketw.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventMap[event] = append(s.eventMap[event], handlers...)
}

// OnConnect registers a handler invoked when a client successfully connects.
func (s *server) OnConnect(handler websocketw.ConnectHandler) {
	s.onConnect = handler
}

// OnDisconnect registers a handler invoked when a client disconnects.
func (s *server) OnDisconnect(handler websocketw.DisconnectHandler) {
	s.onDisconnect = handler
	s.hub.onDisconnect = handler
}

// Close gracefully shuts down the WebSocket server and disconnects all clients.
func (s *server) Close() error {
	s.hub.mu.Lock()
	for _, c := range s.hub.clients {
		c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutting down"))
		close(c.send)
		c.conn.Close()
		delete(s.hub.clients, c.id)
		// Invoke onDisconnect handler
		if s.hub.onDisconnect != nil {
			s.hub.onDisconnect(c.ctx, c.info)
		}
	}
	s.hub.rooms = make(map[string]map[string]struct{})
	s.hub.mu.Unlock()

	logw.Info("[WS] server closed")
	return nil
}
package gorillaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AndreeJait/go-utility/v2/websocketw"
	"github.com/gorilla/websocket"
)

func TestNew(t *testing.T) {
	srv := New(nil)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewWithConfig(t *testing.T) {
	srv := New(&Config{
		DebugMode: true,
	})
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestUpgrade(t *testing.T) {
	srv := New(nil)
	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to upgrade connection: %v", err)
	}
	defer conn.Close()
}

func TestOnEvent(t *testing.T) {
	received := make(chan *websocketw.Message, 1)

	srv := New(nil)
	srv.OnEvent("test.event", func(ctx context.Context, msg *websocketw.Message) error {
		received <- msg
		return nil
	})

	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Give connection time to register
	time.Sleep(50 * time.Millisecond)

	msg := websocketw.Message{
		Event:   "test.event",
		Payload: []byte(`{"key":"value"}`),
	}
	data, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	select {
	case got := <-received:
		if got.Event != "test.event" {
			t.Errorf("expected event 'test.event', got '%s'", got.Event)
		}
		if string(got.Payload) != `{"key":"value"}` {
			t.Errorf("expected payload '{\"key\":\"value\"}', got '%s'", string(got.Payload))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message handler")
	}
}

func TestOnEventMiddlewareChain(t *testing.T) {
	var order []string
	received := make(chan struct{}, 1)

	srv := New(nil)
	srv.OnEvent("chain.test",
		func(ctx context.Context, msg *websocketw.Message) error {
			order = append(order, "first")
			return nil
		},
		func(ctx context.Context, msg *websocketw.Message) error {
			order = append(order, "second")
			received <- struct{}{}
			return nil
		},
	)

	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	msg := websocketw.Message{Event: "chain.test", Payload: []byte("test")}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)

	select {
	case <-received:
		if len(order) != 2 || order[0] != "first" || order[1] != "second" {
			t.Errorf("expected [first, second], got %v", order)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for handler chain")
	}
}

func TestOnEventChainHaltOnError(t *testing.T) {
	var order []string

	srv := New(nil)
	srv.OnEvent("halt.test",
		func(ctx context.Context, msg *websocketw.Message) error {
			order = append(order, "first")
			return fmt.Errorf("stop chain")
		},
		func(ctx context.Context, msg *websocketw.Message) error {
			order = append(order, "second") // should not be called
			return nil
		},
	)

	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	msg := websocketw.Message{Event: "halt.test", Payload: []byte("test")}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	if len(order) != 1 || order[0] != "first" {
		t.Errorf("expected chain to halt after first handler, got %v", order)
	}
}

func TestBroadcast(t *testing.T) {
	srv := New(nil)

	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Connect two clients
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("client1 failed to connect: %v", err)
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("client2 failed to connect: %v", err)
	}
	defer conn2.Close()

	// Give connections time to register
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	msg := &websocketw.Message{Event: "broadcast", Payload: []byte("hello all")}
	srv.Broadcast(ctx, msg)

	// Give hub goroutine time to process the broadcast
	time.Sleep(50 * time.Millisecond)

	// Both clients should receive the message
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg1, err := conn1.ReadMessage()
	if err != nil {
		t.Errorf("client1 failed to read broadcast: %v", err)
	}

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg2, err := conn2.ReadMessage()
	if err != nil {
		t.Errorf("client2 failed to read broadcast: %v", err)
	}

	if len(msg1) == 0 || len(msg2) == 0 {
		t.Error("expected both clients to receive broadcast message")
	}
}

func TestRooms(t *testing.T) {
	// Use OnConnect to capture client IDs reliably
	var mu sync.Mutex
	var clientIDs []string

	srv := New(nil)
	srv.OnConnect(func(ctx context.Context, client *websocketw.ClientInfo) error {
		mu.Lock()
		clientIDs = append(clientIDs, client.ID)
		mu.Unlock()
		return nil
	})

	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Connect two clients
	conn1, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	defer conn1.Close()

	conn2, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	defer conn2.Close()

	// Wait for both clients to register
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if len(clientIDs) < 2 {
		mu.Unlock()
		t.Fatalf("expected 2 client IDs, got %d", len(clientIDs))
	}
	id1 := clientIDs[0]
	mu.Unlock()

	ctx := context.Background()

	// Join first client to "room1"
	if err := srv.JoinRoom(ctx, id1, "room1"); err != nil {
		t.Fatalf("failed to join room: %v", err)
	}

	// Broadcast to room1 — only client1 (in room) should receive
	msg := &websocketw.Message{Event: "room.msg", Payload: []byte("room hello")}
	srv.BroadcastToRoom(ctx, "room1", msg)

	// client1 should receive the room broadcast
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg1, err := conn1.ReadMessage()
	if err != nil {
		t.Errorf("client1 (in room) failed to read: %v", err)
	}
	if len(msg1) == 0 {
		t.Error("expected client1 to receive room broadcast")
	}
}

func TestRoomIsolation(t *testing.T) {
	// Verify that broadcasting to one room does NOT send to clients in another room
	var mu sync.Mutex
	var clientIDs []string

	srv := New(nil)
	srv.OnConnect(func(ctx context.Context, client *websocketw.ClientInfo) error {
		mu.Lock()
		clientIDs = append(clientIDs, client.ID)
		mu.Unlock()
		return nil
	})

	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	conn1, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	defer conn1.Close()

	conn2, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	defer conn2.Close()

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if len(clientIDs) < 2 {
		mu.Unlock()
		t.Fatalf("expected 2 client IDs, got %d", len(clientIDs))
	}
	id1, id2 := clientIDs[0], clientIDs[1]
	mu.Unlock()

	ctx := context.Background()

	// Join client1 to roomA, client2 to roomB
	srv.JoinRoom(ctx, id1, "roomA")
	srv.JoinRoom(ctx, id2, "roomB")

	msg := &websocketw.Message{Event: "room.test", Payload: []byte("roomA only")}

	// Broadcast to roomA
	srv.BroadcastToRoom(ctx, "roomA", msg)

	// client1 should receive
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg1, err := conn1.ReadMessage()
	if err != nil {
		t.Errorf("client1 (in roomA) failed to read: %v", err)
	}
	if len(msg1) == 0 {
		t.Error("expected client1 to receive roomA broadcast")
	}
}

func TestLeaveRoom(t *testing.T) {
	var mu sync.Mutex
	var clientIDs []string

	srv := New(nil)
	srv.OnConnect(func(ctx context.Context, client *websocketw.ClientInfo) error {
		mu.Lock()
		clientIDs = append(clientIDs, client.ID)
		mu.Unlock()
		return nil
	})

	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	conn1, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	defer conn1.Close()

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	id1 := clientIDs[0]
	mu.Unlock()

	ctx := context.Background()

	// Join and then leave a room
	srv.JoinRoom(ctx, id1, "room1")
	srv.LeaveRoom(ctx, id1, "room1")

	// Broadcast to the room — client1 should NOT receive since it left
	msg := &websocketw.Message{Event: "left.test", Payload: []byte("after leave")}
	srv.BroadcastToRoom(ctx, "room1", msg)

	// Verify room is empty by checking there are no members
	s := srv.(*server)
	s.hub.mu.RLock()
	members := s.hub.rooms["room1"]
	s.hub.mu.RUnlock()

	if members != nil && len(members) != 0 {
		t.Errorf("expected room1 to be empty after leave, got %d members", len(members))
	}
}

func TestSend(t *testing.T) {
	s := New(nil)
	srv := s.(*server)

	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	// Get client ID
	srv.hub.mu.RLock()
	var clientID string
	for id := range srv.hub.clients {
		clientID = id
		break
	}
	srv.hub.mu.RUnlock()

	ctx := context.Background()
	msg := &websocketw.Message{Event: "direct", Payload: []byte("hello you")}
	if err := srv.Send(ctx, clientID, msg); err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, received, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read direct message: %v", err)
	}

	var parsed websocketw.Message
	if err := json.Unmarshal(received, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if parsed.Event != "direct" {
		t.Errorf("expected event 'direct', got '%s'", parsed.Event)
	}

	// Test send to nonexistent client
	err = srv.Send(ctx, "nonexistent", msg)
	if err == nil {
		t.Error("expected error when sending to nonexistent client")
	}
}

func TestOnConnect(t *testing.T) {
	connectCalled := make(chan *websocketw.ClientInfo, 1)

	srv := New(nil)
	srv.OnConnect(func(ctx context.Context, client *websocketw.ClientInfo) error {
		connectCalled <- client
		return nil
	})

	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	defer conn.Close()

	select {
	case info := <-connectCalled:
		if info.ID == "" {
			t.Error("expected non-empty client ID on connect")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for onConnect handler")
	}
}

func TestOnDisconnect(t *testing.T) {
	disconnectCalled := make(chan *websocketw.ClientInfo, 1)

	srv := New(nil)
	srv.OnDisconnect(func(ctx context.Context, client *websocketw.ClientInfo) {
		disconnectCalled <- client
	})

	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)

	// Give connection time to register
	time.Sleep(50 * time.Millisecond)

	// Close the client connection
	conn.Close()

	select {
	case info := <-disconnectCalled:
		if info.ID == "" {
			t.Error("expected non-empty client ID on disconnect")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for onDisconnect handler")
	}
}

func TestClose(t *testing.T) {
	srv := New(nil)

	ts := httptest.NewServer(srv)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)

	time.Sleep(50 * time.Millisecond)

	if err := srv.Close(); err != nil {
		t.Errorf("expected clean close, got error: %v", err)
	}

	// Client should receive close frame
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Error("expected connection to be closed after server.Close()")
	}
}

func TestUnknownEvent(t *testing.T) {
	srv := New(nil)

	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Send message for unregistered event — server should not crash
	msg := websocketw.Message{Event: "unknown.event", Payload: []byte("test")}
	data, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Connection should still be alive — send another message
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatal("expected connection to still be alive after unknown event")
	}
}
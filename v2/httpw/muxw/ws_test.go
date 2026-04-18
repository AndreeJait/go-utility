package muxw

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AndreeJait/go-utility/v2/websocketw"
	"github.com/AndreeJait/go-utility/v2/websocketw/gorillaw"
	"github.com/gorilla/websocket"
)

func TestMux_WSUpgradeHandler(t *testing.T) {
	ws := gorillaw.New(nil)

	r := New(&Config{DebugMode: true})
	r.HandleFunc("/ws", WSUpgradeHandler(ws))

	ts := httptest.NewServer(r)
	defer ts.Close()
	defer ws.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws", nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()
}

func TestMux_WSBroadcast(t *testing.T) {
	ws := gorillaw.New(nil)

	r := New(&Config{DebugMode: true})
	r.HandleFunc("/ws", WSUpgradeHandler(ws))

	ts := httptest.NewServer(r)
	defer ts.Close()
	defer ws.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	conn1, _, _ := websocket.DefaultDialer.Dial(wsURL+"/ws", nil)
	defer conn1.Close()

	conn2, _, _ := websocket.DefaultDialer.Dial(wsURL+"/ws", nil)
	defer conn2.Close()

	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	msg := &websocketw.Message{Event: "broadcast", Payload: []byte("hello from mux")}
	ws.Broadcast(ctx, msg)

	time.Sleep(50 * time.Millisecond)

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
		t.Error("expected both clients to receive broadcast")
	}

	// Verify the message content
	var parsed websocketw.Message
	if err := json.Unmarshal(msg1, &parsed); err != nil {
		t.Errorf("failed to unmarshal message: %v", err)
	}
	if parsed.Event != "broadcast" {
		t.Errorf("expected event 'broadcast', got '%s'", parsed.Event)
	}
}
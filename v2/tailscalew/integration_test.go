package tailscalew

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// skipIfNoCreds skips the integration test unless Tailscale credentials are
// available in the environment.
func skipIfNoCreds(t *testing.T) (tailnet, apiKey string) {
	t.Helper()

	tailnet = os.Getenv("TAILSCALE_TAILNET")
	apiKey = os.Getenv("TAILSCALE_API_KEY")

	if tailnet == "" || apiKey == "" {
		t.Skipf("skipping integration test: set TAILSCALE_TAILNET and TAILSCALE_API_KEY")
	}
	return tailnet, apiKey
}

func newIntegrationClient(t *testing.T) Tailscale {
	t.Helper()

	tailnet, apiKey := skipIfNoCreds(t)

	ts, err := New(&Config{
		Tailnet:   tailnet,
		APIKey:    apiKey,
		DebugMode: true,
	})
	if err != nil {
		t.Fatalf("failed to create tailscalew client: %v", err)
	}
	return ts
}

func TestIntegration_ListDevices(t *testing.T) {
	ts := newIntegrationClient(t)

	devices, err := ts.ListDevices(context.Background())
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}

	t.Logf("found %d devices", len(devices))
	for _, d := range devices {
		if d.NodeID == "" {
			t.Errorf("device %q has empty NodeID", d.Name)
		}
	}
}

func TestIntegration_GetDevice(t *testing.T) {
	ts := newIntegrationClient(t)

	devices, err := ts.ListDevices(context.Background())
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	if len(devices) == 0 {
		t.Skip("no devices in tailnet to test GetDevice")
	}

	nodeID := devices[0].NodeID
	got, err := ts.GetDevice(context.Background(), nodeID)
	if err != nil {
		t.Fatalf("GetDevice(%q) failed: %v", nodeID, err)
	}
	if got.NodeID != nodeID {
		t.Errorf("GetDevice returned wrong device: got %q, want %q", got.NodeID, nodeID)
	}
}

func TestIntegration_AuthKeyLifecycle(t *testing.T) {
	ts := newIntegrationClient(t)

	key, err := ts.CreateAuthKey(context.Background(), CreateAuthKeyRequest{
		Description: "tailscalew integration test",
		Expiry:      5 * time.Minute,
		Ephemeral:   true,
		Reusable:    false,
		Tags:        []string{"tag:tailscalew-test"},
	})
	if err != nil {
		t.Fatalf("CreateAuthKey failed: %v", err)
	}
	if key.ID == "" {
		t.Fatal("created auth key has empty ID")
	}
	if !strings.HasPrefix(key.Key, "tskey-auth-") {
		t.Errorf("unexpected key prefix: %q", key.Key)
	}

	t.Cleanup(func() {
		if err := ts.DeleteAuthKey(context.Background(), key.ID); err != nil {
			t.Logf("cleanup DeleteAuthKey(%q) failed: %v", key.ID, err)
		}
	})

	keys, err := ts.ListAuthKeys(context.Background())
	if err != nil {
		t.Fatalf("ListAuthKeys failed: %v", err)
	}

	found := false
	for _, k := range keys {
		if k.ID == key.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created auth key %q not found in ListAuthKeys", key.ID)
	}
}

package tsnetw

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/AndreeJait/go-utility/v2/tailscalew"
)

// skipIfNoAuthKey skips the integration test unless a Tailscale auth key is
// available in the TS_AUTHKEY environment variable.
func skipIfNoAuthKey(t *testing.T) string {
	t.Helper()

	key := os.Getenv("TS_AUTHKEY")
	if key == "" {
		key = os.Getenv("TAILSCALE_AUTHKEY")
	}
	if key == "" {
		t.Skip("skipping integration test: set TS_AUTHKEY or TAILSCALE_AUTHKEY")
	}
	return key
}

// skipIfNoAPIToken skips the integration test unless Tailscale API credentials
// are available for creating auth keys programmatically.
func skipIfNoAPIToken(t *testing.T) (tailnet, apiKey string) {
	t.Helper()

	tailnet = os.Getenv("TAILSCALE_TAILNET")
	apiKey = os.Getenv("TAILSCALE_API_KEY")
	if tailnet == "" || apiKey == "" {
		t.Skip("skipping integration test: set TAILSCALE_TAILNET and TAILSCALE_API_KEY")
	}
	return tailnet, apiKey
}

func TestIntegration_VPN_StartAndStatus(t *testing.T) {
	authKey := skipIfNoAuthKey(t)

	dir := t.TempDir()
	vpn, err := New(&Config{
		Hostname:  "tsnetw-integration-test",
		AuthKey:   authKey,
		Dir:       dir,
		Ephemeral: true,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	status, err := vpn.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.BackendState != "Running" {
		t.Fatalf("expected BackendState Running, got %q", status.BackendState)
	}
	if len(status.TailscaleIPs) == 0 {
		t.Fatal("expected at least one Tailscale IP")
	}

	t.Logf("tsnetw connected: backend=%s ips=%v", status.BackendState, status.TailscaleIPs)

	// Re-fetching status via the local API should also work.
	fetched, err := vpn.Status(ctx)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if fetched.BackendState == "" {
		t.Fatal("fetched status has empty backend state")
	}

	// Clean up: closing the VPN should not error.
	if err := vpn.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestIntegration_VPN_WithCreatedAuthKey exercises the full agent lifecycle:
// create an auth key via tailscalew, use it to start an embedded tsnet node,
// verify connectivity, then clean up.
func TestIntegration_VPN_WithCreatedAuthKey(t *testing.T) {
	tailnet, apiKey := skipIfNoAPIToken(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	api, err := tailscalew.New(&tailscalew.Config{
		Tailnet: tailnet,
		APIKey:  apiKey,
	})
	if err != nil {
		t.Fatalf("failed to create tailscalew client: %v", err)
	}

	key, err := api.CreateAuthKey(ctx, tailscalew.CreateAuthKeyRequest{
		Description:   "tsnetw integration test",
		Expiry:        10 * time.Minute,
		Ephemeral:     true,
		Reusable:      false,
		Preauthorized: true,
		Tags:          []string{"tag:tsnetw-test"},
	})
	if err != nil {
		t.Fatalf("CreateAuthKey failed: %v", err)
	}
	if key.Key == "" {
		t.Fatal("created auth key has empty Key value")
	}

	t.Logf("created auth key %q for embedded node", key.ID)

	dir := t.TempDir()
	vpn, err := New(&Config{
		Hostname:      "tsnetw-key-provisioned",
		AuthKey:       key.Key,
		Dir:           dir,
		Ephemeral:     true,
		AdvertiseTags: []string{"tag:tsnetw-test"},
	})
	if err != nil {
		t.Fatalf("tsnetw.New failed: %v", err)
	}

	status, err := vpn.Start(ctx)
	if err != nil {
		t.Fatalf("vpn.Start failed: %v", err)
	}
	if status.BackendState != "Running" {
		t.Fatalf("expected Running, got %q", status.BackendState)
	}
	if len(status.TailscaleIPs) == 0 {
		t.Fatal("expected at least one Tailscale IP")
	}

	t.Logf("embedded node connected: backend=%s ips=%v name=%s", status.BackendState, status.TailscaleIPs, status.Self.Name)

	t.Cleanup(func() {
		_ = vpn.Close()
		if err := api.DeleteAuthKey(context.Background(), key.ID); err != nil {
			t.Logf("cleanup DeleteAuthKey(%q) failed: %v", key.ID, err)
		}
	})

	// Re-fetch status through the local API to confirm the node remains healthy.
	fetched, err := vpn.Status(ctx)
	if err != nil {
		t.Fatalf("vpn.Status failed: %v", err)
	}
	if fetched.BackendState == "" {
		t.Fatal("fetched status has empty backend state")
	}
}

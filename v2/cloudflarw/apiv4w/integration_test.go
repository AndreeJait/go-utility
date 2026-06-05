//go:build integration

package apiv4w

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/AndreeJait/go-utility/v2/cloudflarw"
)

// newIntegrationClient creates a Cloudflare client for integration testing.
// Requires CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID environment variables.
//
// Run with:
//
//	cd v2 && go test ./cloudflarw/apiv4w/ -tags=integration -run TestIntegration -v -timeout 120s
func newIntegrationClient(t *testing.T) cloudflarw.Cloudflare {
	t.Helper()

	token := os.Getenv("CLOUDFLARE_API_TOKEN")
	if token == "" {
		t.Fatal("CLOUDFLARE_API_TOKEN environment variable is required for integration tests")
	}

	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	if accountID == "" {
		t.Fatal("CLOUDFLARE_ACCOUNT_ID environment variable is required for integration tests")
	}

	cf, err := New(&Config{
		APIToken:  token,
		AccountID: accountID,
		DebugMode: true,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return cf
}

func TestIntegration_ListAccounts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cf := newIntegrationClient(t)
	accounts, info, err := cf.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	t.Logf("accounts: %+v", accounts)
	t.Logf("result_info: %+v", info)

	if len(accounts) == 0 {
		t.Fatal("expected at least one account")
	}
}

func TestIntegration_ListZones(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cf := newIntegrationClient(t)
	zones, info, err := cf.ListZones(ctx)
	if err != nil {
		t.Fatalf("ListZones: %v", err)
	}
	t.Logf("zones count: %d", len(zones))
	t.Logf("result_info: %+v", info)

	if len(zones) == 0 {
		t.Log("no zones found — this is normal if no domains are integrated")
	}
}

func TestIntegration_DNSRecords_CRUD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cf := newIntegrationClient(t)

	// Find a zone to test with
	zones, _, err := cf.ListZones(ctx)
	if err != nil {
		t.Fatalf("ListZones: %v", err)
	}
	if len(zones) == 0 {
		t.Skip("no zones available for DNS record testing")
	}

	zoneID := zones[0].ID
	zoneName := zones[0].Name
	t.Logf("using zone: %s (%s)", zoneName, zoneID)

	// Create a test TXT record
	recordName := fmt.Sprintf("_cfw-test.%s", zoneName)
	created, err := cf.CreateDNSRecord(ctx, zoneID, &cloudflarw.DNSRecord{
		Type:    "TXT",
		Name:    recordName,
		Content: "cloudflarw integration test",
		TTL:     120,
	})
	if err != nil {
		t.Fatalf("CreateDNSRecord: %v", err)
	}
	t.Logf("created DNS record: id=%s name=%s", created.ID, created.Name)

	// Clean up: delete the record after test
	defer func() {
		delCtx, delCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer delCancel()
		if err := cf.DeleteDNSRecord(delCtx, zoneID, created.ID); err != nil {
			t.Logf("cleanup: failed to delete DNS record %s: %v", created.ID, err)
		} else {
			t.Logf("cleanup: deleted DNS record %s", created.ID)
		}
	}()

	// Get the record
	got, err := cf.GetDNSRecord(ctx, zoneID, created.ID)
	if err != nil {
		t.Fatalf("GetDNSRecord: %v", err)
	}
	if got.Content != "cloudflarw integration test" {
		t.Fatalf("expected content 'cloudflarw integration test', got %q", got.Content)
	}
	t.Logf("got DNS record: id=%s content=%s", got.ID, got.Content)

	// Update the record
	updated, err := cf.UpdateDNSRecord(ctx, zoneID, created.ID, &cloudflarw.DNSRecord{
		Type:    "TXT",
		Name:    recordName,
		Content: "cloudflarw integration test updated",
		TTL:     120,
	})
	if err != nil {
		t.Fatalf("UpdateDNSRecord: %v", err)
	}
	if updated.Content != "cloudflarw integration test updated" {
		t.Fatalf("expected updated content, got %q", updated.Content)
	}
	t.Logf("updated DNS record: id=%s content=%s", updated.ID, updated.Content)

	// List records and verify our record exists
	records, _, err := cf.ListDNSRecords(ctx, zoneID)
	if err != nil {
		t.Fatalf("ListDNSRecords: %v", err)
	}
	found := false
	for _, r := range records {
		if r.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("created DNS record not found in list")
	}
	t.Logf("listed %d DNS records, found test record", len(records))
}

func TestIntegration_Tunnels_CRUD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cf := newIntegrationClient(t)

	// List existing tunnels
	tunnels, _, err := cf.ListTunnels(ctx, "")
	if err != nil {
		t.Fatalf("ListTunnels: %v", err)
	}
	t.Logf("existing tunnels: %d", len(tunnels))

	// Create a test tunnel
	tunnel, err := cf.CreateTunnel(ctx, "", "cfw-integration-test", "cfd_tunnel")
	if err != nil {
		t.Fatalf("CreateTunnel: %v", err)
	}
	t.Logf("created tunnel: id=%s name=%s", tunnel.ID, tunnel.Name)

	// Clean up: delete the tunnel after test
	defer func() {
		delCtx, delCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer delCancel()
		if err := cf.DeleteTunnel(delCtx, "", tunnel.ID); err != nil {
			t.Logf("cleanup: failed to delete tunnel %s: %v", tunnel.ID, err)
		} else {
			t.Logf("cleanup: deleted tunnel %s", tunnel.ID)
		}
	}()

	// Get the tunnel
	got, err := cf.GetTunnel(ctx, "", tunnel.ID)
	if err != nil {
		t.Fatalf("GetTunnel: %v", err)
	}
	if got.Name != "cfw-integration-test" {
		t.Fatalf("expected name cfw-integration-test, got %q", got.Name)
	}
	t.Logf("got tunnel: id=%s name=%s status=%s", got.ID, got.Name, got.Status)

	// Get tunnel token
	token, err := cf.GetTunnelToken(ctx, "", tunnel.ID)
	if err != nil {
		t.Logf("GetTunnelToken: %v (may fail if tunnel is inactive)", err)
	} else {
		t.Logf("tunnel token: %s...", token[:min(20, len(token))])
	}

	// Get tunnel config
	config, err := cf.GetTunnelConfig(ctx, "", tunnel.ID)
	if err != nil {
		t.Logf("GetTunnelConfig: %v (may fail for new tunnels)", err)
	} else {
		t.Logf("tunnel config ingress rules: %d", len(config.Config.Ingress))
	}
}

func TestIntegration_AccessApps_List(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cf := newIntegrationClient(t)

	apps, info, err := cf.ListAccessApps(ctx, "")
	if err != nil {
		t.Fatalf("ListAccessApps: %v", err)
	}
	t.Logf("access apps count: %d", len(apps))
	t.Logf("result_info: %+v", info)

	for i, app := range apps {
		t.Logf("app[%d]: id=%s name=%s domain=%s type=%s", i, app.ID, app.Name, app.Domain, app.Type)
	}
}

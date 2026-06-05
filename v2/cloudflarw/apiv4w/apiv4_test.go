package apiv4w

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AndreeJait/go-utility/v2/cloudflarw"
)

// --- Interface compliance ---

func TestInterfaceCompliance(t *testing.T) {
	var _ cloudflarw.Cloudflare = (*cfClient)(nil)
}

// --- Constructor tests ---

func TestNew_NilConfig(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_EmptyToken(t *testing.T) {
	_, err := New(&Config{APIToken: ""})
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	if !strings.Contains(err.Error(), "api token is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_ValidConfig(t *testing.T) {
	cf, err := New(&Config{APIToken: "test-token"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cf == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNew_DefaultBaseURL(t *testing.T) {
	client, err := New(&Config{APIToken: "test-token"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := client.(*cfClient)
	if c.baseURL != defaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", defaultBaseURL, c.baseURL)
	}
}

func TestNew_CustomBaseURL(t *testing.T) {
	client, err := New(&Config{APIToken: "test-token", BaseURL: "https://custom.api.url"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := client.(*cfClient)
	if c.baseURL != "https://custom.api.url" {
		t.Fatalf("expected custom base URL, got %q", c.baseURL)
	}
}

func TestNew_DefaultAccountID(t *testing.T) {
	client, err := New(&Config{APIToken: "test-token", AccountID: "acc-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := client.(*cfClient)
	if c.accountID != "acc-123" {
		t.Fatalf("expected account ID acc-123, got %q", c.accountID)
	}
}

// --- resolveAccountID tests ---

func TestResolveAccountID_Explicit(t *testing.T) {
	c := &cfClient{accountID: "default"}
	resolved, err := c.resolveAccountID("explicit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "explicit" {
		t.Fatalf("expected explicit, got %q", resolved)
	}
}

func TestResolveAccountID_Fallback(t *testing.T) {
	c := &cfClient{accountID: "default"}
	resolved, err := c.resolveAccountID("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "default" {
		t.Fatalf("expected default, got %q", resolved)
	}
}

func TestResolveAccountID_Empty(t *testing.T) {
	c := &cfClient{}
	_, err := c.resolveAccountID("")
	if err == nil {
		t.Fatal("expected error for empty account ID")
	}
	if !strings.Contains(err.Error(), "account id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Helper to create test server and client ---

func newTestClient(handler http.HandlerFunc) (cloudflarw.Cloudflare, *httptest.Server) {
	ts := httptest.NewServer(handler)
	cf, _ := New(&Config{
		APIToken: "test-token",
		BaseURL:  ts.URL,
	})
	return cf, ts
}

func envelope(result any, success bool, errors ...apiError) map[string]any {
	return map[string]any{
		"success": success,
		"errors":  errors,
		"result":  result,
	}
}

func envelopeWithInfo(result any, info *cloudflarw.ListResultInfo) map[string]any {
	return map[string]any{
		"success":     true,
		"errors":      []any{},
		"result":      result,
		"result_info": info,
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	w.Write(data)
}

// --- Accounts ---

func TestListAccounts_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/accounts" {
			t.Fatalf("expected /accounts, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}
		writeJSON(t, w, envelopeWithInfo([]map[string]string{
			{"id": "acc-1", "name": "My Account", "type": "standard"},
		}, &cloudflarw.ListResultInfo{Count: 1, Page: 1, PerPage: 20, TotalCount: 1}))
	})
	defer ts.Close()

	accounts, info, err := cf.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].ID != "acc-1" {
		t.Fatalf("expected acc-1, got %s", accounts[0].ID)
	}
	if info == nil || info.TotalCount != 1 {
		t.Fatalf("expected total_count 1, got %+v", info)
	}
}

func TestListAccounts_APIError(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, envelope(nil, false, apiError{Code: 1000, Message: "invalid token"}))
	})
	defer ts.Close()

	_, _, err := cf.ListAccounts(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "1000: invalid token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Zones ---

func TestListZones_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zones" {
			t.Fatalf("expected /zones, got %s", r.URL.Path)
		}
		writeJSON(t, w, envelopeWithInfo([]map[string]any{
			{"id": "zone-1", "name": "example.com", "status": "active", "account": map[string]string{"id": "acc-1", "name": "My Account"}},
		}, &cloudflarw.ListResultInfo{Count: 1, TotalCount: 1}))
	})
	defer ts.Close()

	zones, _, err := cf.ListZones(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(zones) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(zones))
	}
	if zones[0].Name != "example.com" {
		t.Fatalf("expected example.com, got %s", zones[0].Name)
	}
}

func TestGetZone_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zones/zone-1" {
			t.Fatalf("expected /zones/zone-1, got %s", r.URL.Path)
		}
		writeJSON(t, w, envelope(map[string]any{
			"id": "zone-1", "name": "example.com", "status": "active", "account": map[string]string{"id": "acc-1"},
		}, true))
	})
	defer ts.Close()

	zone, err := cf.GetZone(context.Background(), "zone-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if zone.ID != "zone-1" {
		t.Fatalf("expected zone-1, got %s", zone.ID)
	}
}

// --- DNS Records ---

func TestListDNSRecords_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zones/zone-1/dns_records" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeJSON(t, w, envelopeWithInfo([]map[string]any{
			{"id": "rec-1", "type": "A", "name": "www.example.com", "content": "1.2.3.4", "proxied": true, "ttl": 1},
		}, &cloudflarw.ListResultInfo{Count: 1, TotalCount: 1}))
	})
	defer ts.Close()

	records, _, err := cf.ListDNSRecords(context.Background(), "zone-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Type != "A" || records[0].Content != "1.2.3.4" {
		t.Fatalf("unexpected record: %+v", records[0])
	}
}

func TestGetDNSRecord_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, envelope(map[string]any{
			"id": "rec-1", "type": "CNAME", "name": "app.example.com", "content": "target.example.com", "proxied": false, "ttl": 3600,
		}, true))
	})
	defer ts.Close()

	record, err := cf.GetDNSRecord(context.Background(), "zone-1", "rec-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Type != "CNAME" {
		t.Fatalf("expected CNAME, got %s", record.Type)
	}
}

func TestCreateDNSRecord_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		if req["type"] != "TXT" {
			t.Fatalf("expected type TXT, got %v", req["type"])
		}

		writeJSON(t, w, envelope(map[string]any{
			"id": "rec-new", "type": "TXT", "name": "_test.example.com", "content": "test-value", "proxied": false, "ttl": 120,
		}, true))
	})
	defer ts.Close()

	record, err := cf.CreateDNSRecord(context.Background(), "zone-1", &cloudflarw.DNSRecord{
		Type: "TXT", Name: "_test.example.com", Content: "test-value", TTL: 120,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.ID != "rec-new" {
		t.Fatalf("expected rec-new, got %s", record.ID)
	}
}

func TestUpdateDNSRecord_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		writeJSON(t, w, envelope(map[string]any{
			"id": "rec-1", "type": "A", "name": "www.example.com", "content": "5.6.7.8", "proxied": true, "ttl": 1,
		}, true))
	})
	defer ts.Close()

	record, err := cf.UpdateDNSRecord(context.Background(), "zone-1", "rec-1", &cloudflarw.DNSRecord{
		Content: "5.6.7.8",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Content != "5.6.7.8" {
		t.Fatalf("expected 5.6.7.8, got %s", record.Content)
	}
}

func TestDeleteDNSRecord_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		writeJSON(t, w, envelope(map[string]string{"id": "rec-1"}, true))
	})
	defer ts.Close()

	err := cf.DeleteDNSRecord(context.Background(), "zone-1", "rec-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Tunnels ---

func TestListTunnels_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/acc-1/cfd_tunnel" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeJSON(t, w, envelopeWithInfo([]map[string]any{
			{"id": "tun-1", "name": "my-tunnel", "status": "healthy", "tun_type": "cfd_tunnel", "account_tag": "acc-1", "conns_count": 2, "created_at": "2024-01-01T00:00:00Z", "deleted_at": "", "remote_config": true},
		}, &cloudflarw.ListResultInfo{Count: 1, TotalCount: 1}))
	})
	defer ts.Close()

	tunnels, _, err := cf.ListTunnels(context.Background(), "acc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(tunnels))
	}
	if tunnels[0].Name != "my-tunnel" {
		t.Fatalf("expected my-tunnel, got %s", tunnels[0].Name)
	}
}

func TestListTunnels_DefaultAccountID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/default-acc/cfd_tunnel" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeJSON(t, w, envelopeWithInfo([]any{}, &cloudflarw.ListResultInfo{}))
	}))
	defer ts.Close()

	cf, _ := New(&Config{APIToken: "test-token", BaseURL: ts.URL, AccountID: "default-acc"})
	_, _, err := cf.ListTunnels(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTunnel_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, envelope(map[string]any{
			"id": "tun-1", "name": "my-tunnel", "status": "healthy", "tun_type": "cfd_tunnel",
		}, true))
	})
	defer ts.Close()

	tunnel, err := cf.GetTunnel(context.Background(), "acc-1", "tun-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tunnel.ID != "tun-1" {
		t.Fatalf("expected tun-1, got %s", tunnel.ID)
	}
}

func TestCreateTunnel_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		if req["name"] != "new-tunnel" {
			t.Fatalf("expected name new-tunnel, got %v", req["name"])
		}

		writeJSON(t, w, envelope(map[string]any{
			"id": "tun-new", "name": "new-tunnel", "status": "inactive", "tun_type": "cfd_tunnel", "account_tag": "acc-1", "conns_count": 0, "created_at": "2024-06-01T00:00:00Z", "deleted_at": "", "remote_config": true,
		}, true))
	})
	defer ts.Close()

	tunnel, err := cf.CreateTunnel(context.Background(), "acc-1", "new-tunnel", "cfd_tunnel")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tunnel.Name != "new-tunnel" {
		t.Fatalf("expected new-tunnel, got %s", tunnel.Name)
	}
}

func TestUpdateTunnel_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		writeJSON(t, w, envelope(map[string]any{
			"id": "tun-1", "name": "renamed-tunnel", "status": "healthy", "tun_type": "cfd_tunnel",
		}, true))
	})
	defer ts.Close()

	tunnel, err := cf.UpdateTunnel(context.Background(), "acc-1", "tun-1", &cloudflarw.Tunnel{Name: "renamed-tunnel"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tunnel.Name != "renamed-tunnel" {
		t.Fatalf("expected renamed-tunnel, got %s", tunnel.Name)
	}
}

func TestDeleteTunnel_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		writeJSON(t, w, envelope(map[string]string{"id": "tun-1"}, true))
	})
	defer ts.Close()

	err := cf.DeleteTunnel(context.Background(), "acc-1", "tun-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTunnelConfig_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, envelope(map[string]any{
			"config": map[string]any{
				"ingress": []map[string]any{
					{"hostname": "app.example.com", "service": "http://localhost:8080"},
					{"service": "http_status:404"},
				},
				"warp-routing": map[string]any{"enabled": true},
			},
		}, true))
	})
	defer ts.Close()

	config, err := cf.GetTunnelConfig(context.Background(), "acc-1", "tun-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(config.Config.Ingress) != 2 {
		t.Fatalf("expected 2 ingress rules, got %d", len(config.Config.Ingress))
	}
	if config.Config.Ingress[0].Hostname != "app.example.com" {
		t.Fatalf("expected app.example.com, got %s", config.Config.Ingress[0].Hostname)
	}
	if config.Config.WarpRouting == nil || !config.Config.WarpRouting.Enabled {
		t.Fatal("expected warp routing enabled")
	}
}

func TestUpdateTunnelConfig_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		writeJSON(t, w, envelope(map[string]any{}, true))
	})
	defer ts.Close()

	config := &cloudflarw.TunnelConfig{}
	config.Config.Ingress = []cloudflarw.IngressRule{
		{Hostname: "app.example.com", Service: "http://localhost:8080"},
		{Service: "http_status:404"},
	}

	err := cf.UpdateTunnelConfig(context.Background(), "acc-1", "tun-1", config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTunnelToken_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/acc-1/cfd_tunnel/tun-1/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeJSON(t, w, envelope("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test-token", true))
	})
	defer ts.Close()

	token, err := cf.GetTunnelToken(context.Background(), "acc-1", "tun-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test-token" {
		t.Fatalf("unexpected token: %s", token)
	}
}

// --- Access Applications ---

func TestListAccessApps_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/acc-1/access/apps" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeJSON(t, w, envelopeWithInfo([]map[string]any{
			{"id": "app-1", "name": "My App", "domain": "app.example.com", "type": "self_hosted", "aud": "aud-123"},
		}, &cloudflarw.ListResultInfo{Count: 1, TotalCount: 1}))
	})
	defer ts.Close()

	apps, _, err := cf.ListAccessApps(context.Background(), "acc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Name != "My App" {
		t.Fatalf("expected My App, got %s", apps[0].Name)
	}
}

func TestGetAccessApp_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, envelope(map[string]any{
			"id": "app-1", "name": "My App", "domain": "app.example.com", "type": "self_hosted", "aud": "aud-123",
		}, true))
	})
	defer ts.Close()

	app, err := cf.GetAccessApp(context.Background(), "acc-1", "app-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.ID != "app-1" {
		t.Fatalf("expected app-1, got %s", app.ID)
	}
}

func TestCreateAccessApp_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		writeJSON(t, w, envelope(map[string]any{
			"id": "app-new", "name": "New App", "domain": "new.example.com", "type": "self_hosted", "aud": "aud-new",
		}, true))
	})
	defer ts.Close()

	app, err := cf.CreateAccessApp(context.Background(), "acc-1", &cloudflarw.AccessApp{
		Name: "New App", Domain: "new.example.com", Type: "self_hosted",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.ID != "app-new" {
		t.Fatalf("expected app-new, got %s", app.ID)
	}
}

func TestUpdateAccessApp_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		writeJSON(t, w, envelope(map[string]any{
			"id": "app-1", "name": "Updated App", "domain": "updated.example.com", "type": "self_hosted", "aud": "aud-1",
		}, true))
	})
	defer ts.Close()

	app, err := cf.UpdateAccessApp(context.Background(), "acc-1", "app-1", &cloudflarw.AccessApp{
		Name: "Updated App", Domain: "updated.example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Name != "Updated App" {
		t.Fatalf("expected Updated App, got %s", app.Name)
	}
}

func TestDeleteAccessApp_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		writeJSON(t, w, envelope(map[string]string{"id": "app-1"}, true))
	})
	defer ts.Close()

	err := cf.DeleteAccessApp(context.Background(), "acc-1", "app-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Error path tests ---

func TestDoRequest_InvalidJSON(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	})
	defer ts.Close()

	_, _, err := cf.ListAccounts(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decoding response") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoRequest_MultipleAPIErrors(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"success": false,
			"errors": []map[string]any{
				{"code": 1000, "message": "first error"},
				{"code": 1001, "message": "second error"},
			},
		})
	})
	defer ts.Close()

	_, _, err := cf.ListAccounts(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "1000: first error") || !strings.Contains(err.Error(), "1001: second error") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTunnelMethods_MissingAccountID(t *testing.T) {
	cf, _ := New(&Config{APIToken: "test-token"})

	_, _, err := cf.ListTunnels(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "account id is required") {
		t.Fatalf("expected account id required error, got: %v", err)
	}

	_, err = cf.GetTunnel(context.Background(), "", "tun-1")
	if err == nil || !strings.Contains(err.Error(), "account id is required") {
		t.Fatalf("expected account id required error, got: %v", err)
	}

	err = cf.DeleteTunnel(context.Background(), "", "tun-1")
	if err == nil || !strings.Contains(err.Error(), "account id is required") {
		t.Fatalf("expected account id required error, got: %v", err)
	}
}

func TestAccessAppMethods_MissingAccountID(t *testing.T) {
	cf, _ := New(&Config{APIToken: "test-token"})

	_, _, err := cf.ListAccessApps(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "account id is required") {
		t.Fatalf("expected account id required error, got: %v", err)
	}

	err = cf.DeleteAccessApp(context.Background(), "", "app-1")
	if err == nil || !strings.Contains(err.Error(), "account id is required") {
		t.Fatalf("expected account id required error, got: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cf, _ := New(&Config{APIToken: "test-token", BaseURL: "https://httpbin.org"})
	_, _, err := cf.ListAccounts(ctx)
	if err == nil {
		t.Fatal("expected error due to cancelled context")
	}
	// The error could be from context cancellation or from the request itself
	if !strings.Contains(err.Error(), "apiv4w:") {
		t.Fatalf("expected apiv4w wrapped error, got: %v", err)
	}
}

func TestAuthHeader_SentOnEveryRequest(t *testing.T) {
	var capturedAuth string
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		writeJSON(t, w, envelope([]any{}, true))
	})
	defer ts.Close()

	_, _, _ = cf.ListAccounts(context.Background())
	if capturedAuth != "Bearer test-token" {
		t.Fatalf("expected Bearer test-token, got %q", capturedAuth)
	}
}

func TestContentTypeHeader_SentOnPostRequests(t *testing.T) {
	var capturedCT string
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		capturedCT = r.Header.Get("Content-Type")
		writeJSON(t, w, envelope(map[string]any{"id": "test"}, true))
	})
	defer ts.Close()

	_, _ = cf.CreateDNSRecord(context.Background(), "zone-1", &cloudflarw.DNSRecord{Type: "A", Name: "test", Content: "1.2.3.4"})
	if capturedCT != "application/json" {
		t.Fatalf("expected application/json, got %q", capturedCT)
	}
}

func TestDNSRecord_PriorityField(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, envelope(map[string]any{
			"id": "rec-1", "type": "MX", "name": "example.com", "content": "mail.example.com",
			"proxied": false, "ttl": 3600, "priority": 10,
		}, true))
	})
	defer ts.Close()

	record, err := cf.GetDNSRecord(context.Background(), "zone-1", "rec-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Priority == nil || *record.Priority != 10 {
		t.Fatalf("expected priority 10, got %v", record.Priority)
	}
}

func TestCustomHTTPClient(t *testing.T) {
	customClient := &http.Client{
		Transport: http.DefaultTransport,
	}
	cf, err := New(&Config{
		APIToken:   "test-token",
		HTTPClient: customClient,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := cf.(*cfClient)
	if c.client != customClient {
		t.Fatal("expected custom HTTP client to be used")
	}
}

func TestAllMethodPaths(t *testing.T) {
	tests := []struct {
		name string
		call func(cf cloudflarw.Cloudflare) error
		path string
	}{
		{
			"ListAccounts", func(cf cloudflarw.Cloudflare) error { _, _, err := cf.ListAccounts(context.Background()); return err }, "/accounts",
		},
		{
			"ListZones", func(cf cloudflarw.Cloudflare) error { _, _, err := cf.ListZones(context.Background()); return err }, "/zones",
		},
		{
			"GetZone", func(cf cloudflarw.Cloudflare) error { _, err := cf.GetZone(context.Background(), "z1"); return err }, "/zones/z1",
		},
		{
			"ListDNSRecords", func(cf cloudflarw.Cloudflare) error { _, _, err := cf.ListDNSRecords(context.Background(), "z1"); return err }, "/zones/z1/dns_records",
		},
		{
			"GetDNSRecord", func(cf cloudflarw.Cloudflare) error { _, err := cf.GetDNSRecord(context.Background(), "z1", "r1"); return err }, "/zones/z1/dns_records/r1",
		},
		{
			"DeleteDNSRecord", func(cf cloudflarw.Cloudflare) error { return cf.DeleteDNSRecord(context.Background(), "z1", "r1") }, "/zones/z1/dns_records/r1",
		},
		{
			"ListTunnels", func(cf cloudflarw.Cloudflare) error { _, _, err := cf.ListTunnels(context.Background(), "a1"); return err }, "/accounts/a1/cfd_tunnel",
		},
		{
			"GetTunnel", func(cf cloudflarw.Cloudflare) error { _, err := cf.GetTunnel(context.Background(), "a1", "t1"); return err }, "/accounts/a1/cfd_tunnel/t1",
		},
		{
			"DeleteTunnel", func(cf cloudflarw.Cloudflare) error { return cf.DeleteTunnel(context.Background(), "a1", "t1") }, "/accounts/a1/cfd_tunnel/t1",
		},
		{
			"GetTunnelConfig", func(cf cloudflarw.Cloudflare) error { _, err := cf.GetTunnelConfig(context.Background(), "a1", "t1"); return err }, "/accounts/a1/cfd_tunnel/t1/configurations",
		},
		{
			"GetTunnelToken", func(cf cloudflarw.Cloudflare) error { _, err := cf.GetTunnelToken(context.Background(), "a1", "t1"); return err }, "/accounts/a1/cfd_tunnel/t1/token",
		},
		{
			"ListAccessApps", func(cf cloudflarw.Cloudflare) error { _, _, err := cf.ListAccessApps(context.Background(), "a1"); return err }, "/accounts/a1/access/apps",
		},
		{
			"GetAccessApp", func(cf cloudflarw.Cloudflare) error { _, err := cf.GetAccessApp(context.Background(), "a1", "ap1"); return err }, "/accounts/a1/access/apps/ap1",
		},
		{
			"DeleteAccessApp", func(cf cloudflarw.Cloudflare) error { return cf.DeleteAccessApp(context.Background(), "a1", "ap1") }, "/accounts/a1/access/apps/ap1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				writeJSON(t, w, envelope([]any{}, true))
			}))
			defer ts.Close()

			cf, _ := New(&Config{APIToken: "test-token", BaseURL: ts.URL, AccountID: "a1"})
			_ = tt.call(cf)
			if capturedPath != tt.path {
				t.Errorf("expected path %s, got %s", tt.path, capturedPath)
			}
		})
	}
}
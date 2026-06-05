package apiv1w

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AndreeJait/go-utility/v2/tuyaw"
)

// --- Interface compliance ---

func TestInterfaceCompliance(t *testing.T) {
	var _ tuyaw.Tuya = (*tuyaClient)(nil)
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

func TestNew_EmptyAccessID(t *testing.T) {
	_, err := New(&Config{AccessID: "", AccessKey: "secret"})
	if err == nil {
		t.Fatal("expected error for empty access id")
	}
	if !strings.Contains(err.Error(), "access id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_EmptyAccessKey(t *testing.T) {
	_, err := New(&Config{AccessID: "client123", AccessKey: ""})
	if err == nil {
		t.Fatal("expected error for empty access key")
	}
	if !strings.Contains(err.Error(), "access key is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_ValidConfig(t *testing.T) {
	cf, err := New(&Config{AccessID: "client123", AccessKey: "secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cf == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNew_DefaultBaseURL(t *testing.T) {
	client, err := New(&Config{AccessID: "client123", AccessKey: "secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := client.(*tuyaClient)
	if c.baseURL != defaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", defaultBaseURL, c.baseURL)
	}
	if c.baseURL != "https://openapi-sg.iotbing.com" {
		t.Fatalf("expected Singapore base URL, got %q", c.baseURL)
	}
}

func TestNew_CustomBaseURL(t *testing.T) {
	client, err := New(&Config{AccessID: "client123", AccessKey: "secret", BaseURL: "https://custom.api.url"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := client.(*tuyaClient)
	if c.baseURL != "https://custom.api.url" {
		t.Fatalf("expected custom base URL, got %q", c.baseURL)
	}
}

func TestNew_RegionOverride(t *testing.T) {
	tests := []struct {
		region   string
		expected string
	}{
		{"us", RegionUS},
		{"cn", RegionCN},
		{"eu", RegionEU},
		{"in", RegionIN},
		{"sg", RegionSG},
		{"US", RegionUS},
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			client, err := New(&Config{AccessID: "client123", AccessKey: "secret", Region: tt.region})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			c := client.(*tuyaClient)
			if c.baseURL != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, c.baseURL)
			}
		})
	}
}

func TestNew_InvalidRegion(t *testing.T) {
	_, err := New(&Config{AccessID: "client123", AccessKey: "secret", Region: "xx"})
	if err == nil {
		t.Fatal("expected error for invalid region")
	}
}

func TestNew_CustomHTTPClient(t *testing.T) {
	customClient := &http.Client{}
	cf, err := New(&Config{AccessID: "client123", AccessKey: "secret", HTTPClient: customClient})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := cf.(*tuyaClient)
	if c.client != customClient {
		t.Fatal("expected custom HTTP client to be used")
	}
}

// --- Signing tests ---

func TestSHA256Hex_EmptyBody(t *testing.T) {
	result := sha256Hex("")
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if result != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestSHA256Hex_NonEmptyBody(t *testing.T) {
	result := sha256Hex("hello")
	if len(result) != 64 {
		t.Fatalf("expected 64-char hex string, got %d chars", len(result))
	}
}

func TestSign_TokenRequest(t *testing.T) {
	client := &tuyaClient{
		accessID:  "1KAD46OrT9HafiKdsXeg",
		accessKey: "4OHBOnWOqaEC1mWXOpVL3yV50s0qGSRC",
	}

	// Token request: sign does NOT include access_token
	sign := client.sign("GET", "/v1.0/token?grant_type=1", "", 1588925778000, "5138cc3a9033d69856923fd07b491173", "")
	// Verify sign is uppercase hex
	if sign != strings.ToUpper(sign) {
		t.Fatalf("sign should be uppercase, got %s", sign)
	}
	if len(sign) != 64 {
		t.Fatalf("sign should be 64 chars (SHA256 hex), got %d", len(sign))
	}
}

func TestSign_BusinessRequest(t *testing.T) {
	client := &tuyaClient{
		accessID:  "1KAD46OrT9HafiKdsXeg",
		accessKey: "4OHBOnWOqaEC1mWXOpVL3yV50s0qGSRC",
	}

	// Business request: sign INCLUDES access_token
	sign := client.sign("GET", "/v1.0/devices/test-device-id/status", "", 1588925778000, "5138cc3a9033d69856923fd07b491173", "3f4eda2bdec17232f67c0b188af3eec1")
	if sign != strings.ToUpper(sign) {
		t.Fatalf("sign should be uppercase, got %s", sign)
	}
	if len(sign) != 64 {
		t.Fatalf("sign should be 64 chars (SHA256 hex), got %d", len(sign))
	}
}

// --- Helper to create test server and client ---

func newTestClient(handler http.HandlerFunc) (tuyaw.Tuya, *httptest.Server) {
	ts := httptest.NewServer(handler)
	cf, _ := New(&Config{
		AccessID:  "test-access-id",
		AccessKey: "test-access-key",
		BaseURL:   ts.URL,
	})
	return cf, ts
}

func writeTuyaJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	w.Write(data)
}

func tuyaSuccess(result any) map[string]any {
	return map[string]any{
		"success": true,
		"result":  result,
	}
}

func tuyaError(code int, msg string) map[string]any {
	return map[string]any{
		"success": false,
		"code":    code,
		"msg":     msg,
	}
}

// --- Token tests ---

func TestGetToken_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/token" {
			t.Fatalf("expected /v1.0/token, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("grant_type") != "1" {
			t.Fatalf("expected grant_type=1, got %s", r.URL.Query().Get("grant_type"))
		}
		writeTuyaJSON(t, w, tuyaSuccess(map[string]any{
			"access_token":  "test-access-token",
			"refresh_token": "test-refresh-token",
			"expire_time":   7200,
			"uid":           "test-uid",
		}))
	})
	defer ts.Close()

	token, err := cf.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "test-access-token" {
		t.Fatalf("expected test-access-token, got %s", token.AccessToken)
	}
	if token.RefreshToken != "test-refresh-token" {
		t.Fatalf("expected test-refresh-token, got %s", token.RefreshToken)
	}
	if token.ExpireTime != 7200 {
		t.Fatalf("expected 7200, got %d", token.ExpireTime)
	}
}

func TestRefreshToken_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeTuyaJSON(t, w, tuyaSuccess(map[string]any{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"expire_time":   7200,
			"uid":           "test-uid",
		}))
	})
	defer ts.Close()

	token, err := cf.RefreshToken(context.Background(), "old-refresh-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "new-access-token" {
		t.Fatalf("expected new-access-token, got %s", token.AccessToken)
	}
}

func TestGetToken_APIError(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeTuyaJSON(t, w, tuyaError(1001, "token expired"))
	})
	defer ts.Close()

	_, err := cf.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "1001") || !strings.Contains(err.Error(), "token expired") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Device Management tests ---

func TestGetDevice_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/devices/device-123" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeTuyaJSON(t, w, tuyaSuccess(map[string]any{
			"id":     "device-123",
			"name":   "Living Room Light",
			"category": "dj",
			"online": true,
		}))
	})
	defer ts.Close()

	// Set a token so ensureToken doesn't try to get one
	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	device, err := cf.GetDevice(context.Background(), "device-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if device.ID != "device-123" {
		t.Fatalf("expected device-123, got %s", device.ID)
	}
	if device.Name != "Living Room Light" {
		t.Fatalf("expected Living Room Light, got %s", device.Name)
	}
}

func TestListDevices_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeTuyaJSON(t, w, tuyaSuccess([]map[string]any{
			{"id": "device-1", "name": "Light 1", "category": "dj", "online": true},
			{"id": "device-2", "name": "Light 2", "category": "dj", "online": false},
		}))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	devices, err := cf.ListDevices(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
}

func TestListDevices_WithUID(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1.0/users/uid-123/devices") {
			t.Fatalf("expected /v1.0/users/uid-123/devices, got %s", r.URL.Path)
		}
		writeTuyaJSON(t, w, tuyaSuccess([]map[string]any{
			{"id": "device-1", "name": "Light 1", "category": "dj"},
		}))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	devices, err := cf.ListDevices(context.Background(), &tuyaw.ListDevicesOptions{UID: "uid-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
}

func TestUpdateDeviceName_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/v1.0/devices/device-123" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeTuyaJSON(t, w, tuyaSuccess(true))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	err := cf.UpdateDeviceName(context.Background(), "device-123", "New Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteDevice_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		writeTuyaJSON(t, w, tuyaSuccess(true))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	err := cf.DeleteDevice(context.Background(), "device-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetDeviceSpecifications_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeTuyaJSON(t, w, tuyaSuccess(map[string]any{
			"functions": []map[string]any{
				{"code": "switch_led", "type": "Boolean", "values": "{}", "name": "Switch"},
			},
			"status": []map[string]any{
				{"code": "switch_led", "type": "Boolean", "value": false, "name": "Switch"},
			},
		}))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	spec, err := cf.GetDeviceSpecifications(context.Background(), "device-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spec.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(spec.Functions))
	}
	if spec.Functions[0].Code != "switch_led" {
		t.Fatalf("expected switch_led, got %s", spec.Functions[0].Code)
	}
}

// --- Device Control tests ---

func TestSendCommands_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1.0/devices/device-123/commands" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		commands, ok := req["commands"].([]any)
		if !ok || len(commands) != 1 {
			t.Fatalf("expected 1 command, got %v", req["commands"])
		}
		writeTuyaJSON(t, w, tuyaSuccess(true))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	err := cf.SendCommands(context.Background(), "device-123", []tuyaw.DeviceCommand{
		{Code: "switch_led", Value: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetDeviceStatus_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeTuyaJSON(t, w, tuyaSuccess([]map[string]any{
			{"code": "switch_led", "type": "Boolean", "value": true},
			{"code": "bright", "type": "Integer", "value": 30},
		}))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	status, err := cf.GetDeviceStatus(context.Background(), "device-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(status) != 2 {
		t.Fatalf("expected 2 status points, got %d", len(status))
	}
	if status[0].Code != "switch_led" {
		t.Fatalf("expected switch_led, got %s", status[0].Code)
	}
}

func TestGetDeviceFunctions_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeTuyaJSON(t, w, tuyaSuccess(map[string]any{
			"category": "dj",
			"functions": []map[string]any{
				{"code": "switch_led", "type": "Boolean", "values": "{}", "name": "Switch"},
				{"code": "bright", "type": "Integer", "values": "{\"min\":10,\"max\":1000,\"scale\":0,\"step\":1}", "name": "Brightness"},
			},
		}))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	result, err := cf.GetDeviceFunctions(context.Background(), "device-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Category != "dj" {
		t.Fatalf("expected category dj, got %s", result.Category)
	}
	if len(result.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(result.Functions))
	}
	if result.Functions[0].Code != "switch_led" {
		t.Fatalf("expected switch_led, got %s", result.Functions[0].Code)
	}
}

func TestGetCategoryFunctions_Success(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/functions/dj" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeTuyaJSON(t, w, tuyaSuccess(map[string]any{
			"category": "dj",
			"functions": []map[string]any{
				{"code": "switch_led", "type": "Boolean", "values": "{}", "name": "Switch"},
			},
		}))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	result, err := cf.GetCategoryFunctions(context.Background(), "dj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(result.Functions))
	}
}

// --- Error path tests ---

func TestDoRequest_APIError(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		writeTuyaJSON(t, w, tuyaError(1106, "invalid permission"))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	_, err := cf.GetDevice(context.Background(), "device-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "1106") || !strings.Contains(err.Error(), "invalid permission") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoRequest_InvalidJSON(t *testing.T) {
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	_, err := cf.GetDevice(context.Background(), "device-123")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decoding response") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Token auto-refresh test ---

func TestTokenAutoRefresh(t *testing.T) {
	tokenCount := 0
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1.0/token") {
			tokenCount++
			writeTuyaJSON(t, w, tuyaSuccess(map[string]any{
				"access_token":  "token-" + strconv.Itoa(tokenCount),
				"refresh_token": "refresh-" + strconv.Itoa(tokenCount),
				"expire_time":   7200,
				"uid":           "test-uid",
			}))
			return
		}
		// Business request
		writeTuyaJSON(t, w, tuyaSuccess(map[string]any{
			"id":     "device-1",
			"name":   "Test",
			"online": true,
		}))
	})
	defer ts.Close()

	// Set an expired token to trigger refresh
	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{
		AccessToken:  "expired-token",
		RefreshToken: "expired-refresh",
		ExpireTime:   1, // 1 second
	})
	// Manually expire it
	c.mu.Lock()
	c.expireAt = time.Now().Add(-1 * time.Hour)
	c.mu.Unlock()

	// This should trigger a token refresh
	_, err := cf.GetDevice(context.Background(), "device-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tokenCount == 0 {
		t.Fatal("expected token request to be made")
	}
}

// --- Signing headers test ---

func TestRequestHeaders_SentOnEveryRequest(t *testing.T) {
	var capturedHeaders = make(map[string]string)
	cf, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders["client_id"] = r.Header.Get("client_id")
		capturedHeaders["sign"] = r.Header.Get("sign")
		capturedHeaders["sign_method"] = r.Header.Get("sign_method")
		capturedHeaders["t"] = r.Header.Get("t")
		capturedHeaders["nonce"] = r.Header.Get("nonce")
		writeTuyaJSON(t, w, tuyaSuccess([]any{}))
	})
	defer ts.Close()

	c := cf.(*tuyaClient)
	c.setToken(&tuyaw.Token{AccessToken: "test-token", RefreshToken: "test-refresh", ExpireTime: 7200})

	_, _ = cf.ListDevices(context.Background(), nil)

	if capturedHeaders["client_id"] != "test-access-id" {
		t.Fatalf("expected client_id 'test-access-id', got %q", capturedHeaders["client_id"])
	}
	if capturedHeaders["sign_method"] != "HMAC-SHA256" {
		t.Fatalf("expected sign_method HMAC-SHA256, got %q", capturedHeaders["sign_method"])
	}
	if capturedHeaders["sign"] == "" {
		t.Fatal("expected non-empty sign header")
	}
	if capturedHeaders["t"] == "" {
		t.Fatal("expected non-empty t header")
	}
	if capturedHeaders["nonce"] == "" {
		t.Fatal("expected non-empty nonce header")
	}
}
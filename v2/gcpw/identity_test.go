package gcpw

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewIdentityTokenProvider_NilConfig(t *testing.T) {
	_, err := NewIdentityTokenProvider(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "gcpw: identity config is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewIdentityTokenProvider_EmptyAudience(t *testing.T) {
	_, err := NewIdentityTokenProvider(context.Background(), &IdentityConfig{})
	if err == nil {
		t.Fatal("expected error for empty audience")
	}
	if !strings.Contains(err.Error(), "gcpw: audience is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewIdentityTokenProvider_InvalidCredentials(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/nonexistent/path/to/credentials.json")

	_, err := NewIdentityTokenProvider(context.Background(), &IdentityConfig{
		Audience: "https://example.run.app",
	})
	if err == nil {
		t.Fatal("expected error for invalid credentials path")
	}
	if !strings.Contains(err.Error(), "gcpw: failed to create identity token credentials") {
		t.Errorf("unexpected error: %v", err)
	}
}

type mockTokenProvider struct {
	token *Token
	err   error
}

func (m *mockTokenProvider) Token(_ context.Context) (*Token, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.token, nil
}

func (m *mockTokenProvider) Close() error { return nil }

func TestAuthenticatedHTTPClient_SetsAuthHeader(t *testing.T) {
	var capturedReq *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tp := &mockTokenProvider{
		token: &Token{Value: "test-id-token", Type: "Bearer"},
	}

	client := AuthenticatedHTTPClient(tp, nil)
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if capturedReq == nil {
		t.Fatal("request was not captured")
	}

	auth := capturedReq.Header.Get("Authorization")
	if auth != "Bearer test-id-token" {
		t.Errorf("Authorization header = %q, want %q", auth, "Bearer test-id-token")
	}
}

func TestAuthenticatedHTTPClient_ClonesRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tp := &mockTokenProvider{
		token: &Token{Value: "test-id-token", Type: "Bearer"},
	}

	client := AuthenticatedHTTPClient(tp, nil)

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	req.Header.Set("X-Custom", "original")
	originalAuth := req.Header.Get("Authorization")

	_, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Header.Get("Authorization") != originalAuth {
		t.Error("original request was modified by the transport")
	}
	if req.Header.Get("X-Custom") != "original" {
		t.Error("original request custom header was modified")
	}
}

func TestAuthenticatedHTTPClient_TokenError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tp := &mockTokenProvider{
		err: errors.New("token service unavailable"),
	}

	client := AuthenticatedHTTPClient(tp, nil)
	_, err := client.Get(ts.URL)
	if err == nil {
		t.Fatal("expected error when token provider fails")
	}
	if !strings.Contains(err.Error(), "gcpw: failed to get token for request") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAuthenticatedHTTPClient_CustomTokenType(t *testing.T) {
	var capturedReq *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tp := &mockTokenProvider{
		token: &Token{Value: "test-token", Type: "MAC"},
	}

	client := AuthenticatedHTTPClient(tp, nil)
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	auth := capturedReq.Header.Get("Authorization")
	if auth != "MAC test-token" {
		t.Errorf("Authorization header = %q, want %q", auth, "MAC test-token")
	}
}

func TestAuthenticatedHTTPClient_EmptyTypeDefaultsBearer(t *testing.T) {
	var capturedReq *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tp := &mockTokenProvider{
		token: &Token{Value: "test-token", Type: ""},
	}

	client := AuthenticatedHTTPClient(tp, nil)
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	auth := capturedReq.Header.Get("Authorization")
	if auth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", auth, "Bearer test-token")
	}
}

func TestAuthenticatedHTTPClient_NilBaseClient(t *testing.T) {
	tp := &mockTokenProvider{
		token: &Token{Value: "test-token", Type: "Bearer"},
	}

	client := AuthenticatedHTTPClient(tp, nil)
	if client == nil {
		t.Fatal("client is nil")
	}
	if client.Transport == nil {
		t.Fatal("transport is nil")
	}
	if _, ok := client.Transport.(*authTransport); !ok {
		t.Errorf("transport type = %T, want *authTransport", client.Transport)
	}
}

func TestToken_ExpiryField(t *testing.T) {
	now := time.Now()
	tok := &Token{
		Value:  "test-token",
		Type:   "Bearer",
		Expiry: now,
	}
	if tok.Expiry != now {
		t.Errorf("Expiry = %v, want %v", tok.Expiry, now)
	}
}

func TestGCloudTokenProvider_NilConfig(t *testing.T) {
	tp := NewGCloudTokenProvider(nil)
	if tp == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestGCloudTokenProvider_DefaultBin(t *testing.T) {
	provider := NewGCloudTokenProvider(&GCloudConfig{})
	gp := provider.(*gcloudTokenProvider)
	if gp.bin != "gcloud" {
		t.Errorf("default bin = %q, want %q", gp.bin, "gcloud")
	}
}

func TestGCloudTokenProvider_CustomBin(t *testing.T) {
	provider := NewGCloudTokenProvider(&GCloudConfig{GCloudBin: "/usr/local/bin/gcloud"})
	gp := provider.(*gcloudTokenProvider)
	if gp.bin != "/usr/local/bin/gcloud" {
		t.Errorf("custom bin = %q, want %q", gp.bin, "/usr/local/bin/gcloud")
	}
}

func TestGCloudTokenProvider_CommandFails(t *testing.T) {
	tp := NewGCloudTokenProvider(&GCloudConfig{
		GCloudBin: "/nonexistent/gcloud",
	})
	defer tp.Close()

	_, err := tp.Token(context.Background())
	if err == nil {
		t.Fatal("expected error for nonexistent gcloud binary")
	}
	if !strings.Contains(err.Error(), "gcpw:") {
		t.Errorf("error should be prefixed with gcpw:, got: %v", err)
	}
}

func TestGCloudTokenProvider_CachesToken(t *testing.T) {
	expiry := time.Now().Add(1 * time.Hour)
	freshTok := &Token{Value: "cached-token", Type: "Bearer", Expiry: expiry}

	provider := &gcloudTokenProvider{bin: "gcloud", cachedTok: freshTok}

	// Should return cached token without calling gcloud.
	tok, err := provider.Token(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Value != "cached-token" {
		t.Errorf("token = %q, want %q", tok.Value, "cached-token")
	}
}

func TestGCloudTokenProvider_RefreshesExpiredToken(t *testing.T) {
	expiredTok := &Token{Value: "expired-token", Type: "Bearer", Expiry: time.Now().Add(-1 * time.Minute)}

	provider := &gcloudTokenProvider{bin: "/nonexistent/gcloud", cachedTok: expiredTok}

	// Expired token should attempt refresh (which fails because bin doesn't exist).
	_, err := provider.Token(context.Background())
	if err == nil {
		t.Fatal("expected error when refreshing expired token with bad binary")
	}
}

func TestParseJWTExpiry_Valid(t *testing.T) {
	expiry := time.Now().Add(1*time.Hour).Unix()
	claims := map[string]interface{}{"exp": float64(expiry)}
	payload, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	token := "header." + encoded + ".signature"

	got, err := parseJWTExpiry(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Unix() != expiry {
		t.Errorf("expiry = %v, want %v", got.Unix(), expiry)
	}
}

func TestParseJWTExpiry_NoExpClaim(t *testing.T) {
	claims := map[string]interface{}{"sub": "test"}
	payload, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	token := "header." + encoded + ".signature"

	_, err := parseJWTExpiry(token)
	if err == nil {
		t.Fatal("expected error for JWT without exp claim")
	}
}

func TestParseJWTExpiry_InvalidFormat(t *testing.T) {
	_, err := parseJWTExpiry("not-a-jwt")
	if err == nil {
		t.Fatal("expected error for invalid JWT format")
	}
}

func TestParseJWTExpiry_StandardBase64(t *testing.T) {
	expiry := time.Now().Add(1*time.Hour).Unix()
	claims := map[string]interface{}{"exp": float64(expiry)}
	payload, _ := json.Marshal(claims)
	// Use standard base64 (with padding) instead of URL-safe.
	encoded := base64.RawStdEncoding.EncodeToString(payload)
	token := "header." + encoded + ".signature"

	got, err := parseJWTExpiry(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Unix() != expiry {
		t.Errorf("expiry = %v, want %v", got.Unix(), expiry)
	}
}
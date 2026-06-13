package tsnetw

import (
	"context"
	"net"
	"net/http"
	"testing"

	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tsnet"
)

func TestNew_MissingConfig(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNew_MissingHostname(t *testing.T) {
	_, err := New(&Config{})
	if err == nil {
		t.Fatal("expected error for empty hostname")
	}
}

func TestNew_WithHostname(t *testing.T) {
	vpn, err := New(&Config{Hostname: "test-node"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vpn == nil {
		t.Fatal("expected non-nil VPN")
	}
}

func TestNew_WithProvidedServer(t *testing.T) {
	srv := &tsnet.Server{Hostname: "provided"}
	vpn, err := New(&Config{Server: srv})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vpn == nil {
		t.Fatal("expected non-nil VPN")
	}
}

func TestDefaultStateDir(t *testing.T) {
	dir := defaultStateDir("my-host")
	if dir == "" {
		t.Fatal("expected non-empty state dir")
	}
	if len(dir) < len("my-host") || dir[len(dir)-len("my-host"):] != "my-host" {
		t.Fatalf("expected state dir ending in my-host, got %q", dir)
	}
}

func TestFromIPNStatus_Nil(t *testing.T) {
	if fromIPNStatus(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestFromIPNStatus_Empty(t *testing.T) {
	out := fromIPNStatus(&ipnstate.Status{})
	if out == nil {
		t.Fatal("expected non-nil output")
	}
	if out.BackendState != "" {
		t.Fatalf("unexpected backend state: %q", out.BackendState)
	}
}

// fakeVPN is a minimal implementation used to verify interface shape.
type fakeVPN struct{}

func (f *fakeVPN) Start(ctx context.Context) (*Status, error)           { return nil, nil }
func (f *fakeVPN) Close() error                                         { return nil }
func (f *fakeVPN) HTTPClient() *http.Client                             { return nil }
func (f *fakeVPN) Listen(network, addr string) (net.Listener, error)    { return nil, nil }
func (f *fakeVPN) ListenTLS(network, addr string) (net.Listener, error) { return nil, nil }
func (f *fakeVPN) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	return nil, nil
}
func (f *fakeVPN) WhoIs(ctx context.Context, remoteAddr string) (*WhoIsResult, error) {
	return nil, nil
}
func (f *fakeVPN) Status(ctx context.Context) (*Status, error) { return nil, nil }

func TestVPNInterface_Compliance(t *testing.T) {
	var _ VPN = (*fakeVPN)(nil)
}

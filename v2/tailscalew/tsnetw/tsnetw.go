// Package tsnetw wraps tailscale.com/tsnet to embed a Tailscale node directly
// into a Go program, joining a tailnet, dialing/listening, and identifying peers.
package tsnetw

import (
	"context"
	"net"
	"net/http"
)

// VPN is an embedded Tailscale node that can join a tailnet and provide
// network services over it.
type VPN interface {
	// Start connects the node to the tailnet. It is idempotent; subsequent
	// calls return the same status as the first successful call.
	Start(ctx context.Context) (*Status, error)

	// Close shuts down the embedded node and releases all resources.
	Close() error

	// HTTPClient returns an *http.Client that sends requests over the tailnet.
	HTTPClient() *http.Client

	// Listen opens a listener on the tailnet. Use ":port" to listen on all
	// tailnet IPs, or omit the port to use Tailscale's default.
	Listen(network, addr string) (net.Listener, error)

	// ListenTLS opens a TLS listener using Tailscale's automatic certificates.
	ListenTLS(network, addr string) (net.Listener, error)

	// Dial connects to a service on the tailnet.
	Dial(ctx context.Context, network, addr string) (net.Conn, error)

	// WhoIs returns Tailscale identity information for a remote address.
	WhoIs(ctx context.Context, remoteAddr string) (*WhoIsResult, error)

	// Status returns the current tailnet status of this node.
	Status(ctx context.Context) (*Status, error)
}

// Status describes the tailnet connection state of a VPN node.
type Status struct {
	BackendState string
	Self         Node
	TailscaleIPs []string
}

// Node is a simplified view of a tailnet node.
type Node struct {
	ID       string
	Name     string
	Hostname string
	User     string
	Tags     []string
}

// WhoIsResult is a simplified view of a remote peer's identity.
type WhoIsResult struct {
	Node     Node
	User     string
	IsTagged bool
}

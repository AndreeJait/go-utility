package gcpw

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Token represents an authentication token obtained from a GCP auth source.
type Token struct {
	Value  string
	Type   string
	Expiry time.Time
}

// TokenProvider defines the contract for obtaining GCP authentication tokens.
// Implementations must be safe for concurrent use.
type TokenProvider interface {
	Token(ctx context.Context) (*Token, error)
	Close() error
}

// AuthenticatedHTTPClient returns an *http.Client that automatically attaches
// the Authorization header to every outgoing request using the given TokenProvider.
// If base is nil, http.DefaultClient is used as the starting point.
func AuthenticatedHTTPClient(tp TokenProvider, base *http.Client) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}

	baseTransport := base.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}

	return &http.Client{
		Transport: &authTransport{
			base: baseTransport,
			tp:   tp,
		},
		Timeout: base.Timeout,
	}
}

type authTransport struct {
	base http.RoundTripper
	tp   TokenProvider
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.tp.Token(req.Context())
	if err != nil {
		return nil, fmt.Errorf("gcpw: failed to get token for request: %w", err)
	}

	req2 := req.Clone(req.Context())

	tokenType := token.Type
	if tokenType == "" {
		tokenType = "Bearer"
	}
	req2.Header.Set("Authorization", tokenType+" "+token.Value)

	return t.base.RoundTrip(req2)
}
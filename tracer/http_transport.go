package tracer

import (
	"context"
	"net/http"
)

// Inject trace headers into outgoing requests from the current ctx.
func Inject(ctx context.Context, h http.Header) {
	if tid := TraceID(ctx); tid != "" {
		h.Set("X-Request-ID", tid)
	}
}

// HTTPTransport wraps RoundTripper to auto-inject headers.
type HTTPTransport struct{ Base http.RoundTripper }

func (t HTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := t.Base
	if rt == nil {
		rt = http.DefaultTransport
	}
	req2 := req.Clone(req.Context())
	Inject(req2.Context(), req2.Header)
	return rt.RoundTrip(req2)
}

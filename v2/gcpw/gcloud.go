package gcpw

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
)

// GCloudConfig holds configuration for the gcloud CLI-based token provider.
type GCloudConfig struct {
	// GCloudBin is the path to the gcloud binary. Defaults to "gcloud" if empty.
	GCloudBin string

	// DebugMode enables verbose logging of token operations.
	DebugMode bool
}

type gcloudTokenProvider struct {
	bin   string
	debug bool

	mu        sync.Mutex
	cachedTok *Token
}

// NewGCloudTokenProvider creates a TokenProvider that obtains identity tokens
// by executing "gcloud auth print-identity-token". Tokens are cached and
// automatically refreshed 5 minutes before expiry.
//
// This is a convenience for local development where gcloud CLI is already
// authenticated. For production workloads, use NewIdentityTokenProvider instead.
func NewGCloudTokenProvider(cfg *GCloudConfig) TokenProvider {
	if cfg == nil {
		cfg = &GCloudConfig{}
	}
	bin := cfg.GCloudBin
	if bin == "" {
		bin = "gcloud"
	}
	return &gcloudTokenProvider{
		bin:   bin,
		debug: cfg.DebugMode,
	}
}

func (p *gcloudTokenProvider) Token(ctx context.Context) (*Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Return cached token if still valid (with 5-minute refresh window).
	if p.cachedTok != nil && time.Now().Before(p.cachedTok.Expiry.Add(-5*time.Minute)) {
		if p.debug {
			logw.CtxInfof(ctx, "gcpw: returning cached gcloud identity token (expires=%s)", p.cachedTok.Expiry)
		}
		return p.cachedTok, nil
	}

	if p.debug {
		logw.CtxInfof(ctx, "gcpw: fetching new gcloud identity token via %s", p.bin)
	}

	cmd := exec.CommandContext(ctx, p.bin, "auth", "print-identity-token")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gcpw: gcloud auth print-identity-token failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("gcpw: gcloud auth print-identity-token: %w", err)
	}

	raw := strings.TrimSpace(string(out))

	expiry, err := parseJWTExpiry(raw)
	if err != nil {
		if p.debug {
			logw.CtxInfof(ctx, "gcpw: could not parse token expiry, using 55-minute default: %v", err)
		}
		// Tokens from gcloud typically expire in 1 hour; use 55 minutes as safe default.
		expiry = time.Now().Add(55 * time.Minute)
	}

	tok := &Token{
		Value:  raw,
		Type:   "Bearer",
		Expiry: expiry,
	}

	p.cachedTok = tok

	if p.debug {
		logw.CtxInfof(ctx, "gcpw: gcloud identity token obtained (expires=%s)", tok.Expiry)
	}

	return tok, nil
}

func (p *gcloudTokenProvider) Close() error {
	if p.debug {
		logw.Infof("gcpw: gcloud token provider closed")
	}
	return nil
}

var _ TokenProvider = (*gcloudTokenProvider)(nil)

// jwtClaims represents the payload of a JWT token.
type jwtClaims struct {
	Exp int64 `json:"exp"`
}

// parseJWTExpiry extracts the expiry time from a JWT without verifying the signature.
func parseJWTExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("gcpw: invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	payload, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		// Try URL-safe encoding.
		payload, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("gcpw: failed to decode JWT payload: %w", err)
		}
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("gcpw: failed to parse JWT claims: %w", err)
	}

	if claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("gcpw: JWT has no exp claim")
	}

	return time.Unix(claims.Exp, 0), nil
}
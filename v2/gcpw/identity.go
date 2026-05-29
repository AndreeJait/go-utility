package gcpw

import (
	"context"
	"fmt"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials/idtoken"

	"github.com/AndreeJait/go-utility/v2/logw"
)

// IdentityConfig holds the configuration for the GCP identity token provider.
type IdentityConfig struct {
	// Audience is the target service URL for the identity token (e.g., "https://my-service-abc123.run.app").
	// Required.
	Audience string

	// CredentialsFile is the path to a service account JSON key file.
	// If empty, Application Default Credentials are used.
	CredentialsFile string

	// CredentialsJSON is the raw bytes of a service account JSON key file.
	// If set, CredentialsFile must be empty.
	CredentialsJSON []byte

	// CustomClaims are extra JWT claims to include in the identity token.
	CustomClaims map[string]interface{}

	// DebugMode enables verbose logging of token operations.
	DebugMode bool
}

type identityTokenProvider struct {
	creds    *auth.Credentials
	audience string
	debug    bool
}

// NewIdentityTokenProvider creates a TokenProvider that obtains OIDC identity tokens
// from GCP Application Default Credentials for the given audience.
//
// On Compute Engine/Cloud Run, it fetches identity tokens from the metadata server.
// With a service account JSON file, it self-signs a JWT and exchanges it via the
// OAuth2 token endpoint. Tokens are automatically cached and refreshed before expiry.
func NewIdentityTokenProvider(ctx context.Context, cfg *IdentityConfig) (TokenProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("gcpw: identity config is required")
	}
	if cfg.Audience == "" {
		return nil, fmt.Errorf("gcpw: audience is required for identity tokens")
	}

	opts := &idtoken.Options{
		Audience:     cfg.Audience,
		CustomClaims: cfg.CustomClaims,
	}

	if cfg.CredentialsFile != "" {
		opts.CredentialsFile = cfg.CredentialsFile
	}
	if len(cfg.CredentialsJSON) > 0 {
		opts.CredentialsJSON = cfg.CredentialsJSON
	}

	creds, err := idtoken.NewCredentials(opts)
	if err != nil {
		return nil, fmt.Errorf("gcpw: failed to create identity token credentials: %w", err)
	}

	if cfg.DebugMode {
		logw.Infof("gcpw: identity token provider initialized for audience %q", cfg.Audience)
	}

	return &identityTokenProvider{
		creds:    creds,
		audience: cfg.Audience,
		debug:    cfg.DebugMode,
	}, nil
}

func (p *identityTokenProvider) Token(ctx context.Context) (*Token, error) {
	sdkToken, err := p.creds.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcpw: failed to obtain identity token: %w", err)
	}

	if p.debug {
		logw.CtxInfof(ctx, "gcpw: identity token obtained (expires=%s)", sdkToken.Expiry)
	}

	return &Token{
		Value:  sdkToken.Value,
		Type:   sdkToken.Type,
		Expiry: sdkToken.Expiry,
	}, nil
}

func (p *identityTokenProvider) Close() error {
	if p.debug {
		logw.Infof("gcpw: identity token provider closed for audience %q", p.audience)
	}
	return nil
}

var _ TokenProvider = (*identityTokenProvider)(nil)
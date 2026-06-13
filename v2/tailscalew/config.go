package tailscalew

import "net/http"

// Config holds authentication and client settings for the Tailscale API.
type Config struct {
	// Tailnet is the tailnet name, e.g. "example.com" or "example.github".
	// If empty, the API client uses the default tailnet for the credential.
	Tailnet string

	// APIKey is a Tailscale API key. Mutually exclusive with OAuth fields.
	APIKey string

	// OAuthClientID and OAuthClientSecret enable OAuth authentication.
	// OAuthScopes defaults to ["all:read"] when empty.
	OAuthClientID     string
	OAuthClientSecret string
	OAuthScopes       []string

	// HTTPClient is an optional custom HTTP client. If nil, the official client
	// creates one with a 1-minute timeout.
	HTTPClient *http.Client

	// DebugMode enables verbose logging of API operations via logw.
	DebugMode bool
}

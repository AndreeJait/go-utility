package tsnetw

import (
	"fmt"

	"tailscale.com/tsnet"
)

// Config holds settings for an embedded Tailscale node.
type Config struct {
	// Hostname is the machine name advertised on the tailnet. Required.
	Hostname string

	// AuthKey is a Tailscale auth key (preferred for unattended agents).
	// If empty, the TS_AUTHKEY environment variable is consulted.
	AuthKey string

	// OAuthClientID and OAuthClientSecret enable OAuth-based auth key generation.
	// If empty, the TS_CLIENT_ID and TS_CLIENT_SECRET environment variables are consulted.
	OAuthClientID     string
	OAuthClientSecret string

	// IDToken and Audience enable workload identity federation.
	// If empty, the TS_ID_TOKEN and TS_AUDIENCE environment variables are consulted.
	IDToken  string
	Audience string

	// Dir is the directory for persistent node state (keys, prefs).
	// If empty, a directory under os.UserConfigDir is chosen automatically.
	Dir string

	// Ephemeral, if true, removes the node from the tailnet when it shuts down.
	Ephemeral bool

	// ControlURL overrides the Tailscale coordination server. Optional.
	ControlURL string

	// AdvertiseTags requests tags for ACL enforcement on this node.
	// The control server must allow the node to adopt these tags.
	AdvertiseTags []string

	// Port is the UDP port for WireGuard and peer-to-peer traffic.
	// Leave at zero unless you know what you are doing.
	Port uint16

	// Logf receives verbose backend logs. If nil, logs are discarded.
	Logf func(format string, args ...any)

	// UserLogf receives user-visible logs (auth URL, status updates).
	// If nil, logs are discarded.
	UserLogf func(format string, args ...any)

	// Server, if provided, is used instead of creating a new tsnet.Server.
	// Useful for tests and advanced customization.
	Server *tsnet.Server
}

func (c *Config) validate() error {
	if c.Server != nil {
		return nil
	}
	if c.Hostname == "" {
		return fmtError("hostname is required")
	}
	if c.AuthKey == "" && c.OAuthClientID == "" && c.OAuthClientSecret == "" &&
		c.IDToken == "" {
		// tsnet will fall back to environment variables, so this is a warning
		// rather than a hard error.
		return nil
	}
	return nil
}

func fmtError(msg string) error {
	return fmt.Errorf("tsnetw: %s", msg)
}

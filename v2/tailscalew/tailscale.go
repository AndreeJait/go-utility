// Package tailscalew wraps the Tailscale HTTP API client with a slim,
// repository-consistent interface and constructor.
package tailscalew

import (
	"context"
	"time"
)

// Tailscale provides a subset of the Tailscale HTTP API commonly used by
// backend services: device and authentication key management.
type Tailscale interface {
	// ListDevices returns devices in the configured tailnet.
	ListDevices(ctx context.Context) ([]Device, error)

	// GetDevice returns a single device by its node ID (preferred) or legacy ID.
	GetDevice(ctx context.Context, deviceID string) (*Device, error)

	// DeleteDevice removes a device from the tailnet.
	DeleteDevice(ctx context.Context, deviceID string) error

	// SetDeviceTags replaces the tags on a device.
	SetDeviceTags(ctx context.Context, deviceID string, tags []string) error

	// AuthorizeDevice approves a pending device to join the tailnet.
	// Required if the tailnet has device approval enabled.
	AuthorizeDevice(ctx context.Context, deviceID string) error

	// UpdateDeviceKeyExpiry enables or disables node key expiration.
	// Set disabled=true for unattended background agents so they do not drop
	// offline when their node key expires.
	UpdateDeviceKeyExpiry(ctx context.Context, deviceID string, disabled bool) error

	// ExpireDevice forces the immediate expiration of a device's node key,
	// instantly disconnecting it from the tailnet without deleting device history.
	ExpireDevice(ctx context.Context, deviceID string) error

	// SetDeviceName updates the machine name shown in the Tailscale admin console.
	SetDeviceName(ctx context.Context, deviceID string, name string) error

	// ListAuthKeys returns active auth keys in the tailnet.
	ListAuthKeys(ctx context.Context) ([]AuthKey, error)

	// CreateAuthKey creates a new auth key with the requested capabilities.
	CreateAuthKey(ctx context.Context, req CreateAuthKeyRequest) (*AuthKey, error)

	// DeleteAuthKey deletes an auth key by ID.
	DeleteAuthKey(ctx context.Context, keyID string) error
}

// Device represents a Tailscale device.
type Device struct {
	ID                 string
	NodeID             string
	Name               string
	Hostname           string
	OS                 string
	User               string
	Tags               []string
	Addresses          []string
	TailscaleIPs       []string
	Authorized         bool
	IsEphemeral        bool
	IsExternal         bool
	ConnectedToControl bool
	ClientVersion      string
	Created            time.Time
	Expires            time.Time
	LastSeen           *time.Time
	UpdateAvailable    bool
}

// AuthKey represents a Tailscale authentication key.
type AuthKey struct {
	ID            string
	Key           string
	KeyType       string
	Description   string
	Tags          []string
	Scopes        []string
	Reusable      bool
	Ephemeral     bool
	Preauthorized bool
	Created       time.Time
	Updated       time.Time
	Expires       time.Time
	Invalid       bool
}

// CreateAuthKeyRequest configures a new auth key.
type CreateAuthKeyRequest struct {
	Description   string
	Expiry        time.Duration
	Reusable      bool
	Ephemeral     bool
	Preauthorized bool
	Tags          []string
}

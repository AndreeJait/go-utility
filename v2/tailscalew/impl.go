package tailscalew

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	"tailscale.com/client/tailscale/v2"
)

var _ Tailscale = (*tailscaleManager)(nil)

// deviceClient is the subset of tailscale.DevicesResource used by this package.
type deviceClient interface {
	List(ctx context.Context, opts ...tailscale.ListDevicesOptions) ([]tailscale.Device, error)
	Get(ctx context.Context, deviceID string) (*tailscale.Device, error)
	Delete(ctx context.Context, deviceID string) error
	SetTags(ctx context.Context, deviceID string, tags []string) error
	SetAuthorized(ctx context.Context, deviceID string, authorized bool) error
	SetKey(ctx context.Context, deviceID string, key tailscale.DeviceKey) error
	SetName(ctx context.Context, deviceID, name string) error
}

// keyClient is the subset of tailscale.KeysResource used by this package.
type keyClient interface {
	List(ctx context.Context, all bool) ([]tailscale.Key, error)
	Create(ctx context.Context, ckr tailscale.CreateKeyRequest) (*tailscale.Key, error)
	Delete(ctx context.Context, id string) error
}

type tailscaleManager struct {
	devices deviceClient
	keys    keyClient
	http    *http.Client
	baseURL string
	apiKey  string
	tailnet string
	debug   bool
}

// New creates a Tailscale wrapper from the provided config.
func New(cfg *Config) (Tailscale, error) {
	if cfg == nil {
		return nil, fmt.Errorf("tailscalew: config is required")
	}

	client := &tailscale.Client{
		Tailnet: cfg.Tailnet,
		APIKey:  cfg.APIKey,
		HTTP:    cfg.HTTPClient,
	}

	switch {
	case cfg.OAuthClientID != "" || cfg.OAuthClientSecret != "":
		scopes := cfg.OAuthScopes
		if len(scopes) == 0 {
			scopes = []string{"all:read"}
		}
		client.Auth = &tailscale.OAuth{
			ClientID:     cfg.OAuthClientID,
			ClientSecret: cfg.OAuthClientSecret,
			Scopes:       scopes,
		}
	case cfg.APIKey == "":
		return nil, fmt.Errorf("tailscalew: either APIKey or OAuth credentials are required")
	}

	if cfg.DebugMode {
		logw.Infof("tailscalew: initialized for tailnet %q", cfg.Tailnet)
	}

	baseURL := "https://api.tailscale.com"
	return &tailscaleManager{
		devices: client.Devices(),
		keys:    client.Keys(),
		http:    cfg.HTTPClient,
		baseURL: baseURL,
		apiKey:  cfg.APIKey,
		tailnet: cfg.Tailnet,
		debug:   cfg.DebugMode,
	}, nil
}

func (m *tailscaleManager) ListDevices(ctx context.Context) ([]Device, error) {
	if m.debug {
		logw.CtxInfof(ctx, "tailscalew: listing devices")
	}

	items, err := m.devices.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("tailscalew: list devices: %w", err)
	}

	out := make([]Device, 0, len(items))
	for _, d := range items {
		out = append(out, fromSDKDevice(&d))
	}
	return out, nil
}

func (m *tailscaleManager) GetDevice(ctx context.Context, deviceID string) (*Device, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("tailscalew: deviceID is required")
	}
	if m.debug {
		logw.CtxInfof(ctx, "tailscalew: getting device %q", deviceID)
	}

	d, err := m.devices.Get(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("tailscalew: get device %q: %w", deviceID, err)
	}

	dev := fromSDKDevice(d)
	return &dev, nil
}

func (m *tailscaleManager) DeleteDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return fmt.Errorf("tailscalew: deviceID is required")
	}
	if m.debug {
		logw.CtxInfof(ctx, "tailscalew: deleting device %q", deviceID)
	}

	if err := m.devices.Delete(ctx, deviceID); err != nil {
		return fmt.Errorf("tailscalew: delete device %q: %w", deviceID, err)
	}
	return nil
}

func (m *tailscaleManager) SetDeviceTags(ctx context.Context, deviceID string, tags []string) error {
	if deviceID == "" {
		return fmt.Errorf("tailscalew: deviceID is required")
	}
	if m.debug {
		logw.CtxInfof(ctx, "tailscalew: setting tags on device %q", deviceID)
	}

	if err := m.devices.SetTags(ctx, deviceID, tags); err != nil {
		return fmt.Errorf("tailscalew: set tags on device %q: %w", deviceID, err)
	}
	return nil
}

func (m *tailscaleManager) AuthorizeDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return fmt.Errorf("tailscalew: deviceID is required")
	}
	if m.debug {
		logw.CtxInfof(ctx, "tailscalew: authorizing device %q", deviceID)
	}

	if err := m.devices.SetAuthorized(ctx, deviceID, true); err != nil {
		return fmt.Errorf("tailscalew: authorize device %q: %w", deviceID, err)
	}
	return nil
}

func (m *tailscaleManager) UpdateDeviceKeyExpiry(ctx context.Context, deviceID string, disabled bool) error {
	if deviceID == "" {
		return fmt.Errorf("tailscalew: deviceID is required")
	}
	if m.debug {
		logw.CtxInfof(ctx, "tailscalew: setting key expiry disabled=%v on device %q", disabled, deviceID)
	}

	if err := m.devices.SetKey(ctx, deviceID, tailscale.DeviceKey{KeyExpiryDisabled: disabled}); err != nil {
		return fmt.Errorf("tailscalew: update key expiry on device %q: %w", deviceID, err)
	}
	return nil
}

func (m *tailscaleManager) ExpireDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return fmt.Errorf("tailscalew: deviceID is required")
	}
	if m.debug {
		logw.CtxInfof(ctx, "tailscalew: expiring device %q", deviceID)
	}

	client := m.http
	if client == nil {
		client = http.DefaultClient
	}

	url := fmt.Sprintf("%s/api/v2/device/%s/expire", m.baseURL, deviceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("tailscalew: build expire request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("tailscalew: expire device %q: %w", deviceID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("tailscalew: expire device %q returned status %d", deviceID, resp.StatusCode)
	}
	return nil
}

func (m *tailscaleManager) SetDeviceName(ctx context.Context, deviceID string, name string) error {
	if deviceID == "" {
		return fmt.Errorf("tailscalew: deviceID is required")
	}
	if name == "" {
		return fmt.Errorf("tailscalew: name is required")
	}
	if m.debug {
		logw.CtxInfof(ctx, "tailscalew: setting name on device %q to %q", deviceID, name)
	}

	if err := m.devices.SetName(ctx, deviceID, name); err != nil {
		return fmt.Errorf("tailscalew: set name on device %q: %w", deviceID, err)
	}
	return nil
}

func (m *tailscaleManager) ListAuthKeys(ctx context.Context) ([]AuthKey, error) {
	if m.debug {
		logw.CtxInfof(ctx, "tailscalew: listing auth keys")
	}

	items, err := m.keys.List(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("tailscalew: list auth keys: %w", err)
	}

	out := make([]AuthKey, 0, len(items))
	for _, k := range items {
		out = append(out, fromSDKKey(&k))
	}
	return out, nil
}

func (m *tailscaleManager) CreateAuthKey(ctx context.Context, req CreateAuthKeyRequest) (*AuthKey, error) {
	if m.debug {
		logw.CtxInfof(ctx, "tailscalew: creating auth key")
	}

	ckr := tailscale.CreateKeyRequest{
		Description:   req.Description,
		ExpirySeconds: int64(req.Expiry.Seconds()),
		Capabilities: tailscale.KeyCapabilities{
			Devices: struct {
				Create struct {
					Reusable      bool     `json:"reusable"`
					Ephemeral     bool     `json:"ephemeral"`
					Tags          []string `json:"tags"`
					Preauthorized bool     `json:"preauthorized"`
				} `json:"create"`
			}{
				Create: struct {
					Reusable      bool     `json:"reusable"`
					Ephemeral     bool     `json:"ephemeral"`
					Tags          []string `json:"tags"`
					Preauthorized bool     `json:"preauthorized"`
				}{
					Reusable:      req.Reusable,
					Ephemeral:     req.Ephemeral,
					Tags:          req.Tags,
					Preauthorized: req.Preauthorized,
				},
			},
		},
	}

	k, err := m.keys.Create(ctx, ckr)
	if err != nil {
		return nil, fmt.Errorf("tailscalew: create auth key: %w", err)
	}

	key := fromSDKKey(k)
	return &key, nil
}

func (m *tailscaleManager) DeleteAuthKey(ctx context.Context, keyID string) error {
	if keyID == "" {
		return fmt.Errorf("tailscalew: keyID is required")
	}
	if m.debug {
		logw.CtxInfof(ctx, "tailscalew: deleting auth key %q", keyID)
	}

	if err := m.keys.Delete(ctx, keyID); err != nil {
		return fmt.Errorf("tailscalew: delete auth key %q: %w", keyID, err)
	}
	return nil
}

func fromSDKDevice(d *tailscale.Device) Device {
	out := Device{
		ID:                 d.ID,
		NodeID:             d.NodeID,
		Name:               d.Name,
		Hostname:           d.Hostname,
		OS:                 d.OS,
		User:               d.User,
		Tags:               d.Tags,
		Addresses:          d.Addresses,
		Authorized:         d.Authorized,
		IsEphemeral:        d.IsEphemeral,
		IsExternal:         d.IsExternal,
		ConnectedToControl: d.ConnectedToControl,
		ClientVersion:      d.ClientVersion,
		UpdateAvailable:    d.UpdateAvailable,
	}
	out.TailscaleIPs = filterTailscaleIPs(d.Addresses)
	out.Created = d.Created.Time
	out.Expires = d.Expires.Time
	if d.LastSeen != nil {
		t := d.LastSeen.Time
		out.LastSeen = &t
	}
	return out
}

func fromSDKKey(k *tailscale.Key) AuthKey {
	out := AuthKey{
		ID:            k.ID,
		Key:           k.Key,
		KeyType:       k.KeyType,
		Description:   k.Description,
		Tags:          k.Tags,
		Scopes:        k.Scopes,
		Created:       k.Created,
		Updated:       k.Updated,
		Expires:       k.Expires,
		Invalid:       k.Invalid,
		Reusable:      k.Capabilities.Devices.Create.Reusable,
		Ephemeral:     k.Capabilities.Devices.Create.Ephemeral,
		Preauthorized: k.Capabilities.Devices.Create.Preauthorized,
	}
	return out
}

func filterTailscaleIPs(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		// Tailscale IPs are the 100.64.0.0/10 CGNAT range.
		if len(a) >= 4 && a[:4] == "100." {
			out = append(out, a)
		}
	}
	return out
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

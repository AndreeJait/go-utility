package tailscalew

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"tailscale.com/client/tailscale/v2"
)

// mockDeviceClient implements deviceClient for unit tests.
type mockDeviceClient struct {
	listErr          error
	listResult       []tailscale.Device
	getErr           error
	getResult        *tailscale.Device
	deleteErr        error
	setTagsErr       error
	setAuthorizedErr error
	setKeyErr        error
	setNameErr       error
	lastKey          tailscale.DeviceKey
	lastName         string
}

func (m *mockDeviceClient) List(ctx context.Context, opts ...tailscale.ListDevicesOptions) ([]tailscale.Device, error) {
	return m.listResult, m.listErr
}

func (m *mockDeviceClient) Get(ctx context.Context, deviceID string) (*tailscale.Device, error) {
	return m.getResult, m.getErr
}

func (m *mockDeviceClient) Delete(ctx context.Context, deviceID string) error {
	return m.deleteErr
}

func (m *mockDeviceClient) SetTags(ctx context.Context, deviceID string, tags []string) error {
	return m.setTagsErr
}

func (m *mockDeviceClient) SetAuthorized(ctx context.Context, deviceID string, authorized bool) error {
	return m.setAuthorizedErr
}

func (m *mockDeviceClient) SetKey(ctx context.Context, deviceID string, key tailscale.DeviceKey) error {
	m.lastKey = key
	return m.setKeyErr
}

func (m *mockDeviceClient) SetName(ctx context.Context, deviceID, name string) error {
	m.lastName = name
	return m.setNameErr
}

// mockKeyClient implements keyClient for unit tests.
type mockKeyClient struct {
	listErr      error
	listResult   []tailscale.Key
	createErr    error
	createResult *tailscale.Key
	deleteErr    error
}

func (m *mockKeyClient) List(ctx context.Context, all bool) ([]tailscale.Key, error) {
	return m.listResult, m.listErr
}

func (m *mockKeyClient) Create(ctx context.Context, ckr tailscale.CreateKeyRequest) (*tailscale.Key, error) {
	return m.createResult, m.createErr
}

func (m *mockKeyClient) Delete(ctx context.Context, id string) error {
	return m.deleteErr
}

func newTestManager(devices deviceClient, keys keyClient) *tailscaleManager {
	return &tailscaleManager{
		devices: devices,
		keys:    keys,
		debug:   false,
	}
}

func TestNew_MissingConfig(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNew_MissingCredentials(t *testing.T) {
	_, err := New(&Config{})
	if err == nil {
		t.Fatal("expected error when both APIKey and OAuth are empty")
	}
}

func TestNew_WithAPIKey(t *testing.T) {
	tt, err := New(&Config{
		Tailnet: "example.github",
		APIKey:  "tskey-api-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tt == nil {
		t.Fatal("expected non-nil Tailscale")
	}
}

func TestNew_WithOAuth(t *testing.T) {
	tt, err := New(&Config{
		Tailnet:           "example.github",
		OAuthClientID:     "client-id",
		OAuthClientSecret: "client-secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tt == nil {
		t.Fatal("expected non-nil Tailscale")
	}
}

func TestListDevices_Success(t *testing.T) {
	created := tailscale.Time{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	expires := tailscale.Time{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	lastSeen := &tailscale.Time{Time: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)}

	devices := &mockDeviceClient{
		listResult: []tailscale.Device{
			{
				ID:                 "id1",
				NodeID:             "node1",
				Name:               "machine1",
				Hostname:           "host1",
				OS:                 "linux",
				User:               "alice",
				Tags:               []string{"tag:web"},
				Addresses:          []string{"100.64.0.1", "192.168.1.2"},
				Authorized:         true,
				IsEphemeral:        false,
				IsExternal:         false,
				ConnectedToControl: true,
				ClientVersion:      "1.68",
				Created:            created,
				Expires:            expires,
				LastSeen:           lastSeen,
				UpdateAvailable:    true,
			},
		},
	}

	m := newTestManager(devices, &mockKeyClient{})
	out, err := m.ListDevices(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 device, got %d", len(out))
	}

	d := out[0]
	if d.ID != "id1" || d.NodeID != "node1" || d.Name != "machine1" {
		t.Fatalf("unexpected device fields: %+v", d)
	}
	if len(d.TailscaleIPs) != 1 || d.TailscaleIPs[0] != "100.64.0.1" {
		t.Fatalf("unexpected TailscaleIPs: %v", d.TailscaleIPs)
	}
	if !d.LastSeen.Equal(lastSeen.Time) {
		t.Fatalf("unexpected LastSeen: %v", d.LastSeen)
	}
}

func TestListDevices_Error(t *testing.T) {
	devices := &mockDeviceClient{listErr: errors.New("api down")}
	m := newTestManager(devices, &mockKeyClient{})
	_, err := m.ListDevices(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetDevice_Success(t *testing.T) {
	devices := &mockDeviceClient{
		getResult: &tailscale.Device{
			ID:        "id2",
			NodeID:    "node2",
			Name:      "machine2",
			Hostname:  "host2",
			OS:        "windows",
			User:      "bob",
			Tags:      []string{"tag:db"},
			Addresses: []string{"100.64.0.2"},
		},
	}
	m := newTestManager(devices, &mockKeyClient{})
	d, err := m.GetDevice(context.Background(), "node2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.NodeID != "node2" {
		t.Fatalf("unexpected device: %+v", d)
	}
}

func TestGetDevice_MissingID(t *testing.T) {
	m := newTestManager(&mockDeviceClient{}, &mockKeyClient{})
	_, err := m.GetDevice(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty deviceID")
	}
}

func TestGetDevice_Error(t *testing.T) {
	devices := &mockDeviceClient{getErr: errors.New("not found")}
	m := newTestManager(devices, &mockKeyClient{})
	_, err := m.GetDevice(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteDevice_Success(t *testing.T) {
	devices := &mockDeviceClient{}
	m := newTestManager(devices, &mockKeyClient{})
	if err := m.DeleteDevice(context.Background(), "node1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteDevice_MissingID(t *testing.T) {
	m := newTestManager(&mockDeviceClient{}, &mockKeyClient{})
	err := m.DeleteDevice(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty deviceID")
	}
}

func TestSetDeviceTags_Success(t *testing.T) {
	devices := &mockDeviceClient{}
	m := newTestManager(devices, &mockKeyClient{})
	if err := m.SetDeviceTags(context.Background(), "node1", []string{"tag:web", "tag:prod"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetDeviceTags_MissingID(t *testing.T) {
	m := newTestManager(&mockDeviceClient{}, &mockKeyClient{})
	err := m.SetDeviceTags(context.Background(), "", []string{"tag:web"})
	if err == nil {
		t.Fatal("expected error for empty deviceID")
	}
}

func TestListAuthKeys_Success(t *testing.T) {
	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	expires := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	keys := &mockKeyClient{
		listResult: []tailscale.Key{
			{
				ID:          "key1",
				Key:         "tskey-auth-abc",
				KeyType:     "auth",
				Description: "ci key",
				Tags:        []string{"tag:ci"},
				Scopes:      []string{"all:write"},
				Created:     created,
				Expires:     expires,
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
							Reusable:      true,
							Ephemeral:     true,
							Preauthorized: true,
						},
					},
				},
			},
		},
	}
	m := newTestManager(&mockDeviceClient{}, keys)
	out, err := m.ListAuthKeys(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 key, got %d", len(out))
	}
	k := out[0]
	if k.ID != "key1" || !k.Reusable || !k.Ephemeral || !k.Preauthorized {
		t.Fatalf("unexpected key fields: %+v", k)
	}
}

func TestCreateAuthKey_Success(t *testing.T) {
	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	expires := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	keys := &mockKeyClient{
		createResult: &tailscale.Key{
			ID:          "newkey",
			Key:         "tskey-auth-new",
			KeyType:     "auth",
			Description: "test",
			Created:     created,
			Expires:     expires,
		},
	}
	m := newTestManager(&mockDeviceClient{}, keys)
	out, err := m.CreateAuthKey(context.Background(), CreateAuthKeyRequest{
		Description:   "test",
		Expiry:        time.Hour,
		Reusable:      true,
		Ephemeral:     true,
		Preauthorized: true,
		Tags:          []string{"tag:ci"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID != "newkey" || out.Key != "tskey-auth-new" {
		t.Fatalf("unexpected key: %+v", out)
	}
}

func TestDeleteAuthKey_Success(t *testing.T) {
	keys := &mockKeyClient{}
	m := newTestManager(&mockDeviceClient{}, keys)
	if err := m.DeleteAuthKey(context.Background(), "key1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteAuthKey_MissingID(t *testing.T) {
	m := newTestManager(&mockDeviceClient{}, &mockKeyClient{})
	err := m.DeleteAuthKey(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty keyID")
	}
}

func TestAuthorizeDevice_Success(t *testing.T) {
	m := newTestManager(&mockDeviceClient{}, &mockKeyClient{})
	if err := m.AuthorizeDevice(context.Background(), "node1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthorizeDevice_MissingID(t *testing.T) {
	m := newTestManager(&mockDeviceClient{}, &mockKeyClient{})
	err := m.AuthorizeDevice(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty deviceID")
	}
}

func TestUpdateDeviceKeyExpiry_Success(t *testing.T) {
	devices := &mockDeviceClient{}
	m := newTestManager(devices, &mockKeyClient{})
	if err := m.UpdateDeviceKeyExpiry(context.Background(), "node1", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !devices.lastKey.KeyExpiryDisabled {
		t.Fatal("expected KeyExpiryDisabled=true")
	}
}

func TestUpdateDeviceKeyExpiry_MissingID(t *testing.T) {
	m := newTestManager(&mockDeviceClient{}, &mockKeyClient{})
	err := m.UpdateDeviceKeyExpiry(context.Background(), "", false)
	if err == nil {
		t.Fatal("expected error for empty deviceID")
	}
}

func TestExpireDevice_Success(t *testing.T) {
	devices := &mockDeviceClient{}
	m := newTestManager(devices, &mockKeyClient{})
	m.http = nil
	m.baseURL = ""
	m.apiKey = ""
	err := m.ExpireDevice(context.Background(), "node1")
	if err == nil {
		t.Fatal("expected error because baseURL is empty")
	}
}

func TestSetDeviceName_Success(t *testing.T) {
	devices := &mockDeviceClient{}
	m := newTestManager(devices, &mockKeyClient{})
	if err := m.SetDeviceName(context.Background(), "node1", "merchant-123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if devices.lastName != "merchant-123" {
		t.Fatalf("unexpected name: %q", devices.lastName)
	}
}

func TestSetDeviceName_MissingID(t *testing.T) {
	m := newTestManager(&mockDeviceClient{}, &mockKeyClient{})
	err := m.SetDeviceName(context.Background(), "", "name")
	if err == nil {
		t.Fatal("expected error for empty deviceID")
	}
}

func TestSetDeviceName_MissingName(t *testing.T) {
	m := newTestManager(&mockDeviceClient{}, &mockKeyClient{})
	err := m.SetDeviceName(context.Background(), "node1", "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestFromSDKDevice_NoTailscaleIP(t *testing.T) {
	d := fromSDKDevice(&tailscale.Device{
		ID:        "id3",
		Addresses: []string{"192.168.1.1", "10.0.0.1"},
		Created:   tailscale.Time{Time: time.Now()},
		Expires:   tailscale.Time{Time: time.Now()},
	})
	if len(d.TailscaleIPs) != 0 {
		t.Fatalf("expected no TailscaleIPs, got %v", d.TailscaleIPs)
	}
}

func TestFromSDKKey_Capabilities(t *testing.T) {
	k := fromSDKKey(&tailscale.Key{
		ID: "key2",
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
					Reusable:  true,
					Ephemeral: true,
				},
			},
		},
	})
	if !k.Reusable || !k.Ephemeral {
		t.Fatalf("capabilities not mapped: %+v", k)
	}
}

func TestFilterTailscaleIPs(t *testing.T) {
	in := []string{"100.64.0.1", "192.168.1.1", "100.100.100.100", "fd7a:115c:a1e0::1"}
	out := filterTailscaleIPs(in)
	want := []string{"100.64.0.1", "100.100.100.100"}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("got %v, want %v", out, want)
	}
}

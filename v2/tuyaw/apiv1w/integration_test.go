//go:build integration

package apiv1w

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/AndreeJait/go-utility/v2/tuyaw"
)

// newIntegrationClient creates a Tuya client for integration testing.
// Requires TUYA_ACCESS_ID and TUYA_ACCESS_KEY environment variables.
//
// Run with:
//
//	cd v2 && go test ./tuyaw/apiv1w/ -tags=integration -run TestIntegration -v -timeout 120s
func newIntegrationClient(t *testing.T) tuyaw.Tuya {
	t.Helper()

	accessID := os.Getenv("TUYA_ACCESS_ID")
	if accessID == "" {
		t.Fatal("TUYA_ACCESS_ID environment variable is required for integration tests")
	}

	accessKey := os.Getenv("TUYA_ACCESS_KEY")
	if accessKey == "" {
		t.Fatal("TUYA_ACCESS_KEY environment variable is required for integration tests")
	}

	cf, err := New(&Config{
		AccessID:  accessID,
		AccessKey: accessKey,
		DebugMode: true,
		Region:    "Sg",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return cf
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func TestIntegration_GetToken(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cf := newIntegrationClient(t)
	token, err := cf.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	t.Logf("token: access_token=%s... refresh_token=%s... expire_time=%d uid=%s",
		token.AccessToken[:min(10, len(token.AccessToken))],
		token.RefreshToken[:min(10, len(token.RefreshToken))],
		token.ExpireTime,
		token.UID,
	)

	if token.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if token.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token")
	}
}

func TestIntegration_RefreshToken(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cf := newIntegrationClient(t)

	// First get a token
	token, err := cf.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}

	// Then refresh it
	newToken, err := cf.RefreshToken(ctx, token.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	t.Logf("refreshed token: access_token=%s... expire_time=%d",
		newToken.AccessToken[:min(10, len(newToken.AccessToken))],
		newToken.ExpireTime,
	)

	if newToken.AccessToken == "" {
		t.Fatal("expected non-empty access token after refresh")
	}
}

func TestIntegration_ListDevices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cf := newIntegrationClient(t)

	// Get token first
	_, err := cf.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}

	uid := os.Getenv("TUYA_UID")
	if uid == "" {
		t.Skip("TUYA_UID environment variable not set, skipping list devices test")
	}

	devices, err := cf.ListDevices(ctx, &tuyaw.ListDevicesOptions{UID: uid})
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	t.Logf("devices count: %d", len(devices))

	for i, d := range devices {
		t.Logf("device[%d]: id=%s name=%s category=%s online=%v", i, d.ID, d.Name, d.Category, d.Online)
	}
}

func TestIntegration_GetDevice_Status(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cf := newIntegrationClient(t)

	// Get token first
	_, err := cf.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}

	deviceID := os.Getenv("TUYA_DEVICE_ID")
	if deviceID == "" {
		t.Skip("TUYA_DEVICE_ID environment variable not set, skipping device test")
	}

	// Get device details
	device, err := cf.GetDevice(ctx, deviceID)
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	t.Logf("device: id=%s name=%s category=%s online=%v", device.ID, device.Name, device.Category, device.Online)

	// Get device status
	status, err := cf.GetDeviceStatus(ctx, deviceID)
	if err != nil {
		t.Fatalf("GetDeviceStatus: %v", err)
	}
	t.Logf("device status: %d points", len(status))
	for i, s := range status {
		t.Logf("  status[%d]: code=%s type=%s value=%v", i, s.Code, s.Type, s.Value)
	}
}

func TestIntegration_GetDeviceFunctions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cf := newIntegrationClient(t)

	// Get token first
	_, err := cf.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}

	deviceID := os.Getenv("TUYA_DEVICE_ID")
	if deviceID == "" {
		t.Skip("TUYA_DEVICE_ID environment variable not set, skipping functions test")
	}

	functions, err := cf.GetDeviceFunctions(ctx, deviceID)
	if err != nil {
		t.Fatalf("GetDeviceFunctions: %v", err)
	}
	t.Logf("device category: %s, functions: %d", functions.Category, len(functions.Functions))
	for i, f := range functions.Functions {
		t.Logf("  function[%d]: code=%s type=%s name=%s", i, f.Code, f.Type, f.Name)
	}
}

func TestIntegration_SendCommands(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cf := newIntegrationClient(t)

	// Get token first
	_, err := cf.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}

	deviceID := os.Getenv("TUYA_DEVICE_ID")
	if deviceID == "" {
		t.Skip("TUYA_DEVICE_ID environment variable not set, skipping commands test")
	}

	// Get current status to find a safe command to send
	functions, err := cf.GetDeviceFunctions(ctx, deviceID)
	if err != nil {
		t.Fatalf("GetDeviceStatus: %v", err)
	}

	logw.CtxInfof(ctx, "%v", functions)
	err = cf.SendCommands(ctx, deviceID, []tuyaw.DeviceCommand{
		{
			Code:  "switch",
			Value: true,
		},
	})
	if err != nil {
		t.Fatalf("SendCommands: %v", err)
	}

	time.Sleep(5 * time.Second)

	logw.CtxInfof(ctx, "%v", functions)
	err = cf.SendCommands(ctx, deviceID, []tuyaw.DeviceCommand{
		{
			Code:  "switch",
			Value: false,
		},
	})
	if err != nil {
		t.Fatalf("SendCommands: %v", err)
	}
}

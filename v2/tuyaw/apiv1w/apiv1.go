package apiv1w

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AndreeJait/go-utility/v2/tuyaw"
)

const defaultBaseURL = "https://openapi-sg.iotbing.com"

// Region base URLs for convenience.
const (
	RegionCN = "https://openapi.tuyacn.com"
	RegionUS = "https://openapi.tuyaus.com"
	RegionEU = "https://openapi.tuyaeu.com"
	RegionIN = "https://openapi.tuyain.com"
	RegionSG = "https://openapi-sg.iotbing.com"
)

// Config holds the configuration for the Tuya API v1 client.
type Config struct {
	// AccessID is the client_id from your Tuya project. Required.
	AccessID string

	// AccessKey is the secret key from your Tuya project. Required.
	AccessKey string

	// BaseURL overrides the Tuya API base URL.
	// Defaults to "https://openapi-sg.iotbing.com" if empty.
	// If Region is set, Region takes precedence.
	BaseURL string

	// Region sets the base URL by region. Supported: "us", "cn", "eu", "in", "sg".
	// If set, overrides BaseURL. Optional.
	Region string

	// HTTPClient is an optional custom HTTP client. If nil, http.DefaultClient is used.
	HTTPClient *http.Client

	// DebugMode enables verbose logging of API requests and responses.
	DebugMode bool
}

type tuyaClient struct {
	client    *http.Client
	baseURL   string
	accessID  string
	accessKey string
	debug     bool

	mu           sync.RWMutex
	accessToken  string
	refreshToken string
	expireAt     time.Time
}

var _ tuyaw.Tuya = (*tuyaClient)(nil)

// New creates a new Tuya API v1 client from the given config.
func New(cfg *Config) (tuyaw.Tuya, error) {
	if cfg == nil {
		return nil, fmt.Errorf("apiv1w: config is required")
	}
	if cfg.AccessID == "" {
		return nil, fmt.Errorf("apiv1w: access id is required")
	}
	if cfg.AccessKey == "" {
		return nil, fmt.Errorf("apiv1w: access key is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	// Region overrides BaseURL
	switch strings.ToLower(cfg.Region) {
	case "cn":
		baseURL = RegionCN
	case "us":
		baseURL = RegionUS
	case "eu":
		baseURL = RegionEU
	case "in":
		baseURL = RegionIN
	case "sg":
		baseURL = RegionSG
	case "":
		// use baseURL as-is
	default:
		return nil, fmt.Errorf("apiv1w: unsupported region %q, use cn, us, eu, in, or sg", cfg.Region)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	return &tuyaClient{
		client:    client,
		baseURL:   baseURL,
		accessID:  cfg.AccessID,
		accessKey: cfg.AccessKey,
		debug:     cfg.DebugMode,
	}, nil
}

// --- Token management ---

func (c *tuyaClient) tokenValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken != "" && time.Now().Before(c.expireAt.Add(-5*time.Minute))
}

func (c *tuyaClient) getAccessToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken
}

func (c *tuyaClient) getRefreshToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.refreshToken
}

func (c *tuyaClient) setToken(token *tuyaw.Token) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = token.AccessToken
	c.refreshToken = token.RefreshToken
	c.expireAt = time.Now().Add(time.Duration(token.ExpireTime) * time.Second)
}

// ensureToken checks if the current token is valid and refreshes/acquires if needed.
func (c *tuyaClient) ensureToken(ctx context.Context) error {
	if c.tokenValid() {
		return nil
	}

	// Try refresh first if we have a refresh token
	refreshToken := c.getRefreshToken()
	if refreshToken != "" {
		token, err := c.RefreshToken(ctx, refreshToken)
		if err != nil {
			// Refresh failed, try getting a new token
			token, err = c.GetToken(ctx)
			if err != nil {
				return fmt.Errorf("apiv1w: failed to acquire token: %w", err)
			}
			c.setToken(token)
			return nil
		}
		c.setToken(token)
		return nil
	}

	// No refresh token, get a new one
	token, err := c.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("apiv1w: failed to acquire token: %w", err)
	}
	c.setToken(token)
	return nil
}

// --- HMAC-SHA256 signing ---

func sha256Hex(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func hmacSHA256(data, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *tuyaClient) sign(method, path, body string, t int64, nonce, accessToken string) string {
	// 1. SHA256 of body (empty body uses known constant)
	contentSHA256 := sha256Hex(body)
	if body == "" {
		contentSHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}

	// 2. stringToSign = method + "\n" + contentSHA256 + "\n" + signatureHeaders + "\n" + url
	stringToSign := method + "\n" + contentSHA256 + "\n\n" + path

	// 3. str = client_id + access_token + t + nonce + stringToSign
	str := c.accessID + accessToken + fmt.Sprintf("%d", t) + nonce + stringToSign

	// 4. sign = HMAC-SHA256(str, secret).toUpperCase()
	return strings.ToUpper(hmacSHA256(str, c.accessKey))
}

// --- HTTP request helper ---

type tuyaResponse struct {
	Success bool            `json:"success"`
	Code    int             `json:"code"`
	Msg     string          `json:"msg"`
	Result  json.RawMessage `json:"result"`
	T       int64           `json:"t"`
}

func (c *tuyaClient) doRequest(ctx context.Context, method, path string, body io.Reader, isTokenRequest bool) (json.RawMessage, error) {
	t := time.Now().UnixMilli()
	nonce := generateNonce()

	accessToken := ""
	if !isTokenRequest {
		if err := c.ensureToken(ctx); err != nil {
			return nil, err
		}
		accessToken = c.getAccessToken()
	}

	// Read body for signing (we need to know the content for the hash)
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("apiv1w: reading request body: %w", err)
		}
	}

	bodyStr := ""
	if len(bodyBytes) > 0 {
		bodyStr = string(bodyBytes)
	}

	sign := c.sign(method, path, bodyStr, t, nonce, accessToken)

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("apiv1w: creating request: %w", err)
	}

	req.Header.Set("client_id", c.accessID)
	req.Header.Set("sign", sign)
	req.Header.Set("sign_method", "HMAC-SHA256")
	req.Header.Set("t", strconv.FormatInt(t, 10))
	req.Header.Set("nonce", nonce)

	if !isTokenRequest && accessToken != "" {
		req.Header.Set("access_token", accessToken)
	}

	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: reading response: %w", err)
	}

	var tuyaResp tuyaResponse
	if err := json.Unmarshal(respBody, &tuyaResp); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding response: %w", err)
	}

	if !tuyaResp.Success {
		return nil, fmt.Errorf("apiv1w: api error %d: %s", tuyaResp.Code, tuyaResp.Msg)
	}

	return tuyaResp.Result, nil
}

func generateNonce() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(i * 7 % 256)
	}
	return hex.EncodeToString(b)
}

// --- Token Management ---

func (c *tuyaClient) GetToken(ctx context.Context) (*tuyaw.Token, error) {
	result, err := c.doRequest(ctx, http.MethodGet, "/v1.0/token?grant_type=1", nil, true)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: get token: %w", err)
	}

	var token tuyaw.Token
	if err := json.Unmarshal(result, &token); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding token: %w", err)
	}

	c.setToken(&token)
	return &token, nil
}

func (c *tuyaClient) RefreshToken(ctx context.Context, refreshToken string) (*tuyaw.Token, error) {
	path := "/v1.0/token/" + refreshToken
	result, err := c.doRequest(ctx, http.MethodGet, path, nil, true)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: refresh token: %w", err)
	}

	var token tuyaw.Token
	if err := json.Unmarshal(result, &token); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding token: %w", err)
	}

	c.setToken(&token)
	return &token, nil
}

// --- Device Management ---

func (c *tuyaClient) GetDevice(ctx context.Context, deviceID string) (*tuyaw.Device, error) {
	result, err := c.doRequest(ctx, http.MethodGet, "/v1.0/devices/"+deviceID, nil, false)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: get device: %w", err)
	}

	var device tuyaw.Device
	if err := json.Unmarshal(result, &device); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding device: %w", err)
	}
	return &device, nil
}

func (c *tuyaClient) ListDevices(ctx context.Context, opts *tuyaw.ListDevicesOptions) ([]tuyaw.Device, error) {
	var path string
	if opts != nil && opts.UID != "" {
		path = "/v1.0/users/" + opts.UID + "/devices"
	} else {
		path = "/v1.0/devices"
	}

	// Add query parameters
	params := url.Values{}
	if opts != nil {
		if opts.PageNo > 0 {
			params.Set("page_no", strconv.Itoa(opts.PageNo))
		}
		if opts.PageSize > 0 {
			params.Set("page_size", strconv.Itoa(opts.PageSize))
		}
		if opts.ProductID != "" {
			params.Set("product_id", opts.ProductID)
		}
		if len(opts.DeviceIDs) > 0 {
			params.Set("device_ids", strings.Join(opts.DeviceIDs, ","))
		}
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	result, err := c.doRequest(ctx, http.MethodGet, path, nil, false)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: list devices: %w", err)
	}

	var devices []tuyaw.Device
	if err := json.Unmarshal(result, &devices); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding devices: %w", err)
	}
	return devices, nil
}

func (c *tuyaClient) UpdateDeviceName(ctx context.Context, deviceID, name string) error {
	body, _ := json.Marshal(map[string]string{"name": name})
	_, err := c.doRequest(ctx, http.MethodPut, "/v1.0/devices/"+deviceID, bytes.NewReader(body), false)
	if err != nil {
		return fmt.Errorf("apiv1w: update device name: %w", err)
	}
	return nil
}

func (c *tuyaClient) DeleteDevice(ctx context.Context, deviceID string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/v1.0/devices/"+deviceID, nil, false)
	if err != nil {
		return fmt.Errorf("apiv1w: delete device: %w", err)
	}
	return nil
}

func (c *tuyaClient) GetDeviceSpecifications(ctx context.Context, deviceID string) (*tuyaw.DeviceSpecification, error) {
	result, err := c.doRequest(ctx, http.MethodGet, "/v1.0/devices/"+deviceID+"/specifications", nil, false)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: get device specifications: %w", err)
	}

	var spec tuyaw.DeviceSpecification
	if err := json.Unmarshal(result, &spec); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding device specifications: %w", err)
	}
	return &spec, nil
}

func (c *tuyaClient) GetDeviceFactoryInfos(ctx context.Context, deviceIDs []string) ([]tuyaw.DeviceFactoryInfo, error) {
	path := "/v1.0/devices/factory-infos?device_ids=" + strings.Join(deviceIDs, ",")
	result, err := c.doRequest(ctx, http.MethodGet, path, nil, false)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: get device factory infos: %w", err)
	}

	var infos []tuyaw.DeviceFactoryInfo
	if err := json.Unmarshal(result, &infos); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding device factory infos: %w", err)
	}
	return infos, nil
}

func (c *tuyaClient) ListSubDevices(ctx context.Context, deviceID string) ([]tuyaw.SubDevice, error) {
	result, err := c.doRequest(ctx, http.MethodGet, "/v1.0/devices/"+deviceID+"/sub-devices", nil, false)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: list sub-devices: %w", err)
	}

	var devices []tuyaw.SubDevice
	if err := json.Unmarshal(result, &devices); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding sub-devices: %w", err)
	}
	return devices, nil
}

func (c *tuyaClient) GetDeviceLogs(ctx context.Context, deviceID string, opts *tuyaw.DeviceLogOptions) ([]tuyaw.DeviceLog, error) {
	path := "/v1.0/devices/" + deviceID + "/logs"
	if opts != nil {
		params := url.Values{}
		if opts.StartTime > 0 {
			params.Set("start_time", strconv.FormatInt(opts.StartTime, 10))
		}
		if opts.EndTime > 0 {
			params.Set("end_time", strconv.FormatInt(opts.EndTime, 10))
		}
		if opts.Category != "" {
			params.Set("type", opts.Category)
		}
		if opts.PageNo > 0 {
			params.Set("page_no", strconv.Itoa(opts.PageNo))
		}
		if opts.PageSize > 0 {
			params.Set("page_size", strconv.Itoa(opts.PageSize))
		}
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
	}

	result, err := c.doRequest(ctx, http.MethodGet, path, nil, false)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: get device logs: %w", err)
	}

	var logs []tuyaw.DeviceLog
	if err := json.Unmarshal(result, &logs); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding device logs: %w", err)
	}
	return logs, nil
}

// --- Device Control ---

func (c *tuyaClient) SendCommands(ctx context.Context, deviceID string, commands []tuyaw.DeviceCommand) error {
	body, _ := json.Marshal(map[string]any{"commands": commands})
	_, err := c.doRequest(ctx, http.MethodPost, "/v1.0/devices/"+deviceID+"/commands", bytes.NewReader(body), false)
	if err != nil {
		return fmt.Errorf("apiv1w: send commands: %w", err)
	}
	return nil
}

func (c *tuyaClient) GetDeviceStatus(ctx context.Context, deviceID string) ([]tuyaw.DeviceStatusPoint, error) {
	result, err := c.doRequest(ctx, http.MethodGet, "/v1.0/devices/"+deviceID+"/status", nil, false)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: get device status: %w", err)
	}

	var status []tuyaw.DeviceStatusPoint
	if err := json.Unmarshal(result, &status); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding device status: %w", err)
	}
	return status, nil
}

func (c *tuyaClient) GetDeviceFunctions(ctx context.Context, deviceID string) (*tuyaw.DeviceFunctionsResult, error) {
	result, err := c.doRequest(ctx, http.MethodGet, "/v1.0/devices/"+deviceID+"/functions", nil, false)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: get device functions: %w", err)
	}

	var functionsResult tuyaw.DeviceFunctionsResult
	if err := json.Unmarshal(result, &functionsResult); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding device functions: %w", err)
	}
	return &functionsResult, nil
}

func (c *tuyaClient) GetCategoryFunctions(ctx context.Context, category string) (*tuyaw.DeviceFunctionsResult, error) {
	result, err := c.doRequest(ctx, http.MethodGet, "/v1.0/functions/"+category, nil, false)
	if err != nil {
		return nil, fmt.Errorf("apiv1w: get category functions: %w", err)
	}

	var functionsResult tuyaw.DeviceFunctionsResult
	if err := json.Unmarshal(result, &functionsResult); err != nil {
		return nil, fmt.Errorf("apiv1w: decoding category functions: %w", err)
	}
	return &functionsResult, nil
}

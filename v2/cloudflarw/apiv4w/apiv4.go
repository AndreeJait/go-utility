package apiv4w

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/AndreeJait/go-utility/v2/cloudflarw"
)

const defaultBaseURL = "https://api.cloudflare.com/client/v4"

// Config holds the configuration for the Cloudflare API v4 client.
type Config struct {
	// APIToken is the Cloudflare API Bearer token. Required.
	APIToken string

	// AccountID is the default account ID used when accountID is not passed
	// explicitly to tunnel and access methods. Optional.
	AccountID string

	// BaseURL overrides the Cloudflare API base URL.
	// Defaults to "https://api.cloudflare.com/client/v4" if empty.
	BaseURL string

	// HTTPClient is an optional custom HTTP client. If nil, http.DefaultClient is used.
	HTTPClient *http.Client

	// DebugMode enables verbose logging of API requests and responses.
	DebugMode bool
}

type cfClient struct {
	client    *http.Client
	baseURL   string
	token     string
	accountID string
	debug     bool
}

var _ cloudflarw.Cloudflare = (*cfClient)(nil)

// New creates a new Cloudflare API v4 client from the given config.
func New(cfg *Config) (cloudflarw.Cloudflare, error) {
	if cfg == nil {
		return nil, fmt.Errorf("apiv4w: config is required")
	}
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("apiv4w: api token is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	return &cfClient{
		client:    client,
		baseURL:   baseURL,
		token:     cfg.APIToken,
		accountID: cfg.AccountID,
		debug:     cfg.DebugMode,
	}, nil
}

// --- API response envelope ---

type apiResponse struct {
	Success    bool                       `json:"success"`
	Errors     []apiError                 `json:"errors"`
	Messages   []apiMessage               `json:"messages"`
	Result     json.RawMessage            `json:"result"`
	ResultInfo *cloudflarw.ListResultInfo  `json:"result_info,omitempty"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type apiMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- Internal helpers ---

func (c *cfClient) doRequest(ctx context.Context, method, path string, body io.Reader) (json.RawMessage, *cloudflarw.ListResultInfo, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, nil, fmt.Errorf("apiv4w: creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("apiv4w: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("apiv4w: reading response: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, nil, fmt.Errorf("apiv4w: decoding response: %w", err)
	}

	if !apiResp.Success {
		var errMsgs []string
		for _, e := range apiResp.Errors {
			errMsgs = append(errMsgs, fmt.Sprintf("%d: %s", e.Code, e.Message))
		}
		return nil, nil, fmt.Errorf("apiv4w: api error: %s", strings.Join(errMsgs, "; "))
	}

	return apiResp.Result, apiResp.ResultInfo, nil
}

func (c *cfClient) resolveAccountID(accountID string) (string, error) {
	if accountID != "" {
		return accountID, nil
	}
	if c.accountID != "" {
		return c.accountID, nil
	}
	return "", fmt.Errorf("apiv4w: account id is required")
}

// --- Accounts ---

func (c *cfClient) ListAccounts(ctx context.Context) ([]cloudflarw.Account, *cloudflarw.ListResultInfo, error) {
	result, info, err := c.doRequest(ctx, http.MethodGet, "/accounts", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("apiv4w: list accounts: %w", err)
	}

	var accounts []cloudflarw.Account
	if err := json.Unmarshal(result, &accounts); err != nil {
		return nil, nil, fmt.Errorf("apiv4w: decoding accounts: %w", err)
	}
	return accounts, info, nil
}

// --- Zones ---

func (c *cfClient) ListZones(ctx context.Context) ([]cloudflarw.Zone, *cloudflarw.ListResultInfo, error) {
	result, info, err := c.doRequest(ctx, http.MethodGet, "/zones", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("apiv4w: list zones: %w", err)
	}

	var zones []cloudflarw.Zone
	if err := json.Unmarshal(result, &zones); err != nil {
		return nil, nil, fmt.Errorf("apiv4w: decoding zones: %w", err)
	}
	return zones, info, nil
}

func (c *cfClient) GetZone(ctx context.Context, zoneID string) (*cloudflarw.Zone, error) {
	result, _, err := c.doRequest(ctx, http.MethodGet, "/zones/"+zoneID, nil)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: get zone: %w", err)
	}

	var zone cloudflarw.Zone
	if err := json.Unmarshal(result, &zone); err != nil {
		return nil, fmt.Errorf("apiv4w: decoding zone: %w", err)
	}
	return &zone, nil
}

// --- DNS Records ---

func (c *cfClient) ListDNSRecords(ctx context.Context, zoneID string) ([]cloudflarw.DNSRecord, *cloudflarw.ListResultInfo, error) {
	result, info, err := c.doRequest(ctx, http.MethodGet, "/zones/"+zoneID+"/dns_records", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("apiv4w: list dns records: %w", err)
	}

	var records []cloudflarw.DNSRecord
	if err := json.Unmarshal(result, &records); err != nil {
		return nil, nil, fmt.Errorf("apiv4w: decoding dns records: %w", err)
	}
	return records, info, nil
}

func (c *cfClient) GetDNSRecord(ctx context.Context, zoneID, recordID string) (*cloudflarw.DNSRecord, error) {
	result, _, err := c.doRequest(ctx, http.MethodGet, "/zones/"+zoneID+"/dns_records/"+recordID, nil)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: get dns record: %w", err)
	}

	var record cloudflarw.DNSRecord
	if err := json.Unmarshal(result, &record); err != nil {
		return nil, fmt.Errorf("apiv4w: decoding dns record: %w", err)
	}
	return &record, nil
}

func (c *cfClient) CreateDNSRecord(ctx context.Context, zoneID string, record *cloudflarw.DNSRecord) (*cloudflarw.DNSRecord, error) {
	body, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: marshal dns record: %w", err)
	}

	result, _, err := c.doRequest(ctx, http.MethodPost, "/zones/"+zoneID+"/dns_records", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("apiv4w: create dns record: %w", err)
	}

	var created cloudflarw.DNSRecord
	if err := json.Unmarshal(result, &created); err != nil {
		return nil, fmt.Errorf("apiv4w: decoding created dns record: %w", err)
	}
	return &created, nil
}

func (c *cfClient) UpdateDNSRecord(ctx context.Context, zoneID, recordID string, record *cloudflarw.DNSRecord) (*cloudflarw.DNSRecord, error) {
	body, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: marshal dns record: %w", err)
	}

	result, _, err := c.doRequest(ctx, http.MethodPatch, "/zones/"+zoneID+"/dns_records/"+recordID, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("apiv4w: update dns record: %w", err)
	}

	var updated cloudflarw.DNSRecord
	if err := json.Unmarshal(result, &updated); err != nil {
		return nil, fmt.Errorf("apiv4w: decoding updated dns record: %w", err)
	}
	return &updated, nil
}

func (c *cfClient) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	_, _, err := c.doRequest(ctx, http.MethodDelete, "/zones/"+zoneID+"/dns_records/"+recordID, nil)
	if err != nil {
		return fmt.Errorf("apiv4w: delete dns record: %w", err)
	}
	return nil
}

// --- Tunnels ---

func (c *cfClient) ListTunnels(ctx context.Context, accountID string) ([]cloudflarw.Tunnel, *cloudflarw.ListResultInfo, error) {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return nil, nil, fmt.Errorf("apiv4w: list tunnels: %w", err)
	}

	result, info, err := c.doRequest(ctx, http.MethodGet, "/accounts/"+resolved+"/cfd_tunnel", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("apiv4w: list tunnels: %w", err)
	}

	var tunnels []cloudflarw.Tunnel
	if err := json.Unmarshal(result, &tunnels); err != nil {
		return nil, nil, fmt.Errorf("apiv4w: decoding tunnels: %w", err)
	}
	return tunnels, info, nil
}

func (c *cfClient) GetTunnel(ctx context.Context, accountID, tunnelID string) (*cloudflarw.Tunnel, error) {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: get tunnel: %w", err)
	}

	result, _, err := c.doRequest(ctx, http.MethodGet, "/accounts/"+resolved+"/cfd_tunnel/"+tunnelID, nil)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: get tunnel: %w", err)
	}

	var tunnel cloudflarw.Tunnel
	if err := json.Unmarshal(result, &tunnel); err != nil {
		return nil, fmt.Errorf("apiv4w: decoding tunnel: %w", err)
	}
	return &tunnel, nil
}

type createTunnelRequest struct {
	Name       string `json:"name"`
	TunnelType string `json:"tunnel_type"`
}

func (c *cfClient) CreateTunnel(ctx context.Context, accountID, name, tunnelType string) (*cloudflarw.Tunnel, error) {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: create tunnel: %w", err)
	}

	body, err := json.Marshal(createTunnelRequest{Name: name, TunnelType: tunnelType})
	if err != nil {
		return nil, fmt.Errorf("apiv4w: marshal create tunnel: %w", err)
	}

	result, _, err := c.doRequest(ctx, http.MethodPost, "/accounts/"+resolved+"/cfd_tunnel", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("apiv4w: create tunnel: %w", err)
	}

	var tunnel cloudflarw.Tunnel
	if err := json.Unmarshal(result, &tunnel); err != nil {
		return nil, fmt.Errorf("apiv4w: decoding created tunnel: %w", err)
	}
	return &tunnel, nil
}

type updateTunnelRequest struct {
	Name         string `json:"name,omitempty"`
	TunnelSecret string `json:"tunnel_secret,omitempty"`
}

func (c *cfClient) UpdateTunnel(ctx context.Context, accountID, tunnelID string, tunnel *cloudflarw.Tunnel) (*cloudflarw.Tunnel, error) {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: update tunnel: %w", err)
	}

	body, err := json.Marshal(updateTunnelRequest{Name: tunnel.Name})
	if err != nil {
		return nil, fmt.Errorf("apiv4w: marshal update tunnel: %w", err)
	}

	result, _, err := c.doRequest(ctx, http.MethodPatch, "/accounts/"+resolved+"/cfd_tunnel/"+tunnelID, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("apiv4w: update tunnel: %w", err)
	}

	var updated cloudflarw.Tunnel
	if err := json.Unmarshal(result, &updated); err != nil {
		return nil, fmt.Errorf("apiv4w: decoding updated tunnel: %w", err)
	}
	return &updated, nil
}

func (c *cfClient) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return fmt.Errorf("apiv4w: delete tunnel: %w", err)
	}

	_, _, err = c.doRequest(ctx, http.MethodDelete, "/accounts/"+resolved+"/cfd_tunnel/"+tunnelID, nil)
	if err != nil {
		return fmt.Errorf("apiv4w: delete tunnel: %w", err)
	}
	return nil
}

func (c *cfClient) GetTunnelConfig(ctx context.Context, accountID, tunnelID string) (*cloudflarw.TunnelConfig, error) {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: get tunnel config: %w", err)
	}

	result, _, err := c.doRequest(ctx, http.MethodGet, "/accounts/"+resolved+"/cfd_tunnel/"+tunnelID+"/configurations", nil)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: get tunnel config: %w", err)
	}

	var config cloudflarw.TunnelConfig
	if err := json.Unmarshal(result, &config); err != nil {
		return nil, fmt.Errorf("apiv4w: decoding tunnel config: %w", err)
	}
	return &config, nil
}

func (c *cfClient) UpdateTunnelConfig(ctx context.Context, accountID, tunnelID string, config *cloudflarw.TunnelConfig) error {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return fmt.Errorf("apiv4w: update tunnel config: %w", err)
	}

	body, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("apiv4w: marshal tunnel config: %w", err)
	}

	_, _, err = c.doRequest(ctx, http.MethodPut, "/accounts/"+resolved+"/cfd_tunnel/"+tunnelID+"/configurations", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("apiv4w: update tunnel config: %w", err)
	}
	return nil
}

func (c *cfClient) GetTunnelToken(ctx context.Context, accountID, tunnelID string) (string, error) {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return "", fmt.Errorf("apiv4w: get tunnel token: %w", err)
	}

	result, _, err := c.doRequest(ctx, http.MethodGet, "/accounts/"+resolved+"/cfd_tunnel/"+tunnelID+"/token", nil)
	if err != nil {
		return "", fmt.Errorf("apiv4w: get tunnel token: %w", err)
	}

	var token string
	if err := json.Unmarshal(result, &token); err != nil {
		return "", fmt.Errorf("apiv4w: decoding tunnel token: %w", err)
	}
	return token, nil
}

// --- Access Applications ---

func (c *cfClient) ListAccessApps(ctx context.Context, accountID string) ([]cloudflarw.AccessApp, *cloudflarw.ListResultInfo, error) {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return nil, nil, fmt.Errorf("apiv4w: list access apps: %w", err)
	}

	result, info, err := c.doRequest(ctx, http.MethodGet, "/accounts/"+resolved+"/access/apps", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("apiv4w: list access apps: %w", err)
	}

	var apps []cloudflarw.AccessApp
	if err := json.Unmarshal(result, &apps); err != nil {
		return nil, nil, fmt.Errorf("apiv4w: decoding access apps: %w", err)
	}
	return apps, info, nil
}

func (c *cfClient) GetAccessApp(ctx context.Context, accountID, appID string) (*cloudflarw.AccessApp, error) {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: get access app: %w", err)
	}

	result, _, err := c.doRequest(ctx, http.MethodGet, "/accounts/"+resolved+"/access/apps/"+appID, nil)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: get access app: %w", err)
	}

	var app cloudflarw.AccessApp
	if err := json.Unmarshal(result, &app); err != nil {
		return nil, fmt.Errorf("apiv4w: decoding access app: %w", err)
	}
	return &app, nil
}

func (c *cfClient) CreateAccessApp(ctx context.Context, accountID string, app *cloudflarw.AccessApp) (*cloudflarw.AccessApp, error) {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: create access app: %w", err)
	}

	body, err := json.Marshal(app)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: marshal access app: %w", err)
	}

	result, _, err := c.doRequest(ctx, http.MethodPost, "/accounts/"+resolved+"/access/apps", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("apiv4w: create access app: %w", err)
	}

	var created cloudflarw.AccessApp
	if err := json.Unmarshal(result, &created); err != nil {
		return nil, fmt.Errorf("apiv4w: decoding created access app: %w", err)
	}
	return &created, nil
}

func (c *cfClient) UpdateAccessApp(ctx context.Context, accountID, appID string, app *cloudflarw.AccessApp) (*cloudflarw.AccessApp, error) {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: update access app: %w", err)
	}

	body, err := json.Marshal(app)
	if err != nil {
		return nil, fmt.Errorf("apiv4w: marshal access app: %w", err)
	}

	result, _, err := c.doRequest(ctx, http.MethodPut, "/accounts/"+resolved+"/access/apps/"+appID, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("apiv4w: update access app: %w", err)
	}

	var updated cloudflarw.AccessApp
	if err := json.Unmarshal(result, &updated); err != nil {
		return nil, fmt.Errorf("apiv4w: decoding updated access app: %w", err)
	}
	return &updated, nil
}

func (c *cfClient) DeleteAccessApp(ctx context.Context, accountID, appID string) error {
	resolved, err := c.resolveAccountID(accountID)
	if err != nil {
		return fmt.Errorf("apiv4w: delete access app: %w", err)
	}

	_, _, err = c.doRequest(ctx, http.MethodDelete, "/accounts/"+resolved+"/access/apps/"+appID, nil)
	if err != nil {
		return fmt.Errorf("apiv4w: delete access app: %w", err)
	}
	return nil
}
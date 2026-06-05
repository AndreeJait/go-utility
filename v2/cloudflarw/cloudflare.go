package cloudflarw

import "context"

// Account represents a Cloudflare account.
type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Zone represents a Cloudflare zone (domain).
type Zone struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Account Account `json:"account"`
}

// DNSRecord represents a DNS record within a zone.
type DNSRecord struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Content    string   `json:"content"`
	Proxied    bool     `json:"proxied"`
	TTL        int      `json:"ttl"`
	Priority   *int     `json:"priority,omitempty"`
	Comment    string   `json:"comment,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	CreatedOn  string   `json:"created_on,omitempty"`
	ModifiedOn string   `json:"modified_on,omitempty"`
}

// Tunnel represents a Cloudflare Tunnel (cloudflared).
type Tunnel struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	TunType        string `json:"tun_type"`
	AccountTag     string `json:"account_tag"`
	ConfigSrc      string `json:"config_src"`
	ConnsCount     int    `json:"conns_count"`
	ConnsActiveAt  string `json:"conns_active_at,omitempty"`
	CreatedAt      string `json:"created_at"`
	DeletedAt      string `json:"deleted_at,omitempty"`
	TunnelToken     string `json:"tunnel_token,omitempty"`
	RemoteConfig   bool   `json:"remote_config"`
}

// TunnelConfig represents the configuration of a Cloudflare Tunnel.
type TunnelConfig struct {
	Config struct {
		Ingress     []IngressRule     `json:"ingress"`
		WarpRouting *WarpRoutingConfig `json:"warp-routing,omitempty"`
	} `json:"config"`
}

// IngressRule represents a single ingress rule in a tunnel configuration.
type IngressRule struct {
	Hostname string        `json:"hostname,omitempty"`
	Path     string        `json:"path,omitempty"`
	Service  string        `json:"service"`
	Origin   *OriginConfig `json:"origin,omitempty"`
}

// OriginConfig represents origin request configuration for an ingress rule.
type OriginConfig struct {
	NoTLSVerify     bool   `json:"noTLSVerify,omitempty"`
	ConnectTimeout  string `json:"connectTimeout,omitempty"`
	TLSTimeout      string `json:"tlsTimeout,omitempty"`
	ProxyType       string `json:"proxyType,omitempty"`
}

// WarpRoutingConfig represents warp routing configuration for a tunnel.
type WarpRoutingConfig struct {
	Enabled bool `json:"enabled"`
}

// AccessApp represents a Cloudflare Zero Trust Access Application.
type AccessApp struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Domain              string   `json:"domain,omitempty"`
	Type                string   `json:"type"`
	Aud                 string   `json:"aud"`
	SessionDuration     string   `json:"session_duration,omitempty"`
	AllowedIDPs         []string `json:"allowed_idps,omitempty"`
	AutoRedirectToIDP   bool     `json:"auto_redirect_to_identity,omitempty"`
	AppLauncherVisible  bool     `json:"app_launcher_visible,omitempty"`
	CreatedAt           string   `json:"created_at,omitempty"`
	UpdatedAt           string   `json:"updated_at,omitempty"`
}

// ListResultInfo holds pagination metadata from Cloudflare API responses.
type ListResultInfo struct {
	Count      int `json:"count"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalCount int `json:"total_count"`
}

// Cloudflare defines the contract for interacting with the Cloudflare API v4.
type Cloudflare interface {
	// --- Accounts ---

	// ListAccounts returns all accounts the authenticated user has access to.
	ListAccounts(ctx context.Context) ([]Account, *ListResultInfo, error)

	// --- Zones ---

	// ListZones returns all zones (domains) for the authenticated account.
	ListZones(ctx context.Context) ([]Zone, *ListResultInfo, error)

	// GetZone returns details for a single zone.
	GetZone(ctx context.Context, zoneID string) (*Zone, error)

	// --- DNS Records ---

	// ListDNSRecords returns all DNS records in a zone.
	ListDNSRecords(ctx context.Context, zoneID string) ([]DNSRecord, *ListResultInfo, error)

	// GetDNSRecord returns a single DNS record.
	GetDNSRecord(ctx context.Context, zoneID, recordID string) (*DNSRecord, error)

	// CreateDNSRecord creates a new DNS record in a zone.
	CreateDNSRecord(ctx context.Context, zoneID string, record *DNSRecord) (*DNSRecord, error)

	// UpdateDNSRecord updates an existing DNS record.
	UpdateDNSRecord(ctx context.Context, zoneID, recordID string, record *DNSRecord) (*DNSRecord, error)

	// DeleteDNSRecord deletes a DNS record from a zone.
	DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error

	// --- Tunnels ---

	// ListTunnels returns all tunnels for the account.
	ListTunnels(ctx context.Context, accountID string) ([]Tunnel, *ListResultInfo, error)

	// GetTunnel returns details for a single tunnel.
	GetTunnel(ctx context.Context, accountID, tunnelID string) (*Tunnel, error)

	// CreateTunnel creates a new tunnel in the account.
	CreateTunnel(ctx context.Context, accountID, name, tunnelType string) (*Tunnel, error)

	// UpdateTunnel updates an existing tunnel.
	UpdateTunnel(ctx context.Context, accountID, tunnelID string, tunnel *Tunnel) (*Tunnel, error)

	// DeleteTunnel deletes a tunnel.
	DeleteTunnel(ctx context.Context, accountID, tunnelID string) error

	// GetTunnelConfig returns the configuration of a tunnel.
	GetTunnelConfig(ctx context.Context, accountID, tunnelID string) (*TunnelConfig, error)

	// UpdateTunnelConfig updates the configuration of a tunnel.
	UpdateTunnelConfig(ctx context.Context, accountID, tunnelID string, config *TunnelConfig) error

	// GetTunnelToken returns the token for a tunnel.
	GetTunnelToken(ctx context.Context, accountID, tunnelID string) (string, error)

	// --- Access Applications (Zero Trust) ---

	// ListAccessApps returns all Access applications for the account.
	ListAccessApps(ctx context.Context, accountID string) ([]AccessApp, *ListResultInfo, error)

	// GetAccessApp returns a single Access application.
	GetAccessApp(ctx context.Context, accountID, appID string) (*AccessApp, error)

	// CreateAccessApp creates a new Access application.
	CreateAccessApp(ctx context.Context, accountID string, app *AccessApp) (*AccessApp, error)

	// UpdateAccessApp updates an existing Access application.
	UpdateAccessApp(ctx context.Context, accountID, appID string, app *AccessApp) (*AccessApp, error)

	// DeleteAccessApp deletes an Access application.
	DeleteAccessApp(ctx context.Context, accountID, appID string) error
}
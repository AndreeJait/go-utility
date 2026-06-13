package tsnetw

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"strconv"

	"github.com/AndreeJait/go-utility/v2/logw"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
	"tailscale.com/types/views"
)

var _ VPN = (*tsnetVPN)(nil)

type tsnetVPN struct {
	server *tsnet.Server
	debug  bool
}

// New creates an embedded Tailscale VPN node from the provided config.
func New(cfg *Config) (VPN, error) {
	if cfg == nil {
		return nil, fmtError("config is required")
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	var srv *tsnet.Server
	if cfg.Server != nil {
		srv = cfg.Server
	} else {
		srv = buildServer(cfg)
	}

	return &tsnetVPN{
		server: srv,
		debug:  cfg.Logf != nil,
	}, nil
}

func buildServer(cfg *Config) *tsnet.Server {
	dir := cfg.Dir
	if dir == "" {
		dir = defaultStateDir(cfg.Hostname)
	}

	logf := cfg.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	userLogf := cfg.UserLogf
	if userLogf == nil {
		userLogf = func(string, ...any) {}
	}

	srv := &tsnet.Server{
		Hostname:      cfg.Hostname,
		Dir:           dir,
		AuthKey:       cfg.AuthKey,
		ClientID:      cfg.OAuthClientID,
		ClientSecret:  cfg.OAuthClientSecret,
		IDToken:       cfg.IDToken,
		Audience:      cfg.Audience,
		Ephemeral:     cfg.Ephemeral,
		ControlURL:    cfg.ControlURL,
		AdvertiseTags: cfg.AdvertiseTags,
		Port:          cfg.Port,
		Logf:          logf,
		UserLogf:      userLogf,
	}

	// Ensure the state directory exists.
	_ = os.MkdirAll(dir, 0o700)

	return srv
}

func defaultStateDir(hostname string) string {
	base, _ := os.UserConfigDir()
	if base == "" {
		base = "/tmp"
	}
	return filepath.Join(base, "tsnetw", hostname)
}

func (v *tsnetVPN) Start(ctx context.Context) (*Status, error) {
	logw.CtxInfof(ctx, "tsnetw: starting tailscale node")

	if err := v.server.Start(); err != nil {
		return nil, fmt.Errorf("tsnetw: start: %w", err)
	}

	status, err := v.server.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("tsnetw: up: %w", err)
	}

	return fromIPNStatus(status), nil
}

func (v *tsnetVPN) Close() error {
	return v.server.Close()
}

func (v *tsnetVPN) HTTPClient() *http.Client {
	return v.server.HTTPClient()
}

func (v *tsnetVPN) Listen(network, addr string) (net.Listener, error) {
	return v.server.Listen(network, addr)
}

func (v *tsnetVPN) ListenTLS(network, addr string) (net.Listener, error) {
	return v.server.ListenTLS(network, addr)
}

func (v *tsnetVPN) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	return v.server.Dial(ctx, network, addr)
}

func (v *tsnetVPN) WhoIs(ctx context.Context, remoteAddr string) (*WhoIsResult, error) {
	lc, err := v.server.LocalClient()
	if err != nil {
		return nil, fmt.Errorf("tsnetw: local client: %w", err)
	}

	whois, err := lc.WhoIs(ctx, remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("tsnetw: whois %q: %w", remoteAddr, err)
	}

	return &WhoIsResult{
		Node:     nodeFromTailcfg(whois.Node),
		User:     whois.UserProfile.LoginName,
		IsTagged: whois.Node.IsTagged(),
	}, nil
}

func (v *tsnetVPN) Status(ctx context.Context) (*Status, error) {
	lc, err := v.server.LocalClient()
	if err != nil {
		return nil, fmt.Errorf("tsnetw: local client: %w", err)
	}

	status, err := lc.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("tsnetw: status: %w", err)
	}

	return fromIPNStatus(status), nil
}

func fromIPNStatus(s *ipnstate.Status) *Status {
	if s == nil {
		return nil
	}

	out := &Status{
		BackendState: s.BackendState,
	}

	if s.Self != nil {
		out.Self = nodeFromPeerStatus(s.Self)
	}

	for _, ip := range s.TailscaleIPs {
		out.TailscaleIPs = append(out.TailscaleIPs, ip.String())
	}

	return out
}

func nodeFromPeerStatus(ps *ipnstate.PeerStatus) Node {
	return Node{
		ID:       string(ps.ID),
		Name:     ps.DNSName,
		Hostname: ps.HostName,
		User:     strconv.FormatInt(int64(ps.UserID), 10),
		Tags:     viewSlice(ps.Tags),
	}
}

func nodeFromTailcfg(n *tailcfg.Node) Node {
	if n == nil {
		return Node{}
	}
	return Node{
		ID:       string(n.StableID),
		Name:     n.Name,
		Hostname: hostinfoHostname(n.Hostinfo),
		User:     strconv.FormatInt(int64(n.User), 10),
		Tags:     n.Tags,
	}
}

func hostinfoHostname(hi tailcfg.HostinfoView) string {
	if !hi.Valid() {
		return ""
	}
	return hi.AsStruct().Hostname
}

func viewSlice(s *views.Slice[string]) []string {
	if s == nil {
		return nil
	}
	return s.AsSlice()
}

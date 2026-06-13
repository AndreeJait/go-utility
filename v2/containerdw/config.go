package containerdw

import "time"

// Config holds connection and default settings for the containerd client.
type Config struct {
	// Address is the containerd gRPC socket path, e.g. "/run/containerd/containerd.sock".
	// If empty, "/run/containerd/containerd.sock" is used.
	Address string

	// Namespace is the default containerd namespace for operations.
	// If empty, "default" is used.
	Namespace string

	// Timeout is the default timeout for operations when no context deadline is set.
	// If zero, no default timeout is applied.
	Timeout time.Duration
}

func (c *Config) address() string {
	if c.Address != "" {
		return c.Address
	}
	return "/run/containerd/containerd.sock"
}

func (c *Config) namespace() string {
	if c.Namespace != "" {
		return c.Namespace
	}
	return "default"
}

package containerdw

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func integrationConfig(t *testing.T) *Config {
	t.Helper()
	addr := os.Getenv("CONTAINERD_ADDRESS")
	if addr == "" {
		addr = "/run/containerd/containerd.sock"
	}

	if _, err := os.Stat(addr); err != nil {
		t.Skipf("containerd socket %q not accessible: %v", addr, err)
	}

	return &Config{
		Address:   addr,
		Namespace: "go-utility-test",
	}
}

func TestIntegration_Version(t *testing.T) {
	cfg := integrationConfig(t)
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create containerd client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	version, err := client.Version(ctx)
	if err != nil {
		t.Fatalf("failed to fetch version: %v", err)
	}
	if version.Version == "" {
		t.Fatalf("expected non-empty version, got %+v", version)
	}
	t.Logf("containerd version: %+v", version)
}

func TestIntegration_ListImages(t *testing.T) {
	cfg := integrationConfig(t)
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create containerd client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	images, err := client.ListImages(ctx)
	if err != nil {
		t.Fatalf("failed to list images: %v", err)
	}
	t.Logf("found %d images", len(images))
}

func TestIntegration_PruneContainers(t *testing.T) {
	cfg := integrationConfig(t)
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create containerd client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := client.PruneContainers(ctx)
	if err != nil {
		t.Fatalf("failed to prune containers: %v", err)
	}
	t.Logf("pruned %d stopped containers", res.Deleted)
}

func TestIntegration_PruneImages(t *testing.T) {
	cfg := integrationConfig(t)
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create containerd client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := client.PruneImages(ctx)
	if err != nil {
		t.Fatalf("failed to prune images: %v", err)
	}
	t.Logf("pruned %d unused images", res.Deleted)
}

func TestIntegration_NetworkLifecycle(t *testing.T) {
	cfg := integrationConfig(t)
	if _, err := exec.LookPath("nerdctl"); err != nil {
		t.Skipf("nerdctl not available: %v", err)
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create containerd client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	name := "go-utility-test-net"
	if err := client.CreateNetwork(ctx, name, "bridge", NetworkOptions{}); err != nil {
		t.Fatalf("failed to create network: %v", err)
	}
	defer client.RemoveNetwork(ctx, name)

	networks, err := client.ListNetworks(ctx)
	if err != nil {
		t.Fatalf("failed to list networks: %v", err)
	}
	found := false
	for _, n := range networks {
		if n.Name == name {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created network %q not found in list", name)
	}
}

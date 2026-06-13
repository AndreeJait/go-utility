// Package containerdw wraps the containerd v2 Go client with a slim,
// repository-consistent interface for image, container, and task lifecycle.
package containerdw

import (
	"context"
	"time"
)

// Containerd provides a high-level interface to a containerd daemon.
type Containerd interface {
	// Version returns containerd server version information.
	Version(ctx context.Context) (Version, error)

	// PullImage pulls an image by reference and unpacks it for the default snapshotter.
	PullImage(ctx context.Context, ref string) (Image, error)

	// GetImage returns an existing image by reference.
	GetImage(ctx context.Context, ref string) (Image, error)

	// ListImages returns all images in the namespace.
	ListImages(ctx context.Context) ([]Image, error)

	// CreateContainer creates a new container from an image.
	CreateContainer(ctx context.Context, id, imageRef string) (Container, error)

	// LoadContainer loads an existing container by ID.
	LoadContainer(ctx context.Context, id string) (Container, error)

	// ListContainers returns all containers in the namespace.
	ListContainers(ctx context.Context) ([]Container, error)

	// StartContainer creates and starts a task for the container.
	StartContainer(ctx context.Context, id string) (Task, error)

	// StopContainer signals a container's task and waits for it to exit.
	StopContainer(ctx context.Context, id string, signal string, timeout time.Duration) error

	// DeleteContainer deletes a container and its task.
	DeleteContainer(ctx context.Context, id string) error

	// ContainerStatus returns the status of a container's task.
	ContainerStatus(ctx context.Context, id string) (TaskStatus, error)

	// ListNetworks returns CNI networks visible to containerd.
	ListNetworks(ctx context.Context) ([]Network, error)

	// CreateNetwork creates a CNI network with the given name and configuration.
	CreateNetwork(ctx context.Context, name, driver string, options NetworkOptions) error

	// RemoveNetwork deletes a CNI network by name.
	RemoveNetwork(ctx context.Context, name string) error

	// PruneImages removes unused images.
	PruneImages(ctx context.Context) (PruneResult, error)

	// PruneContainers removes stopped containers.
	PruneContainers(ctx context.Context) (PruneResult, error)

	// Close closes the underlying client connection.
	Close() error
}

// Network describes a container runtime network.
type Network struct {
	Name   string
	Labels map[string]string
}

// NetworkOptions holds optional settings when creating a network.
type NetworkOptions struct {
	Labels map[string]string
}

// PruneResult reports what was removed by a prune operation.
type PruneResult struct {
	Deleted int
}

// Version describes a containerd server.
type Version struct {
	Version  string
	Revision string
}

// Image describes a container image stored in containerd.
type Image struct {
	Name   string
	Labels map[string]string
}

// Container describes a container metadata object.
type Container struct {
	ID     string
	Image  string
	Labels map[string]string
	Status TaskStatus
	Pid    uint32
}

// Task describes a running container task.
type Task struct {
	ID     string
	Pid    uint32
	Status TaskStatus
}

// TaskStatus is the runtime status of a task.
type TaskStatus string

// Common task statuses.
const (
	StatusCreated TaskStatus = "created"
	StatusRunning TaskStatus = "running"
	StatusStopped TaskStatus = "stopped"
	StatusPaused  TaskStatus = "paused"
	StatusPausing TaskStatus = "pausing"
	StatusUnknown TaskStatus = "unknown"
)

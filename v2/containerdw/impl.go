package containerdw

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
)

var _ Containerd = (*containerdManager)(nil)

// clientAdapter is the subset of *containerd.Client used by this package.
type clientAdapter interface {
	Close() error
	Version(ctx context.Context) (containerd.Version, error)
	Pull(ctx context.Context, ref string, opts ...containerd.RemoteOpt) (containerd.Image, error)
	GetImage(ctx context.Context, ref string) (containerd.Image, error)
	ListImages(ctx context.Context, filters ...string) ([]containerd.Image, error)
	NewContainer(ctx context.Context, id string, opts ...containerd.NewContainerOpts) (containerAdapter, error)
	LoadContainer(ctx context.Context, id string) (containerAdapter, error)
	Containers(ctx context.Context, filters ...string) ([]containerAdapter, error)
}

// containerAdapter is the subset of containerd.Container used by this package.
type containerAdapter interface {
	ID() string
	Image(ctx context.Context) (containerd.Image, error)
	Labels(ctx context.Context) (map[string]string, error)
	NewTask(ctx context.Context, ioCreate cio.Creator, opts ...containerd.NewTaskOpts) (taskAdapter, error)
	Task(context.Context, cio.Attach) (taskAdapter, error)
	Delete(ctx context.Context, opts ...containerd.DeleteOpts) error
}

// taskAdapter is the subset of containerd.Task used by this package.
type taskAdapter interface {
	ID() string
	Pid() uint32
	Status(ctx context.Context) (containerd.Status, error)
	Start(ctx context.Context) error
	Wait(ctx context.Context) (<-chan containerd.ExitStatus, error)
	Kill(ctx context.Context, signal syscall.Signal, opts ...containerd.KillOpts) error
	Delete(ctx context.Context, opts ...containerd.ProcessDeleteOpts) (*containerd.ExitStatus, error)
}

type containerdManager struct {
	client    clientAdapter
	namespace string
	debug     bool
}

// clientWrapper adapts *containerd.Client to clientAdapter and wraps
// container/task results so they can be mocked in tests.
type clientWrapper struct {
	*containerd.Client
}

func (w *clientWrapper) NewContainer(ctx context.Context, id string, opts ...containerd.NewContainerOpts) (containerAdapter, error) {
	c, err := w.Client.NewContainer(ctx, id, opts...)
	if err != nil {
		return nil, err
	}
	return &containerWrapper{c}, nil
}

func (w *clientWrapper) LoadContainer(ctx context.Context, id string) (containerAdapter, error) {
	c, err := w.Client.LoadContainer(ctx, id)
	if err != nil {
		return nil, err
	}
	return &containerWrapper{c}, nil
}

func (w *clientWrapper) Containers(ctx context.Context, filters ...string) ([]containerAdapter, error) {
	cs, err := w.Client.Containers(ctx, filters...)
	if err != nil {
		return nil, err
	}
	out := make([]containerAdapter, len(cs))
	for i, c := range cs {
		out[i] = &containerWrapper{c}
	}
	return out, nil
}

type containerWrapper struct {
	c containerd.Container
}

func (w *containerWrapper) ID() string { return w.c.ID() }

func (w *containerWrapper) Image(ctx context.Context) (containerd.Image, error) {
	return w.c.Image(ctx)
}

func (w *containerWrapper) Labels(ctx context.Context) (map[string]string, error) {
	return w.c.Labels(ctx)
}

func (w *containerWrapper) NewTask(ctx context.Context, ioCreate cio.Creator, opts ...containerd.NewTaskOpts) (taskAdapter, error) {
	t, err := w.c.NewTask(ctx, ioCreate, opts...)
	if err != nil {
		return nil, err
	}
	return &taskWrapper{t}, nil
}

func (w *containerWrapper) Task(ctx context.Context, attach cio.Attach) (taskAdapter, error) {
	t, err := w.c.Task(ctx, attach)
	if err != nil {
		return nil, err
	}
	return &taskWrapper{t}, nil
}

func (w *containerWrapper) Delete(ctx context.Context, opts ...containerd.DeleteOpts) error {
	return w.c.Delete(ctx, opts...)
}

type taskWrapper struct {
	t containerd.Task
}

func (w *taskWrapper) ID() string { return w.t.ID() }

func (w *taskWrapper) Pid() uint32 { return w.t.Pid() }

func (w *taskWrapper) Status(ctx context.Context) (containerd.Status, error) { return w.t.Status(ctx) }

func (w *taskWrapper) Start(ctx context.Context) error { return w.t.Start(ctx) }

func (w *taskWrapper) Wait(ctx context.Context) (<-chan containerd.ExitStatus, error) {
	return w.t.Wait(ctx)
}

func (w *taskWrapper) Kill(ctx context.Context, signal syscall.Signal, opts ...containerd.KillOpts) error {
	return w.t.Kill(ctx, signal, opts...)
}

func (w *taskWrapper) Delete(ctx context.Context, opts ...containerd.ProcessDeleteOpts) (*containerd.ExitStatus, error) {
	return w.t.Delete(ctx, opts...)
}

// New creates a containerd wrapper connected to the configured socket.
func New(cfg *Config) (Containerd, error) {
	if cfg == nil {
		return nil, fmt.Errorf("containerdw: config is required")
	}

	addr := cfg.address()
	if _, err := os.Stat(addr); err != nil && !strings.Contains(addr, ":") {
		return nil, fmt.Errorf("containerdw: containerd socket %q not accessible: %w", addr, err)
	}

	client, err := containerd.New(addr)
	if err != nil {
		return nil, fmt.Errorf("containerdw: connect to %q: %w", addr, err)
	}

	return &containerdManager{
		client:    &clientWrapper{client},
		namespace: cfg.namespace(),
		debug:     true,
	}, nil
}

func (m *containerdManager) withNamespace(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, m.namespace)
}

func (m *containerdManager) Version(ctx context.Context) (Version, error) {
	ctx = m.withNamespace(ctx)

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: fetching version")
	}

	v, err := m.client.Version(ctx)
	if err != nil {
		return Version{}, fmt.Errorf("containerdw: version: %w", err)
	}
	return Version{Version: v.Version, Revision: v.Revision}, nil
}

func (m *containerdManager) PullImage(ctx context.Context, ref string) (Image, error) {
	if ref == "" {
		return Image{}, fmt.Errorf("containerdw: image ref is required")
	}
	ctx = m.withNamespace(ctx)

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: pulling image %q", ref)
	}

	img, err := m.client.Pull(ctx, ref, containerd.WithPullUnpack)
	if err != nil {
		return Image{}, fmt.Errorf("containerdw: pull image %q: %w", ref, err)
	}

	return m.toImage(img), nil
}

func (m *containerdManager) GetImage(ctx context.Context, ref string) (Image, error) {
	if ref == "" {
		return Image{}, fmt.Errorf("containerdw: image ref is required")
	}
	ctx = m.withNamespace(ctx)

	img, err := m.client.GetImage(ctx, ref)
	if err != nil {
		return Image{}, fmt.Errorf("containerdw: get image %q: %w", ref, err)
	}

	return m.toImage(img), nil
}

func (m *containerdManager) ListImages(ctx context.Context) ([]Image, error) {
	ctx = m.withNamespace(ctx)

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: listing images")
	}

	images, err := m.client.ListImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("containerdw: list images: %w", err)
	}

	out := make([]Image, 0, len(images))
	for _, img := range images {
		out = append(out, m.toImage(img))
	}
	return out, nil
}

func (m *containerdManager) CreateContainer(ctx context.Context, id, imageRef string) (Container, error) {
	if id == "" {
		return Container{}, fmt.Errorf("containerdw: container id is required")
	}
	if imageRef == "" {
		return Container{}, fmt.Errorf("containerdw: image ref is required")
	}
	ctx = m.withNamespace(ctx)

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: creating container %q from %q", id, imageRef)
	}

	image, err := m.client.GetImage(ctx, imageRef)
	if err != nil {
		return Container{}, fmt.Errorf("containerdw: get image %q: %w", imageRef, err)
	}

	container, err := m.client.NewContainer(
		ctx,
		id,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(id, image),
		containerd.WithNewSpec(oci.WithImageConfig(image)),
	)
	if err != nil {
		return Container{}, fmt.Errorf("containerdw: create container %q: %w", id, err)
	}

	return m.toContainer(ctx, container), nil
}

func (m *containerdManager) LoadContainer(ctx context.Context, id string) (Container, error) {
	if id == "" {
		return Container{}, fmt.Errorf("containerdw: container id is required")
	}
	ctx = m.withNamespace(ctx)

	container, err := m.client.LoadContainer(ctx, id)
	if err != nil {
		return Container{}, fmt.Errorf("containerdw: load container %q: %w", id, err)
	}

	return m.toContainer(ctx, container), nil
}

func (m *containerdManager) ListContainers(ctx context.Context) ([]Container, error) {
	ctx = m.withNamespace(ctx)

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: listing containers")
	}

	containers, err := m.client.Containers(ctx)
	if err != nil {
		return nil, fmt.Errorf("containerdw: list containers: %w", err)
	}

	out := make([]Container, 0, len(containers))
	for _, c := range containers {
		out = append(out, m.toContainer(ctx, c))
	}
	return out, nil
}

func (m *containerdManager) StartContainer(ctx context.Context, id string) (Task, error) {
	if id == "" {
		return Task{}, fmt.Errorf("containerdw: container id is required")
	}
	ctx = m.withNamespace(ctx)

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: starting container %q", id)
	}

	container, err := m.client.LoadContainer(ctx, id)
	if err != nil {
		return Task{}, fmt.Errorf("containerdw: load container %q: %w", id, err)
	}

	task, err := container.NewTask(ctx, cio.NullIO)
	if err != nil {
		return Task{}, fmt.Errorf("containerdw: create task for %q: %w", id, err)
	}

	if err := task.Start(ctx); err != nil {
		_, _ = task.Delete(ctx, containerd.WithProcessKill)
		return Task{}, fmt.Errorf("containerdw: start task for %q: %w", id, err)
	}

	return m.toTask(task), nil
}

func (m *containerdManager) StopContainer(ctx context.Context, id string, signal string, timeout time.Duration) error {
	if id == "" {
		return fmt.Errorf("containerdw: container id is required")
	}
	ctx = m.withNamespace(ctx)

	sig, err := parseSignal(signal)
	if err != nil {
		return err
	}

	container, err := m.client.LoadContainer(ctx, id)
	if err != nil {
		return fmt.Errorf("containerdw: load container %q: %w", id, err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("containerdw: get task for %q: %w", id, err)
	}

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: stopping task for %q with signal %v", id, sig)
	}

	if err := task.Kill(ctx, sig); err != nil {
		return fmt.Errorf("containerdw: kill task %q: %w", id, err)
	}

	exitCh, err := task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("containerdw: wait task %q: %w", id, err)
	}

	select {
	case <-exitCh:
	case <-time.After(timeout):
		// Force kill if timeout exceeded.
		if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
			return fmt.Errorf("containerdw: force kill task %q: %w", id, err)
		}
		<-exitCh
	}

	if _, err := task.Delete(ctx, containerd.WithProcessKill); err != nil {
		return fmt.Errorf("containerdw: delete task %q: %w", id, err)
	}
	return nil
}

func (m *containerdManager) DeleteContainer(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("containerdw: container id is required")
	}
	ctx = m.withNamespace(ctx)

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: deleting container %q", id)
	}

	container, err := m.client.LoadContainer(ctx, id)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("containerdw: load container %q: %w", id, err)
	}

	// Stop and delete any running task first.
	if task, err := container.Task(ctx, nil); err == nil {
		_ = task.Kill(ctx, syscall.SIGKILL)
		exitCh, _ := task.Wait(ctx)
		select {
		case <-exitCh:
		case <-time.After(10 * time.Second):
		}
		_, _ = task.Delete(ctx, containerd.WithProcessKill)
	}

	if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		return fmt.Errorf("containerdw: delete container %q: %w", id, err)
	}
	return nil
}

func (m *containerdManager) ContainerStatus(ctx context.Context, id string) (TaskStatus, error) {
	if id == "" {
		return StatusUnknown, fmt.Errorf("containerdw: container id is required")
	}
	ctx = m.withNamespace(ctx)

	container, err := m.client.LoadContainer(ctx, id)
	if err != nil {
		return StatusUnknown, fmt.Errorf("containerdw: load container %q: %w", id, err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return StatusStopped, nil
		}
		return StatusUnknown, fmt.Errorf("containerdw: get task for %q: %w", id, err)
	}

	status, err := task.Status(ctx)
	if err != nil {
		return StatusUnknown, fmt.Errorf("containerdw: task status %q: %w", id, err)
	}

	return mapTaskStatus(status.Status), nil
}

func (m *containerdManager) ListNetworks(ctx context.Context) ([]Network, error) {
	if _, err := exec.LookPath("nerdctl"); err != nil {
		return nil, fmt.Errorf("containerdw: nerdctl is required for network management: %w", err)
	}

	ctx = m.withNamespace(ctx)
	if m.debug {
		logw.CtxInfof(ctx, "containerdw: listing networks")
	}

	cmd := exec.CommandContext(ctx, "nerdctl", "network", "ls", "--quiet")
	cmd.Env = append(os.Environ(), "CONTAINERD_NAMESPACE="+m.namespace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("containerdw: list networks: %w: %s", err, string(out))
	}

	names := strings.Fields(string(out))
	networks := make([]Network, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		networks = append(networks, Network{Name: name})
	}
	return networks, nil
}

func (m *containerdManager) CreateNetwork(ctx context.Context, name, driver string, options NetworkOptions) error {
	if name == "" {
		return fmt.Errorf("containerdw: network name is required")
	}
	if _, err := exec.LookPath("nerdctl"); err != nil {
		return fmt.Errorf("containerdw: nerdctl is required for network management: %w", err)
	}
	ctx = m.withNamespace(ctx)

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: creating network %q", name)
	}

	args := []string{"network", "create", name}
	if driver != "" {
		args = append(args, "--driver", driver)
	}
	for k, v := range options.Labels {
		args = append(args, "--label", k+"="+v)
	}

	cmd := exec.CommandContext(ctx, "nerdctl", args...)
	cmd.Env = append(os.Environ(), "CONTAINERD_NAMESPACE="+m.namespace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("containerdw: create network %q: %w: %s", name, err, string(out))
	}
	return nil
}

func (m *containerdManager) RemoveNetwork(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("containerdw: network name is required")
	}
	if _, err := exec.LookPath("nerdctl"); err != nil {
		return fmt.Errorf("containerdw: nerdctl is required for network management: %w", err)
	}
	ctx = m.withNamespace(ctx)

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: removing network %q", name)
	}

	cmd := exec.CommandContext(ctx, "nerdctl", "network", "rm", name)
	cmd.Env = append(os.Environ(), "CONTAINERD_NAMESPACE="+m.namespace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("containerdw: remove network %q: %w: %s", name, err, string(out))
	}
	return nil
}

func (m *containerdManager) PruneImages(ctx context.Context) (PruneResult, error) {
	ctx = m.withNamespace(ctx)

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: pruning unused images")
	}

	client, ok := m.client.(*clientWrapper)
	if !ok {
		return PruneResult{}, fmt.Errorf("containerdw: prune requires a real containerd client")
	}

	imgStore := client.Client.ImageService()
	allImages, err := imgStore.List(ctx)
	if err != nil {
		return PruneResult{}, fmt.Errorf("containerdw: list images for prune: %w", err)
	}

	containers, err := client.Client.Containers(ctx)
	if err != nil {
		return PruneResult{}, fmt.Errorf("containerdw: list containers for prune: %w", err)
	}

	inUse := make(map[string]struct{}, len(containers))
	for _, c := range containers {
		img, err := c.Image(ctx)
		if err != nil || img == nil {
			continue
		}
		inUse[img.Name()] = struct{}{}
	}

	deleted := 0
	for _, img := range allImages {
		if _, used := inUse[img.Name]; used {
			continue
		}
		if err := imgStore.Delete(ctx, img.Name, images.SynchronousDelete()); err != nil {
			if !errdefs.IsNotFound(err) {
				return PruneResult{Deleted: deleted}, fmt.Errorf("containerdw: delete image %q: %w", img.Name, err)
			}
		}
		deleted++
	}
	return PruneResult{Deleted: deleted}, nil
}

func (m *containerdManager) PruneContainers(ctx context.Context) (PruneResult, error) {
	ctx = m.withNamespace(ctx)

	if m.debug {
		logw.CtxInfof(ctx, "containerdw: pruning stopped containers")
	}

	containers, err := m.client.Containers(ctx)
	if err != nil {
		return PruneResult{}, fmt.Errorf("containerdw: list containers for prune: %w", err)
	}

	deleted := 0
	for _, c := range containers {
		_, err := c.Task(ctx, nil)
		hasTask := err == nil
		if hasTask {
			continue
		}
		if !errdefs.IsNotFound(err) {
			return PruneResult{Deleted: deleted}, fmt.Errorf("containerdw: get task for %q: %w", c.ID(), err)
		}
		if err := c.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
			if !errdefs.IsNotFound(err) {
				return PruneResult{Deleted: deleted}, fmt.Errorf("containerdw: delete container %q: %w", c.ID(), err)
			}
		}
		deleted++
	}
	return PruneResult{Deleted: deleted}, nil
}

func (m *containerdManager) Close() error {
	return m.client.Close()
}

func (m *containerdManager) toImage(img containerd.Image) Image {
	return Image{Name: img.Name(), Labels: img.Labels()}
}

func (m *containerdManager) toContainer(ctx context.Context, c containerAdapter) Container {
	id := c.ID()
	image, _ := c.Image(ctx)
	labels, _ := c.Labels(ctx)

	imageName := ""
	if image != nil {
		imageName = image.Name()
	}

	status, pid := StatusStopped, uint32(0)
	if task, err := c.Task(ctx, nil); err == nil {
		s, _ := task.Status(ctx)
		status = mapTaskStatus(s.Status)
		pid = task.Pid()
	}

	return Container{
		ID:     id,
		Image:  imageName,
		Labels: labels,
		Status: status,
		Pid:    pid,
	}
}

func (m *containerdManager) toTask(task taskAdapter) Task {
	status, _ := task.Status(context.Background())
	return Task{
		ID:     task.ID(),
		Pid:    task.Pid(),
		Status: mapTaskStatus(status.Status),
	}
}

func mapTaskStatus(status containerd.ProcessStatus) TaskStatus {
	switch status {
	case containerd.Created:
		return StatusCreated
	case containerd.Running:
		return StatusRunning
	case containerd.Stopped:
		return StatusStopped
	case containerd.Paused:
		return StatusPaused
	case containerd.Pausing:
		return StatusPausing
	default:
		return StatusUnknown
	}
}

func parseSignal(s string) (syscall.Signal, error) {
	if s == "" {
		return syscall.SIGTERM, nil
	}

	s = strings.ToUpper(s)
	if !strings.HasPrefix(s, "SIG") {
		s = "SIG" + s
	}

	sig, ok := signalNames[s]
	if !ok {
		return 0, fmt.Errorf("containerdw: unsupported signal %q", s)
	}
	return sig, nil
}

var signalNames = map[string]syscall.Signal{
	"SIGABRT":  syscall.SIGABRT,
	"SIGALRM":  syscall.SIGALRM,
	"SIGBUS":   syscall.SIGBUS,
	"SIGCHLD":  syscall.SIGCHLD,
	"SIGCONT":  syscall.SIGCONT,
	"SIGFPE":   syscall.SIGFPE,
	"SIGHUP":   syscall.SIGHUP,
	"SIGILL":   syscall.SIGILL,
	"SIGINT":   syscall.SIGINT,
	"SIGKILL":  syscall.SIGKILL,
	"SIGPIPE":  syscall.SIGPIPE,
	"SIGQUIT":  syscall.SIGQUIT,
	"SIGSEGV":  syscall.SIGSEGV,
	"SIGSTOP":  syscall.SIGSTOP,
	"SIGTERM":  syscall.SIGTERM,
	"SIGTRAP":  syscall.SIGTRAP,
	"SIGTSTP":  syscall.SIGTSTP,
	"SIGTTIN":  syscall.SIGTTIN,
	"SIGTTOU":  syscall.SIGTTOU,
	"SIGURG":   syscall.SIGURG,
	"SIGUSR1":  syscall.SIGUSR1,
	"SIGUSR2":  syscall.SIGUSR2,
	"SIGWINCH": syscall.SIGWINCH,
}

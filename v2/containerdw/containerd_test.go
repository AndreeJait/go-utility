package containerdw

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// fakeClient implements clientAdapter for unit tests.
type fakeClient struct {
	versionErr    error
	versionResult containerd.Version

	containersErr    error
	containersResult []containerAdapter

	loadContainerErr    error
	loadContainerResult containerAdapter
	lastLoadID          string

	newContainerErr    error
	newContainerResult containerAdapter

	pullErr    error
	pullResult containerd.Image

	getImageErr    error
	getImageResult containerd.Image

	listImagesErr    error
	listImagesResult []containerd.Image
}

func (f *fakeClient) Close() error { return nil }

func (f *fakeClient) Version(ctx context.Context) (containerd.Version, error) {
	return f.versionResult, f.versionErr
}

func (f *fakeClient) Pull(ctx context.Context, ref string, opts ...containerd.RemoteOpt) (containerd.Image, error) {
	return f.pullResult, f.pullErr
}

func (f *fakeClient) GetImage(ctx context.Context, ref string) (containerd.Image, error) {
	return f.getImageResult, f.getImageErr
}

func (f *fakeClient) ListImages(ctx context.Context, filters ...string) ([]containerd.Image, error) {
	return f.listImagesResult, f.listImagesErr
}

func (f *fakeClient) NewContainer(ctx context.Context, id string, opts ...containerd.NewContainerOpts) (containerAdapter, error) {
	return f.newContainerResult, f.newContainerErr
}

func (f *fakeClient) LoadContainer(ctx context.Context, id string) (containerAdapter, error) {
	f.lastLoadID = id
	return f.loadContainerResult, f.loadContainerErr
}

func (f *fakeClient) Containers(ctx context.Context, filters ...string) ([]containerAdapter, error) {
	return f.containersResult, f.containersErr
}

// fakeImage is a minimal containerd.Image stub.
type fakeImage struct {
	name   string
	labels map[string]string
}

func (f *fakeImage) Name() string               { return f.name }
func (f *fakeImage) Labels() map[string]string  { return f.labels }
func (f *fakeImage) Target() ocispec.Descriptor { return ocispec.Descriptor{} }
func (f *fakeImage) Unpack(ctx context.Context, snapshotterName string, opts ...containerd.UnpackOpt) error {
	return nil
}
func (f *fakeImage) RootFS(ctx context.Context) ([]digest.Digest, error) { return nil, nil }
func (f *fakeImage) Size(ctx context.Context) (int64, error)             { return 0, nil }
func (f *fakeImage) Usage(ctx context.Context, opts ...containerd.UsageOpt) (int64, error) {
	return 0, nil
}
func (f *fakeImage) Config(ctx context.Context) (ocispec.Descriptor, error) {
	return ocispec.Descriptor{}, nil
}
func (f *fakeImage) IsUnpacked(ctx context.Context, snapshotterName string) (bool, error) {
	return true, nil
}
func (f *fakeImage) ContentStore() content.Store                     { return nil }
func (f *fakeImage) Metadata() images.Image                          { return images.Image{Name: f.name} }
func (f *fakeImage) Platform() platforms.MatchComparer               { return nil }
func (f *fakeImage) Spec(ctx context.Context) (ocispec.Image, error) { return ocispec.Image{}, nil }

// fakeContainer implements containerAdapter for unit tests.
type fakeContainer struct {
	id            string
	image         containerd.Image
	labels        map[string]string
	task          taskAdapter
	taskErr       error
	newTaskErr    error
	newTaskResult taskAdapter
	deleteErr     error
}

func (f *fakeContainer) ID() string                                            { return f.id }
func (f *fakeContainer) Image(ctx context.Context) (containerd.Image, error)   { return f.image, nil }
func (f *fakeContainer) Labels(ctx context.Context) (map[string]string, error) { return f.labels, nil }
func (f *fakeContainer) NewTask(ctx context.Context, ioCreate cio.Creator, opts ...containerd.NewTaskOpts) (taskAdapter, error) {
	return f.newTaskResult, f.newTaskErr
}
func (f *fakeContainer) Task(ctx context.Context, attach cio.Attach) (taskAdapter, error) {
	if f.task == nil && f.taskErr == nil {
		return nil, errdefs.ErrNotFound
	}
	return f.task, f.taskErr
}
func (f *fakeContainer) Delete(ctx context.Context, opts ...containerd.DeleteOpts) error {
	return f.deleteErr
}

// fakeTask implements taskAdapter for unit tests.
type fakeTask struct {
	id        string
	pid       uint32
	status    containerd.Status
	startErr  error
	killErr   error
	waitCh    <-chan containerd.ExitStatus
	waitErr   error
	deleteErr error
}

func (f *fakeTask) ID() string                                            { return f.id }
func (f *fakeTask) Pid() uint32                                           { return f.pid }
func (f *fakeTask) Status(ctx context.Context) (containerd.Status, error) { return f.status, nil }
func (f *fakeTask) Start(ctx context.Context) error                       { return f.startErr }
func (f *fakeTask) Wait(ctx context.Context) (<-chan containerd.ExitStatus, error) {
	return f.waitCh, f.waitErr
}
func (f *fakeTask) Kill(ctx context.Context, signal syscall.Signal, opts ...containerd.KillOpts) error {
	return f.killErr
}
func (f *fakeTask) Delete(ctx context.Context, opts ...containerd.ProcessDeleteOpts) (*containerd.ExitStatus, error) {
	return nil, f.deleteErr
}

func newTestManager(client clientAdapter) *containerdManager {
	return &containerdManager{
		client:    client,
		namespace: "test",
		debug:     false,
	}
}

func TestNew_MissingConfig(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestVersion_Success(t *testing.T) {
	client := &fakeClient{
		versionResult: containerd.Version{Version: "2.0.0", Revision: "abc"},
	}
	m := newTestManager(client)
	v, err := m.Version(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Version != "2.0.0" {
		t.Fatalf("unexpected version: %v", v)
	}
}

func TestVersion_Error(t *testing.T) {
	client := &fakeClient{versionErr: errors.New("unreachable")}
	m := newTestManager(client)
	_, err := m.Version(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListContainers_Success(t *testing.T) {
	client := &fakeClient{
		containersResult: []containerAdapter{
			&fakeContainer{id: "c1", image: &fakeImage{name: "nginx"}, labels: map[string]string{"app": "web"}},
		},
	}
	m := newTestManager(client)
	out, err := m.ListContainers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].ID != "c1" {
		t.Fatalf("unexpected containers: %+v", out)
	}
}

func TestContainerStatus_Running(t *testing.T) {
	client := &fakeClient{
		loadContainerResult: &fakeContainer{
			id: "c1",
			task: &fakeTask{
				status: containerd.Status{Status: containerd.Running},
			},
		},
	}
	m := newTestManager(client)
	status, err := m.ContainerStatus(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != StatusRunning {
		t.Fatalf("expected running, got %v", status)
	}
}

func TestContainerStatus_StoppedWhenNoTask(t *testing.T) {
	client := &fakeClient{
		loadContainerResult: &fakeContainer{
			id:      "c1",
			taskErr: errdefs.ErrNotFound,
		},
	}
	m := newTestManager(client)
	status, err := m.ContainerStatus(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != StatusStopped {
		t.Fatalf("expected stopped, got %v", status)
	}
}

func TestStartContainer_Success(t *testing.T) {
	client := &fakeClient{
		loadContainerResult: &fakeContainer{
			id:            "c1",
			newTaskResult: &fakeTask{id: "c1", pid: 42, status: containerd.Status{Status: containerd.Running}},
		},
	}
	m := newTestManager(client)
	task, err := m.StartContainer(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Pid != 42 {
		t.Fatalf("unexpected pid: %d", task.Pid)
	}
}

func TestStartContainer_NewTaskError(t *testing.T) {
	client := &fakeClient{
		loadContainerResult: &fakeContainer{
			id:         "c1",
			newTaskErr: errors.New("no runtime"),
		},
	}
	m := newTestManager(client)
	_, err := m.StartContainer(context.Background(), "c1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStopContainer_Success(t *testing.T) {
	ch := make(chan containerd.ExitStatus, 1)
	ch <- containerd.ExitStatus{}
	client := &fakeClient{
		loadContainerResult: &fakeContainer{
			id: "c1",
			task: &fakeTask{
				status: containerd.Status{Status: containerd.Running},
				waitCh: ch,
			},
		},
	}
	m := newTestManager(client)
	err := m.StopContainer(context.Background(), "c1", "SIGTERM", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopContainer_NoTask(t *testing.T) {
	client := &fakeClient{
		loadContainerResult: &fakeContainer{
			id:      "c1",
			taskErr: errdefs.ErrNotFound,
		},
	}
	m := newTestManager(client)
	err := m.StopContainer(context.Background(), "c1", "SIGTERM", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteContainer_Success(t *testing.T) {
	client := &fakeClient{
		loadContainerResult: &fakeContainer{id: "c1"},
	}
	m := newTestManager(client)
	err := m.DeleteContainer(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteContainer_NotFound(t *testing.T) {
	client := &fakeClient{
		loadContainerErr: errdefs.ErrNotFound,
	}
	m := newTestManager(client)
	err := m.DeleteContainer(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPullImage_MissingRef(t *testing.T) {
	m := newTestManager(&fakeClient{})
	_, err := m.PullImage(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateContainer_MissingID(t *testing.T) {
	m := newTestManager(&fakeClient{})
	_, err := m.CreateContainer(context.Background(), "", "nginx")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSignal_Default(t *testing.T) {
	sig, err := parseSignal("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig != syscall.SIGTERM {
		t.Fatalf("expected SIGTERM, got %v", sig)
	}
}

func TestParseSignal_ByName(t *testing.T) {
	for _, name := range []string{"SIGKILL", "KILL", "kill"} {
		sig, err := parseSignal(name)
		if err != nil {
			t.Fatalf("parseSignal(%q): %v", name, err)
		}
		if sig != syscall.SIGKILL {
			t.Fatalf("expected SIGKILL, got %v", sig)
		}
	}
}

func TestParseSignal_Invalid(t *testing.T) {
	_, err := parseSignal("SIGFAKE")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMapTaskStatus(t *testing.T) {
	cases := []struct {
		in   containerd.ProcessStatus
		want TaskStatus
	}{
		{containerd.Running, StatusRunning},
		{containerd.Created, StatusCreated},
		{containerd.Stopped, StatusStopped},
		{containerd.Paused, StatusPaused},
		{containerd.Pausing, StatusPausing},
		{containerd.ProcessStatus("weird"), StatusUnknown},
	}
	for _, c := range cases {
		got := mapTaskStatus(c.in)
		if got != c.want {
			t.Errorf("mapTaskStatus(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// fakeContainerd is a minimal implementation used to verify interface shape.
type fakeContainerd struct{}

func (f *fakeContainerd) Version(ctx context.Context) (Version, error) { return Version{}, nil }
func (f *fakeContainerd) PullImage(ctx context.Context, ref string) (Image, error) {
	return Image{}, nil
}
func (f *fakeContainerd) GetImage(ctx context.Context, ref string) (Image, error) {
	return Image{}, nil
}
func (f *fakeContainerd) ListImages(ctx context.Context) ([]Image, error) { return nil, nil }
func (f *fakeContainerd) CreateContainer(ctx context.Context, id, imageRef string) (Container, error) {
	return Container{}, nil
}
func (f *fakeContainerd) LoadContainer(ctx context.Context, id string) (Container, error) {
	return Container{}, nil
}
func (f *fakeContainerd) ListContainers(ctx context.Context) ([]Container, error) { return nil, nil }
func (f *fakeContainerd) StartContainer(ctx context.Context, id string) (Task, error) {
	return Task{}, nil
}
func (f *fakeContainerd) StopContainer(ctx context.Context, id string, signal string, timeout time.Duration) error {
	return nil
}
func (f *fakeContainerd) DeleteContainer(ctx context.Context, id string) error { return nil }
func (f *fakeContainerd) ContainerStatus(ctx context.Context, id string) (TaskStatus, error) {
	return StatusUnknown, nil
}
func (f *fakeContainerd) ListNetworks(ctx context.Context) ([]Network, error) { return nil, nil }
func (f *fakeContainerd) CreateNetwork(ctx context.Context, name, driver string, options NetworkOptions) error {
	return nil
}
func (f *fakeContainerd) RemoveNetwork(ctx context.Context, name string) error { return nil }
func (f *fakeContainerd) PruneImages(ctx context.Context) (PruneResult, error) {
	return PruneResult{}, nil
}
func (f *fakeContainerd) PruneContainers(ctx context.Context) (PruneResult, error) {
	return PruneResult{}, nil
}
func (f *fakeContainerd) Close() error { return nil }

func TestContainerdInterface_Compliance(t *testing.T) {
	var _ Containerd = (*fakeContainerd)(nil)
}

func TestListNetworks_MissingNerdctl(t *testing.T) {
	m := newTestManager(&fakeClient{})
	_, err := m.ListNetworks(context.Background())
	if err == nil {
		t.Fatal("expected error when nerdctl is missing")
	}
}

func TestCreateNetwork_MissingName(t *testing.T) {
	m := newTestManager(&fakeClient{})
	err := m.CreateNetwork(context.Background(), "", "bridge", NetworkOptions{})
	if err == nil {
		t.Fatal("expected error for empty network name")
	}
}

func TestRemoveNetwork_MissingName(t *testing.T) {
	m := newTestManager(&fakeClient{})
	err := m.RemoveNetwork(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty network name")
	}
}

func TestPruneImages_Mocked(t *testing.T) {
	client := &fakeClient{
		listImagesResult: []containerd.Image{
			&fakeImage{name: "unused:latest"},
			&fakeImage{name: "used:latest"},
		},
		containersResult: []containerAdapter{
			&fakeContainer{id: "c1", image: &fakeImage{name: "used:latest"}},
		},
	}
	m := newTestManager(client)

	_, err := m.PruneImages(context.Background())
	if err == nil {
		t.Fatal("expected error because prune images requires a real containerd client")
	}
}

func TestPruneContainers_Mocked(t *testing.T) {
	client := &fakeClient{
		containersResult: []containerAdapter{
			&fakeContainer{id: "running", task: &fakeTask{status: containerd.Status{Status: containerd.Running}}},
			&fakeContainer{id: "stopped"},
		},
	}
	m := newTestManager(client)

	res, err := m.PruneContainers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Deleted != 1 {
		t.Fatalf("expected 1 deletion, got %d", res.Deleted)
	}
}

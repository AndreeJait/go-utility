package goroutine

import "context"

type OptionKey string

// list options supported
const (
	// OptionKeyWorker total allowed worker
	OptionKeyWorker OptionKey = "worker"

	// OptionKeyBlockError stop all process if there error exists in one async process
	OptionKeyBlockError OptionKey = "block_error"

	// OptionKeyBatchProcess batch size for batch input splitting
	OptionKeyBatchProcess OptionKey = "batch_process"

	// OptionKeyPanicAsError convert panic inside step into error (default: true)
	OptionKeyPanicAsError OptionKey = "panic_as_error"

	// OptionKeyStepTimeoutMs apply timeout per job execution in milliseconds (default: 0 = no timeout)
	OptionKeyStepTimeoutMs OptionKey = "step_timeout_ms"
)

type Func func(ctx context.Context, in interface{}) (resp interface{}, err error)

// BatchCollector merges all batch responses into one response output (key = stepKey)
type BatchCollector func(ctx context.Context, stepKey string, batchResults []Response) (output interface{}, err error)

type Response struct {
	Output    interface{}
	Err       error
	IsExecute bool
}

type Option struct {
	Name  OptionKey
	Value interface{}
}

type Goroutine interface {
	AddStep(key string, f Func)

	AddInput(key string, in interface{})

	AddBatchInput(key string, in interface{})

	AddCollectorBatch(key string, collector BatchCollector)

	AddOptions(options ...Option)

	IsProcessSuccess() bool

	GetFailedProcess() []string

	GetSuccessProcess() []string

	Run(ctx context.Context) map[string]Response
}

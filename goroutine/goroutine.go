package goroutine

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type option struct {
	HasWorker bool
	Worker    int32

	BlockError bool

	IsBatchProcess bool
	BatchProcess   int32

	PanicAsError  bool
	StepTimeoutMs int32
}

type goroutine struct {
	mu sync.Mutex

	input map[string]interface{}

	// batchInput is item-level. We'll split into chunks at Run().
	batchInput map[string][]interface{}

	activeOption option

	steps map[string]Func

	successProcess []string
	failedProcess  []string

	outputResp map[string]Response

	batchCollectors map[string]BatchCollector
	batchResults    map[string][]Response
}

type job struct {
	processKey string
	stepKey    string
	input      interface{}
	fn         Func
}

func (g *goroutine) AddStep(key string, f Func) {
	if key == "" || f == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.steps[key] = f
}

func (g *goroutine) AddInput(key string, in interface{}) {
	if key == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.input[key] = in
}

func (g *goroutine) AddBatchInput(key string, in interface{}) {
	if key == "" || in == nil {
		return
	}

	// Accept:
	// - slice/array: []T / [N]T -> append each element as interface{}
	// - single value: treat as one item batch
	v := reflect.ValueOf(in)
	k := v.Kind()

	g.mu.Lock()
	defer g.mu.Unlock()

	switch k {
	case reflect.Slice, reflect.Array:
		n := v.Len()
		for i := 0; i < n; i++ {
			g.batchInput[key] = append(g.batchInput[key], v.Index(i).Interface())
		}
	default:
		g.batchInput[key] = append(g.batchInput[key], in)
	}
}

func (g *goroutine) AddOptions(options ...Option) {
	_, _ = g.setOptions(options...)
}

func (g *goroutine) AddCollectorBatch(key string, collector BatchCollector) {
	if key == "" || collector == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.batchCollectors[key] = collector
}

func (g *goroutine) IsProcessSuccess() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.failedProcess) == 0
}

func (g *goroutine) GetFailedProcess() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]string, len(g.failedProcess))
	copy(out, g.failedProcess)
	return out
}

func (g *goroutine) GetSuccessProcess() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]string, len(g.successProcess))
	copy(out, g.successProcess)
	return out
}

func (g *goroutine) Run(ctx context.Context) map[string]Response {
	if ctx == nil {
		ctx = context.Background()
	}

	// snapshot state to avoid holding mutex during whole run
	g.mu.Lock()
	steps := make(map[string]Func, len(g.steps))
	for k, f := range g.steps {
		steps[k] = f
	}
	input := make(map[string]interface{}, len(g.input))
	for k, v := range g.input {
		input[k] = v
	}
	batchInput := make(map[string][]interface{}, len(g.batchInput))
	for k, v := range g.batchInput {
		cp := make([]interface{}, len(v))
		copy(cp, v)
		batchInput[k] = cp
	}
	opt := g.activeOption

	// reset run results
	g.successProcess = g.successProcess[:0]
	g.failedProcess = g.failedProcess[:0]
	g.outputResp = make(map[string]Response)
	g.batchResults = make(map[string][]Response)
	g.mu.Unlock()

	// workers default
	workers := int32(runtime.GOMAXPROCS(0))
	if opt.HasWorker && opt.Worker > 0 {
		workers = opt.Worker
	}
	if workers <= 0 {
		workers = 1
	}

	// if BlockError -> cancel on first error
	runCtx := ctx
	cancel := func() {}
	if opt.BlockError {
		runCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	// build jobs (including batch splitting)
	jobs := g.buildJobs(steps, input, batchInput, opt)

	for _, jb := range jobs {
		// stop scheduling new jobs if block-error and already canceled
		if opt.BlockError {
			select {
			case <-runCtx.Done():
				// mark remaining jobs as not executed
				g.recordResponse(jb.processKey, Response{
					IsExecute: false,
					Err:       errors.Wrap(runCtx.Err(), "canceled before execute"),
				})
				continue
			default:
			}
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(j job) {
			defer func() {
				<-sem
				wg.Done()
			}()

			resp := g.executeJob(runCtx, j, opt)

			g.recordResponse(j.processKey, resp)

			// if error and block-error: cancel context ASAP
			if opt.BlockError && resp.Err != nil {
				cancel()
			}
		}(jb)
	}

	wg.Wait()

	g.applyBatchCollectors(runCtx)

	// return a copy so caller can't mutate internal map
	g.mu.Lock()
	out := make(map[string]Response, len(g.outputResp))
	for k, v := range g.outputResp {
		out[k] = v
	}
	g.mu.Unlock()

	return out
}

func (g *goroutine) buildJobs(
	steps map[string]Func,
	input map[string]interface{},
	batchInput map[string][]interface{},
	opt option,
) []job {
	var jobs []job

	for stepKey, fn := range steps {
		// 1) batch input
		if items, ok := batchInput[stepKey]; ok && len(items) > 0 {
			if opt.IsBatchProcess && opt.BatchProcess > 0 {
				chunks := chunkInterfaces(items, int(opt.BatchProcess))
				for i, c := range chunks {
					pk := fmt.Sprintf("%s#batch_%d", stepKey, i+1)
					jobs = append(jobs, job{
						processKey: pk,
						stepKey:    stepKey,
						input:      c, // []interface{}
						fn:         fn,
					})
				}
			} else {
				pk := fmt.Sprintf("%s#batch_all", stepKey)
				jobs = append(jobs, job{
					processKey: pk,
					stepKey:    stepKey,
					input:      items, // []interface{}
					fn:         fn,
				})
			}
			continue
		}

		// 2) single input
		if in, ok := input[stepKey]; ok {
			jobs = append(jobs, job{
				processKey: stepKey,
				stepKey:    stepKey,
				input:      in,
				fn:         fn,
			})
			continue
		}

		// 3) no input -> still run once with nil
		jobs = append(jobs, job{
			processKey: stepKey,
			stepKey:    stepKey,
			input:      nil,
			fn:         fn,
		})
	}

	return jobs
}

func (g *goroutine) executeJob(ctx context.Context, j job, opt option) (resp Response) {
	resp.IsExecute = true

	// apply per-job timeout if configured
	runCtx := ctx
	cancel := func() {}
	if opt.StepTimeoutMs > 0 {
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(opt.StepTimeoutMs)*time.Millisecond)
	}
	defer cancel()

	// panic safe
	if opt.PanicAsError {
		defer func() {
			if r := recover(); r != nil {
				resp.Err = errors.Errorf("panic in step %s (%s): %v", j.stepKey, j.processKey, r)
			}
		}()
	}

	out, err := j.fn(runCtx, j.input)
	resp.Output = out
	resp.Err = err
	return resp
}

func (g *goroutine) recordResponse(processKey string, resp Response) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.outputResp[processKey] = resp

	if resp.IsExecute && resp.Err == nil {
		g.successProcess = append(g.successProcess, processKey)
	}
	if resp.IsExecute && resp.Err != nil {
		g.failedProcess = append(g.failedProcess, processKey)
	}

	// NEW: if this is a batch key, store it for collector usage
	if stepKey, idx, ok := parseBatchProcessKey(processKey); ok {
		if len(g.batchResults[stepKey]) < idx {
			newSlice := make([]Response, idx)
			copy(newSlice, g.batchResults[stepKey])
			g.batchResults[stepKey] = newSlice
		}
		g.batchResults[stepKey][idx-1] = resp
	}
}

func (g *goroutine) applyBatchCollectors(ctx context.Context) {
	g.mu.Lock()
	collectors := make(map[string]BatchCollector, len(g.batchCollectors))
	for k, v := range g.batchCollectors {
		collectors[k] = v
	}

	// snapshot batchResults
	batchResults := make(map[string][]Response, len(g.batchResults))
	for k, v := range g.batchResults {
		cp := make([]Response, len(v))
		copy(cp, v)
		batchResults[k] = cp
	}

	// snapshot keys so we can delete per-batch keys safely
	keys := make([]string, 0, len(g.outputResp))
	for k := range g.outputResp {
		keys = append(keys, k)
	}
	g.mu.Unlock()

	for stepKey, collector := range collectors {
		results, ok := batchResults[stepKey]
		if !ok || len(results) == 0 {
			// collector set but no batches ran -> still run once with empty slice (optional)
			out, err := collector(ctx, stepKey, []Response{})
			g.mu.Lock()
			g.outputResp[stepKey] = Response{Output: out, Err: err, IsExecute: true}
			g.mu.Unlock()
			continue
		}

		// merge
		out, err := collector(ctx, stepKey, results)

		g.mu.Lock()
		// put merged response using key = stepKey
		g.outputResp[stepKey] = Response{
			Output:    out,
			Err:       err,
			IsExecute: true,
		}

		// IMPORTANT: remove per-batch keys for this stepKey (so caller sees only the merged one)
		prefix := stepKey + "#batch_"
		for _, k := range keys {
			if strings.HasPrefix(k, prefix) {
				delete(g.outputResp, k)
			}
		}

		// optional: you may also want to clean success/failed slices, but usually not required.
		// if you want "public" success/failed to reflect merged view only, tell me and I'll adjust.
		g.mu.Unlock()
	}
}

func NewGoroutine(opts ...Option) (Goroutine, error) {
	var g = goroutine{
		input:           make(map[string]interface{}),
		batchInput:      make(map[string][]interface{}),
		activeOption:    option{PanicAsError: true}, // default true
		steps:           make(map[string]Func),
		successProcess:  make([]string, 0),
		failedProcess:   make([]string, 0),
		outputResp:      make(map[string]Response),
		batchCollectors: make(map[string]BatchCollector),
		batchResults:    make(map[string][]Response),
	}

	_, err := g.setOptions(opts...)
	if err != nil {
		return nil, err
	}

	return &g, nil
}

func (g *goroutine) isNumber(val interface{}) (int32, bool) {
	switch v := val.(type) {
	case int:
		return int32(v), true
	case float64:
		return int32(v), true
	case int32:
		return v, true
	case int64:
		return int32(v), true
	case float32:
		return int32(v), true
	case uint:
		return int32(v), true
	case uint32:
		return int32(v), true
	case uint64:
		return int32(v), true
	}
	return 0, false
}

func (g *goroutine) isBoolean(val interface{}) (bool, bool) {
	switch v := val.(type) {
	case bool:
		return v, true
	}
	return false, false
}

func (g *goroutine) setOptions(opts ...Option) (map[string]interface{}, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	var activeOptions = make(map[string]interface{})
	for _, opt := range opts {
		switch opt.Name {
		case OptionKeyWorker:
			num, ok := g.isNumber(opt.Value)
			if !ok || num <= 0 {
				return nil, errors.New("invalid option value for worker (must be > 0)")
			}
			g.activeOption.HasWorker = true
			g.activeOption.Worker = num

		case OptionKeyBlockError:
			isBlock, ok := g.isBoolean(opt.Value)
			if !ok {
				return nil, errors.New("invalid option value for block_error (must be bool)")
			}
			g.activeOption.BlockError = isBlock

		case OptionKeyBatchProcess:
			totalBatch, ok := g.isNumber(opt.Value)
			if !ok || totalBatch <= 0 {
				return nil, errors.New("invalid option value for batch_process (must be > 0)")
			}
			g.activeOption.IsBatchProcess = true
			g.activeOption.BatchProcess = totalBatch

		case OptionKeyPanicAsError:
			isOn, ok := g.isBoolean(opt.Value)
			if !ok {
				return nil, errors.New("invalid option value for panic_as_error (must be bool)")
			}
			g.activeOption.PanicAsError = isOn

		case OptionKeyStepTimeoutMs:
			ms, ok := g.isNumber(opt.Value)
			if !ok || ms < 0 {
				return nil, errors.New("invalid option value for step_timeout_ms (must be >= 0)")
			}
			g.activeOption.StepTimeoutMs = ms

		default:
			// ignore unknown options (or return error if you prefer strict)
			activeOptions[string(opt.Name)] = opt.Value
		}
	}
	return activeOptions, nil
}

func chunkInterfaces(items []interface{}, size int) [][]interface{} {
	if size <= 0 || len(items) == 0 {
		return [][]interface{}{items}
	}
	var chunks [][]interface{}
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunk := make([]interface{}, end-i)
		copy(chunk, items[i:end])
		chunks = append(chunks, chunk)
	}
	return chunks
}

func parseBatchProcessKey(processKey string) (stepKey string, batchIndex int, ok bool) {
	// expects: "<stepKey>#batch_<n>"
	const sep = "#batch_"
	pos := strings.Index(processKey, sep)
	if pos < 0 {
		return "", 0, false
	}
	stepKey = processKey[:pos]
	numStr := processKey[pos+len(sep):]
	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return stepKey, n, true
}

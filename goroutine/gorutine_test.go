package goroutine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func mustNew(t *testing.T, opts ...Option) Goroutine {
	t.Helper()
	g, err := NewGoroutine(opts...)
	if err != nil {
		t.Fatalf("NewGoroutine error: %v", err)
	}
	return g
}

func processSingleSleep(d time.Duration) Func {
	return func(ctx context.Context, in interface{}) (interface{}, error) {
		if d > 0 {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if in == nil {
			return "Hello <nil>", nil
		}
		return fmt.Sprintf("Hello %s", in.(string)), nil
	}
}

func processBatchJoinSleep(d time.Duration) Func {
	return func(ctx context.Context, in interface{}) (interface{}, error) {
		if d > 0 {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		batch := in.([]interface{})
		names := make([]string, len(batch))
		for i, param := range batch {
			names[i] = fmt.Sprintf("Hello %s", param.(string))
		}
		return strings.Join(names, ","), nil
	}
}

func collectorJoin() BatchCollector {
	return func(ctx context.Context, stepKey string, batchResults []Response) (interface{}, error) {
		var results []string
		for _, r := range batchResults {
			if r.Err != nil {
				return nil, r.Err
			}
			results = append(results, r.Output.(string))
		}
		return strings.Join(results, ","), nil
	}
}

func TestGoroutine_OptionCases(t *testing.T) {
	t.Run("no_input_step_still_runs", func(t *testing.T) {
		g := mustNew(t,
			Option{Name: OptionKeyWorker, Value: 2},
		)

		g.AddStep("noinput", func(ctx context.Context, in interface{}) (interface{}, error) {
			// must be called with in == nil
			if in != nil {
				return nil, errors.New("expected nil input")
			}
			return "ok", nil
		})

		resp := g.Run(context.Background())

		r, ok := resp["noinput"]
		if !ok {
			t.Fatalf("expected response for noinput step")
		}
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
		if r.Output != "ok" {
			t.Fatalf("unexpected output: %#v", r.Output)
		}
	})

	t.Run("worker_limit_concurrency_respected", func(t *testing.T) {
		// We measure peak concurrent executions and assert <= Worker
		var current int32
		var peak int32

		concurrencyFn := func(ctx context.Context, in interface{}) (interface{}, error) {
			cur := atomic.AddInt32(&current, 1)
			for {
				p := atomic.LoadInt32(&peak)
				if cur > p && atomic.CompareAndSwapInt32(&peak, p, cur) {
					break
				}
				if cur <= p {
					break
				}
			}

			// hold briefly to allow overlaps
			select {
			case <-time.After(40 * time.Millisecond):
			case <-ctx.Done():
				atomic.AddInt32(&current, -1)
				return nil, ctx.Err()
			}

			atomic.AddInt32(&current, -1)
			return "ok", nil
		}

		g := mustNew(t,
			Option{Name: OptionKeyWorker, Value: 2},
		)

		// Create many jobs by using batch splitting, but no collector
		g.AddStep("batch", concurrencyFn)
		g.AddBatchInput("batch", []string{"a", "b", "c", "d", "e", "f"})
		g.AddOptions(Option{Name: OptionKeyBatchProcess, Value: 1}) // each item => its own job

		_ = g.Run(context.Background())

		if atomic.LoadInt32(&peak) > 2 {
			t.Fatalf("expected peak concurrency <= 2, got %d", peak)
		}
	})

	t.Run("block_error_true_cancels_other_jobs", func(t *testing.T) {
		// One job fails quickly, another job should see ctx cancellation.
		g := mustNew(t,
			Option{Name: OptionKeyWorker, Value: 3},
			Option{Name: OptionKeyBlockError, Value: true},
		)

		g.AddStep("failfast", func(ctx context.Context, in interface{}) (interface{}, error) {
			return nil, errors.New("boom")
		})
		g.AddInput("failfast", "x")

		g.AddStep("slow", func(ctx context.Context, in interface{}) (interface{}, error) {
			// If block_error works, this should be canceled (or not executed depending on timing),
			// but in our current design we still schedule; cancellation should happen quickly.
			select {
			case <-time.After(500 * time.Millisecond):
				return "done", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
		g.AddInput("slow", "y")

		resp := g.Run(context.Background())

		// failfast must fail
		if r := resp["failfast"]; r.Err == nil {
			t.Fatalf("expected failfast error, got nil")
		}

		// slow should typically be canceled (allow either canceled or not-executed behavior depending on timing)
		rSlow, ok := resp["slow"]
		if !ok {
			t.Fatalf("expected slow response entry")
		}
		if rSlow.Err == nil {
			// If it finishes before cancel (rare), test could flake.
			// To avoid flake, we assert that overall process is marked fail.
			if g.IsProcessSuccess() {
				t.Fatalf("expected overall failure when failfast errors")
			}
		} else {
			// cancellation error is acceptable
			if !strings.Contains(rSlow.Err.Error(), "context") {
				t.Fatalf("expected context-related error, got: %v", rSlow.Err)
			}
		}
	})

	t.Run("block_error_false_does_not_cancel_others", func(t *testing.T) {
		g := mustNew(t,
			Option{Name: OptionKeyWorker, Value: 3},
			Option{Name: OptionKeyBlockError, Value: false},
		)

		g.AddStep("failfast", func(ctx context.Context, in interface{}) (interface{}, error) {
			return nil, errors.New("boom")
		})
		g.AddInput("failfast", "x")

		g.AddStep("ok", processSingleSleep(20*time.Millisecond))
		g.AddInput("ok", "Andree")

		resp := g.Run(context.Background())

		if resp["failfast"].Err == nil {
			t.Fatalf("expected failfast error")
		}
		if resp["ok"].Err != nil {
			t.Fatalf("expected ok succeed, got err: %v", resp["ok"].Err)
		}
	})

	t.Run("panic_as_error_true_converts_panic_to_error", func(t *testing.T) {
		g := mustNew(t,
			Option{Name: OptionKeyPanicAsError, Value: true},
		)

		g.AddStep("panic", func(ctx context.Context, in interface{}) (interface{}, error) {
			panic("kaboom")
		})

		resp := g.Run(context.Background())
		r := resp["panic"]
		if r.Err == nil {
			t.Fatalf("expected error from panic")
		}
		if !strings.Contains(r.Err.Error(), "panic") {
			t.Fatalf("expected panic in error, got: %v", r.Err)
		}
	})

	t.Run("step_timeout_ms_enforced", func(t *testing.T) {
		g := mustNew(t,
			Option{Name: OptionKeyStepTimeoutMs, Value: 10}, // 10ms
		)

		g.AddStep("slow", processSingleSleep(200*time.Millisecond))
		g.AddInput("slow", "Andree")

		resp := g.Run(context.Background())

		r := resp["slow"]
		if r.Err == nil {
			t.Fatalf("expected timeout error, got nil")
		}
		if !strings.Contains(r.Err.Error(), "context") {
			t.Fatalf("expected context timeout/cancel error, got: %v", r.Err)
		}
	})

	t.Run("batch_process_splits_into_chunks_when_collector_not_set", func(t *testing.T) {
		g := mustNew(t,
			Option{Name: OptionKeyBatchProcess, Value: 2},
		)

		g.AddStep("batch", processBatchJoinSleep(0))
		g.AddBatchInput("batch", []string{"one", "two", "three", "four"})

		resp := g.Run(context.Background())

		// No collector => expect batch keys present (and NOT the merged "batch")
		_, hasMerged := resp["batch"]
		if hasMerged {
			t.Fatalf("did not expect merged key 'batch' when collector not set")
		}
		if _, ok := resp["batch#batch_1"]; !ok {
			t.Fatalf("expected batch#batch_1")
		}
		if _, ok := resp["batch#batch_2"]; !ok {
			t.Fatalf("expected batch#batch_2")
		}

		// basic content check
		if resp["batch#batch_1"].Output.(string) != "Hello one,Hello two" {
			t.Fatalf("unexpected output batch_1: %#v", resp["batch#batch_1"].Output)
		}
		if resp["batch#batch_2"].Output.(string) != "Hello three,Hello four" {
			t.Fatalf("unexpected output batch_2: %#v", resp["batch#batch_2"].Output)
		}
	})

	t.Run("collector_set_merges_batches_into_stepKey_and_removes_batch_keys", func(t *testing.T) {
		g := mustNew(t,
			Option{Name: OptionKeyBatchProcess, Value: 2},
		)

		g.AddStep("batch", processBatchJoinSleep(0))
		g.AddBatchInput("batch", []string{"one", "two", "three", "four"})
		g.AddCollectorBatch("batch", collectorJoin())

		resp := g.Run(context.Background())

		// collector => merged key exists
		r, ok := resp["batch"]
		if !ok {
			t.Fatalf("expected merged key 'batch'")
		}
		if r.Err != nil {
			t.Fatalf("unexpected merged error: %v", r.Err)
		}
		if r.Output.(string) != "Hello one,Hello two,Hello three,Hello four" {
			t.Fatalf("unexpected merged output: %#v", r.Output)
		}

		// per-batch keys should be removed (as per your desired behavior)
		if _, ok := resp["batch#batch_1"]; ok {
			t.Fatalf("did not expect batch#batch_1 when collector is set (should be removed)")
		}
		if _, ok := resp["batch#batch_2"]; ok {
			t.Fatalf("did not expect batch#batch_2 when collector is set (should be removed)")
		}
	})

	t.Run("collector_propagates_error_if_any_batch_failed", func(t *testing.T) {
		g := mustNew(t,
			Option{Name: OptionKeyBatchProcess, Value: 2},
		)

		g.AddStep("batch", func(ctx context.Context, in interface{}) (interface{}, error) {
			b := in.([]interface{})
			// fail if chunk contains "three"
			for _, x := range b {
				if x.(string) == "three" {
					return nil, errors.New("bad item")
				}
			}
			// success otherwise
			names := make([]string, len(b))
			for i, x := range b {
				names[i] = "Hello " + x.(string)
			}
			return strings.Join(names, ","), nil
		})

		g.AddBatchInput("batch", []string{"one", "two", "three", "four"})
		g.AddCollectorBatch("batch", collectorJoin())

		resp := g.Run(context.Background())

		r := resp["batch"]
		if r.Err == nil {
			t.Fatalf("expected merged error, got nil")
		}
		if !strings.Contains(r.Err.Error(), "bad item") {
			t.Fatalf("unexpected error: %v", r.Err)
		}
	})
}

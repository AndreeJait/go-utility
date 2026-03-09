package spanw

import (
	"context"
	"reflect"
	"runtime"
	"strings"
	"time"
)

type contextKey string

const (
	spanChainKey   contextKey = "x-span-chain"
	processTimeKey contextKey = "x-total-process-time"
)

// MetricFuncType defines the signature for custom metric functions (e.g., Prometheus, StatsD).
// These functions are automatically invoked at the end of a span's execution.
type MetricFuncType func(ctx context.Context, funcName string, duration time.Duration)

// SpanConfig holds the global configuration for the spanw package.
type SpanConfig struct {
	// Enabled is a flag to turn tracing on or off globally.
	Enabled bool
}

var (
	globalConfig  *SpanConfig
	globalMetrics []MetricFuncType
)

// Init initializes the span configuration and registers variadic custom metric functions.
// It should be called once when the application starts (ideally alongside logw.Init).
func Init(cfg *SpanConfig, metrics ...MetricFuncType) {
	if cfg == nil {
		cfg = &SpanConfig{Enabled: true}
	}
	globalConfig = cfg
	globalMetrics = metrics
}

// GetRealFuncName dynamically extracts the actual function name based on the provided function parameter.
// It returns the full function name including its package path.
func GetRealFuncName(fn any) string {
	if fn == nil {
		return "unknown_func"
	}

	val := reflect.ValueOf(fn)
	if val.Kind() != reflect.Func {
		return "not_a_func"
	}

	pc := val.Pointer()
	runtimeFunc := runtime.FuncForPC(pc)
	if runtimeFunc == nil {
		return "unknown_func"
	}

	return runtimeFunc.Name()
}

// Start begins a new tracing span and returns a derived context along with a finish closure.
// The finish closure must be deferred to calculate the execution duration and trigger the registered metric functions.
//
// Usage example:
//
//	ctx, finish := spanw.Start(ctx, spanw.GetRealFuncName(u.Register))
//	defer finish()
func Start(ctx context.Context, funcName string) (context.Context, func()) {
	if ctx == nil {
		ctx = context.Background()
	}

	// If tracing is disabled globally, return the original context and a no-op function.
	if globalConfig != nil && !globalConfig.Enabled {
		return ctx, func() {}
	}

	// 1. Record the start time.
	startTime := time.Now()

	// 2. Retrieve the existing chain and append the new function name.
	existingChain, _ := ctx.Value(spanChainKey).([]string)
	newChain := make([]string, len(existingChain), len(existingChain)+1)
	copy(newChain, existingChain)
	newChain = append(newChain, funcName)

	newCtx := context.WithValue(ctx, spanChainKey, newChain)

	// 3. Return the new context and the finish closure.
	return newCtx, func() {
		// Calculate the total execution time.
		duration := time.Since(startTime)

		// Inject x-total-process-time into a specific metric context.
		metricCtx := context.WithValue(newCtx, processTimeKey, duration)

		// Execute all custom metric functions registered during Init.
		for _, metric := range globalMetrics {
			metric(metricCtx, funcName, duration)
		}
	}
}

// GetChain returns a slice containing the sequence of executed function names in the current context.
func GetChain(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	chain, _ := ctx.Value(spanChainKey).([]string)
	return chain
}

// GetTraceString returns a formatted string of the execution chain.
// Example: "Handler.Register -> Usecase.Register -> Repository.Insert"
func GetTraceString(ctx context.Context) string {
	chain := GetChain(ctx)
	if len(chain) == 0 {
		return ""
	}
	return strings.Join(chain, " -> ")
}

// GetCurrentFunc returns the name of the last function recorded in the current context chain.
func GetCurrentFunc(ctx context.Context) string {
	chain := GetChain(ctx)
	if len(chain) == 0 {
		return ""
	}
	return chain[len(chain)-1]
}

// GetTotalProcessTime retrieves the execution duration from the context.
// Note: This will only contain a valid duration if called from within a MetricFuncType after the span finishes.
func GetTotalProcessTime(ctx context.Context) time.Duration {
	if ctx == nil {
		return 0
	}
	if val, ok := ctx.Value(processTimeKey).(time.Duration); ok {
		return val
	}
	return 0
}

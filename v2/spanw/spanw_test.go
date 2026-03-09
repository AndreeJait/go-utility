package spanw

import (
	"context"
	"strings"
	"testing"
	"time"
)

// dummyFunction is used to test GetRealFuncName.
func dummyFunction() {}

func resetGlobals() {
	globalConfig = nil
	globalMetrics = nil
}

func TestGetRealFuncName(t *testing.T) {
	t.Run("Valid Function", func(t *testing.T) {
		name := GetRealFuncName(dummyFunction)
		if !strings.Contains(name, "dummyFunction") {
			t.Errorf("expected function name to contain 'dummyFunction', got: %s", name)
		}
	})

	t.Run("Nil Value", func(t *testing.T) {
		name := GetRealFuncName(nil)
		if name != "unknown_func" {
			t.Errorf("expected 'unknown_func', got: %s", name)
		}
	})

	t.Run("Non-Function Value", func(t *testing.T) {
		name := GetRealFuncName("just a string")
		if name != "not_a_func" {
			t.Errorf("expected 'not_a_func', got: %s", name)
		}
	})
}

func TestSpanChaining(t *testing.T) {
	resetGlobals()
	Init(&SpanConfig{Enabled: true})

	ctx := context.Background()

	// 1st Span
	ctx1, finish1 := Start(ctx, "Layer1")
	defer finish1()

	if current := GetCurrentFunc(ctx1); current != "Layer1" {
		t.Errorf("expected current func 'Layer1', got: %s", current)
	}

	// 2nd Span (Child of 1st)
	ctx2, finish2 := Start(ctx1, "Layer2")
	defer finish2()

	if trace := GetTraceString(ctx2); trace != "Layer1 -> Layer2" {
		t.Errorf("expected trace 'Layer1 -> Layer2', got: %s", trace)
	}

	chain := GetChain(ctx2)
	if len(chain) != 2 || chain[0] != "Layer1" || chain[1] != "Layer2" {
		t.Errorf("unexpected chain array: %v", chain)
	}
}

func TestSpanMetricsAndDuration(t *testing.T) {
	resetGlobals()

	metricCalled := false
	var capturedDuration time.Duration

	// Mock metric function
	mockMetric := func(ctx context.Context, funcName string, duration time.Duration) {
		metricCalled = true
		capturedDuration = duration

		// Verify that we can extract the process time from this specific metric context
		ctxTime := GetTotalProcessTime(ctx)
		if ctxTime != duration {
			t.Errorf("expected GetTotalProcessTime to be %v, got %v", duration, ctxTime)
		}

		if funcName != "TestFunc" {
			t.Errorf("expected funcName 'TestFunc', got: %s", funcName)
		}
	}

	Init(&SpanConfig{Enabled: true}, mockMetric)

	ctx := context.Background()
	_, finish := Start(ctx, "TestFunc")

	// Simulate some processing time
	time.Sleep(10 * time.Millisecond)

	// Trigger the finish closure
	finish()

	if !metricCalled {
		t.Fatal("expected custom metric function to be called, but it wasn't")
	}

	if capturedDuration < 10*time.Millisecond {
		t.Errorf("expected duration to be at least 10ms, got: %v", capturedDuration)
	}
}

func TestSpanDisabled(t *testing.T) {
	resetGlobals()

	metricCalled := false
	mockMetric := func(ctx context.Context, funcName string, duration time.Duration) {
		metricCalled = true
	}

	// Initialize with tracing disabled
	Init(&SpanConfig{Enabled: false}, mockMetric)

	ctx := context.Background()
	ctxDisabled, finish := Start(ctx, "DisabledFunc")

	finish()

	if metricCalled {
		t.Errorf("expected metric not to be called when tracing is disabled")
	}

	if GetCurrentFunc(ctxDisabled) != "" {
		t.Errorf("expected no chain to be formed when tracing is disabled")
	}
}

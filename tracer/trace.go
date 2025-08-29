package tracer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

type ctxKey string

const (
	ctxTraceIDKey ctxKey = "tracer.trace_id"
	ctxTraceKey   ctxKey = "tracer.trace"
	ctxSpanKey    ctxKey = "tracer.current_span"
)

type LogFunc func(ctx context.Context, level, msg string, fields map[string]any)

var logf LogFunc = func(context.Context, string, string, map[string]any) {}

func SetLogger(fn LogFunc) { logf = fn }

// ---------------- IDs ----------------

func newTraceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	out := make([]byte, 32)
	hex.Encode(out, b[:])
	return string(out)
}

// ---------------- Core structs ----------------

type Trace struct {
	TraceID  string
	Name     string
	Start    time.Time
	mu       sync.Mutex
	timeline []SpanRecord
	errCnt   int
}

type Span struct {
	t      *Trace
	Name   string
	Depth  int
	Func   string
	Caller string

	start  time.Time
	failed string // error string if Fail() called
}

type SpanRecord struct {
	Name       string `json:"name"`
	Depth      int    `json:"depth"`
	DurationMS int64  `json:"duration_ms"`
	Func       string `json:"func"`
	Caller     string `json:"caller"`
	Error      string `json:"error,omitempty"`
}

// ---------------- Public API ----------------

// StartTrace: create a trace and put it in ctx (used by middleware)
func StartTrace(ctx context.Context, name string) (context.Context, *Trace) {
	tid := TraceID(ctx)
	if tid == "" {
		tid = newTraceID()
		ctx = context.WithValue(ctx, ctxTraceIDKey, tid)
	}
	tr := &Trace{
		TraceID:  tid,
		Name:     name,
		Start:    time.Now(),
		timeline: make([]SpanRecord, 0, 8),
	}
	ctx = context.WithValue(ctx, ctxTraceKey, tr)
	return ctx, tr
}

// StartSpan: begins a child span; returns span and a new ctx that sets it as current
func StartSpan(ctx context.Context, name string) (*Span, context.Context) {
	tr := CurrentTrace(ctx)
	if tr == nil {
		// allow use without StartTrace; create ephemeral trace
		ctx, tr = StartTrace(ctx, "implicit")
	}
	parent := CurrentSpan(ctx)
	depth := 0
	if parent != nil {
		depth = parent.Depth + 1
	}
	fn, file, line := caller(2)

	sp := &Span{
		t:      tr,
		Name:   name,
		Depth:  depth,
		Func:   fn,
		Caller: fmtPos(file, line),
		start:  time.Now(),
	}
	ctx = context.WithValue(ctx, ctxSpanKey, sp)
	return sp, ctx
}

// Fail marks this span as having an error (call before End if needed)
func (s *Span) Fail(err error) {
	if s != nil && err != nil {
		s.failed = err.Error()
	}
}

// End finalizes span; no logging here—buffered for Flush
func (s *Span) End() {
	if s == nil || s.t == nil {
		return
	}
	rec := SpanRecord{
		Name:       s.Name,
		Depth:      s.Depth,
		DurationMS: time.Since(s.start).Milliseconds(),
		Func:       s.Func,
		Caller:     s.Caller,
	}
	if s.failed != "" {
		rec.Error = s.failed
	}

	s.t.mu.Lock()
	if rec.Error != "" {
		s.t.errCnt++
	}
	s.t.timeline = append(s.t.timeline, rec)
	s.t.mu.Unlock()
}

// Flush emits exactly one log for the whole trace.
// If *errp != nil → level=error and include stack []string.
// If handler ok but some spans failed → level=warn.
// Else → level=info.
func Flush(ctx context.Context, errp *error) {
	tr := CurrentTrace(ctx)
	if tr == nil {
		return
	}

	tr.mu.Lock()
	timeline := append([]SpanRecord(nil), tr.timeline...)
	errCnt := tr.errCnt
	tr.mu.Unlock()

	total := time.Since(tr.Start).Milliseconds()
	fields := map[string]any{
		"trace_id":    tr.TraceID,
		"name":        tr.Name,
		"total_ms":    total,
		"span_count":  len(timeline),
		"error_count": errCnt,
		"timeline":    timeline,
	}

	level := "info"
	msg := "[trace] finished"

	if errp != nil && *errp != nil {
		level = "error"
		fields["handler_error"] = (*errp).Error()
		fields["stack"] = buildStackSlice(3, 12) // []string
	} else if errCnt > 0 {
		level = "warn"
	}

	logf(ctx, level, msg, fields)
}

// ---------------- Context helpers ----------------

func CurrentTrace(ctx context.Context) *Trace {
	if v := ctx.Value(ctxTraceKey); v != nil {
		if t, ok := v.(*Trace); ok {
			return t
		}
	}
	return nil
}

func CurrentSpan(ctx context.Context) *Span {
	if v := ctx.Value(ctxSpanKey); v != nil {
		if s, ok := v.(*Span); ok {
			return s
		}
	}
	return nil
}

func TraceID(ctx context.Context) string {
	if v := ctx.Value(ctxTraceIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ---------------- Utils ----------------

func caller(skip int) (fn, file string, line int) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", "unknown", 0
	}
	fn = runtime.FuncForPC(pc).Name()
	return fn, file, line
}
func shortFile(f string) string {
	if i := strings.LastIndex(f, "/"); i >= 0 {
		return f[i+1:]
	}
	return f
}
func fmtPos(file string, line int) string { return fmt.Sprintf("%s:%d", shortFile(file), line) }

func buildStackSlice(skip, max int) []string {
	pcs := make([]uintptr, max+skip)
	n := runtime.Callers(skip, pcs)
	fs := runtime.CallersFrames(pcs[:n])

	out := make([]string, 0, max)
	depth := 0
	for {
		fr, more := fs.Next()
		if strings.Contains(fr.Function, "runtime.") {
			if !more {
				break
			}
			continue
		}
		out = append(out, fmt.Sprintf("%s (%s:%d)", fr.Function, shortFile(fr.File), fr.Line))
		depth++
		if depth >= max || !more {
			break
		}
	}
	return out
}

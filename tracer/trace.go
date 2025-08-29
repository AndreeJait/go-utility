package tracer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

type ctxKey string

const (
	ctxTraceIDKey ctxKey = "tracer.trace_id"
	ctxSpanKey    ctxKey = "tracer.span"
)

type LogFunc func(ctx context.Context, level, msg string, fields map[string]any)

var logf LogFunc = func(ctx context.Context, level, msg string, fields map[string]any) { /* no-op by default */ }

func SetLogger(fn LogFunc) { logf = fn }

func newTraceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	x := make([]byte, 32)
	hex.Encode(x, b[:])
	return string(x)
}
func newSpanID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	x := make([]byte, 16)
	hex.Encode(x, b[:])
	return string(x)
}

func parseTraceParent(s string) (traceID, parentID string, ok bool) {
	s = strings.TrimSpace(s)
	p := strings.Split(s, "-")
	if len(p) != 4 || len(p[1]) != 32 || len(p[2]) != 16 {
		return "", "", false
	}
	return p[1], p[2], true
}
func formatTraceParent(traceID, spanID string) string {
	return fmt.Sprintf("00-%s-%s-01", traceID, spanID)
}

type Span struct {
	TraceID   string
	SpanID    string
	ParentID  string
	Name      string
	Start     time.Time
	Depth     int
	CallerFn  string
	CallerPos string // file:line
	Fields    map[string]any
	ended     int32
}

type StartOption func(*Span)
type Field struct {
	Key string
	Val any
}

func WithField(k string, v any) StartOption { return func(s *Span) { s.Fields[k] = v } }
func WithFields(m map[string]any) StartOption {
	return func(s *Span) {
		for k, v := range m {
			s.Fields[k] = v
		}
	}
}

func StartSpan(ctx context.Context, name string, opts ...StartOption) (context.Context, *Span) {
	traceID := TraceID(ctx)
	if traceID == "" {
		traceID = newTraceID()
		ctx = context.WithValue(ctx, ctxTraceIDKey, traceID)
	}
	parent := Current(ctx)
	parentID := ""
	depth := 0
	if parent != nil {
		parentID = parent.SpanID
		depth = parent.Depth + 1
	}
	fn, file, line := caller(2)
	sp := &Span{
		TraceID:   traceID,
		SpanID:    newSpanID(),
		ParentID:  parentID,
		Name:      name,
		Start:     time.Now(),
		Depth:     depth,
		CallerFn:  fn,
		CallerPos: fmt.Sprintf("%s:%d", shortFile(file), line),
		Fields:    map[string]any{},
	}
	for _, o := range opts {
		o(sp)
	}

	logf(ctx, "debug", "[span] started", map[string]any{
		"trace_id":  sp.TraceID,
		"span_id":   sp.SpanID,
		"parent_id": sp.ParentID,
		"name":      sp.Name,
		"depth":     sp.Depth,
		"func":      sp.CallerFn,
		"caller":    sp.CallerPos,
	})

	ctx = context.WithValue(ctx, ctxSpanKey, sp)
	return ctx, sp
}

func (s *Span) End(err error, extra ...Field) {
	if s == nil || !atomic.CompareAndSwapInt32(&s.ended, 0, 1) {
		return
	}
	dur := time.Since(s.Start)
	fields := map[string]any{
		"trace_id":    s.TraceID,
		"span_id":     s.SpanID,
		"parent_id":   s.ParentID,
		"name":        s.Name,
		"duration_ms": dur.Milliseconds(),
		"depth":       s.Depth,
		"func":        s.CallerFn,
		"caller":      s.CallerPos,
	}
	for k, v := range s.Fields {
		fields[k] = v
	}
	for _, f := range extra {
		fields[f.Key] = f.Val
	}

	level := "info"
	msg := "[span] finished"
	if err != nil {
		level = "error"
		fields["error"] = err.Error()
		msg = "[span] finished_with_error"
	}
	logf(context.WithValue(context.Background(), ctxTraceIDKey, s.TraceID), level, msg, fields)
}

func Event(ctx context.Context, msg string, kv map[string]any) {
	if kv == nil {
		kv = map[string]any{}
	}
	if sp := Current(ctx); sp != nil {
		kv["trace_id"], kv["span_id"], kv["depth"] = sp.TraceID, sp.SpanID, sp.Depth
	}
	logf(ctx, "debug", msg, kv)
}

func Current(ctx context.Context) *Span {
	if v := ctx.Value(ctxSpanKey); v != nil {
		if sp, ok := v.(*Span); ok {
			return sp
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

func Inject(ctx context.Context, h http.Header) {
	tid := TraceID(ctx)
	if tid == "" {
		tid = newTraceID()
	}
	h.Set("traceparent", formatTraceParent(tid, newSpanID()))
	h.Set("X-Request-ID", tid)
}

func Extract(h http.Header) (traceID, parentSpanID string, ok bool) {
	if tp := h.Get("traceparent"); tp != "" {
		if tid, pid, ok := parseTraceParent(tp); ok {
			return tid, pid, true
		}
	}
	if rid := h.Get("X-Request-ID"); rid != "" {
		return rid, "", true
	}
	return "", "", false
}

func caller(skip int) (fn, file string, line int) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", "unknown", 0
	}
	fn = runtime.FuncForPC(pc).Name()
	return fn, file, line
}
func shortFile(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// ===== helpers you already had =====

func GetFuncName(fn interface{}) string {
	if fn == nil {
		return ""
	}
	val := reflect.ValueOf(fn)
	if val.Kind() != reflect.Func {
		return ""
	}
	pc := val.Pointer()
	f := runtime.FuncForPC(pc)
	if f == nil {
		return ""
	}
	return f.Name()
}

func GetShortFuncName(fn interface{}) string {
	full := GetFuncName(fn)
	if full == "" {
		return ""
	}
	parts := strings.Split(full, ".")
	return parts[len(parts)-1]
}

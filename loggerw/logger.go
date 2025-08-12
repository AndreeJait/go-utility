package loggerw

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

type (
	Logger interface {
		// Context-aware logging (no error param)
		Info(ctx context.Context, args ...interface{})
		//go:formatprintf 2
		Infof(ctx context.Context, format string, args ...interface{})
		Debug(ctx context.Context, args ...interface{})
		//go:formatprintf 2
		Debugf(ctx context.Context, format string, args ...interface{})
		Warning(ctx context.Context, args ...interface{})
		//go:formatprintf 2
		Warningf(ctx context.Context, format string, args ...interface{})
		Fatal(ctx context.Context, args ...interface{})
		//go:formatprintf 2
		Fatalf(ctx context.Context, format string, args ...interface{})
		Print(ctx context.Context, args ...interface{})
		//go:formatprintf 2
		Printf(ctx context.Context, format string, args ...interface{})
		Println(ctx context.Context, args ...interface{})

		// Error variants REQUIRE an error param
		Error(ctx context.Context, err error, args ...interface{})
		//go:formatprintf 3
		Errorf(ctx context.Context, err error, format string, args ...interface{})

		// With extra structured fields (keeps ctx-bound request_id)
		With(ctx context.Context, fields Fields) Logger

		Instance() interface{}
	}

	Level     string
	Formatter string
	Fields    map[string]any

	Option struct {
		Level                       Level
		LogFilePath                 string
		Formatter                   Formatter
		MaxSize, MaxBackups, MaxAge int
		Compress                    bool

		// Extras
		TimestampFormat string // default: time.RFC3339Nano
		ReportCaller    bool   // include file:line & func
		IncludeStack    bool   // attach 'stack' field (trimmed)
		StackMaxDepth   int    // default: 6
		ConsoleAlso     bool   // also write to stdout if file is enabled
	}

	logger struct {
		instance *logrus.Logger
		opt      *Option
		base     *logrus.Entry
	}
)

const (
	Info  Level = "INFO"
	Debug Level = "DEBUG"
	Error Level = "ERROR"

	JSONFormatter Formatter = "JSON"
	TextFormatter Formatter = "TEXT"
)

// ===== Request ID in context =====

type contextKey int

var RequestIDHeader = "X-Request-ID"

const (
	requestIDKey contextKey = iota
)

func GetRequestID(ctx context.Context) string {
	if reqID, ok := ctx.Value(requestIDKey).(string); ok {
		return reqID
	}
	return ""
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func ensureRequestID(ctx context.Context) (context.Context, string) {
	id := GetRequestID(ctx)
	if id == "" {
		id = uuid.New().String()
		ctx = WithRequestID(ctx, id)
	}
	return ctx, id
}

// ===== Constructor =====

func New(option *Option) (Logger, error) {
	if option == nil {
		option = &Option{}
	}
	if option.TimestampFormat == "" {
		option.TimestampFormat = time.RFC3339Nano
	}
	if option.StackMaxDepth <= 0 {
		option.StackMaxDepth = 6
	}

	l := logrus.New()

	// Level
	switch option.Level {
	case Debug:
		l.SetLevel(logrus.DebugLevel)
	case Error:
		l.SetLevel(logrus.ErrorLevel)
	default:
		l.SetLevel(logrus.InfoLevel)
	}

	// Formatter
	var fmtr logrus.Formatter
	switch option.Formatter {
	case JSONFormatter:
		j := &logrus.JSONFormatter{
			TimestampFormat: option.TimestampFormat,
			PrettyPrint:     false,
		}
		fmtr = j
	default:
		t := &logrus.TextFormatter{
			TimestampFormat: option.TimestampFormat,
			FullTimestamp:   true,
			DisableQuote:    true,
		}
		fmtr = t
	}

	// Caller
	l.SetReportCaller(option.ReportCaller)
	// Prettyfier when caller is enabled
	fmtr = wrapCallerPrettyfier(fmtr)
	l.SetFormatter(fmtr)

	// Outputs
	var writers []io.Writer
	if option.LogFilePath != "" {
		writers = append(writers, &lumberjack.Logger{
			Filename:   option.LogFilePath,
			MaxSize:    option.MaxSize,
			MaxAge:     option.MaxAge,
			MaxBackups: option.MaxBackups,
			LocalTime:  true,
			Compress:   option.Compress,
		})
	}
	if option.ConsoleAlso || option.LogFilePath == "" {
		writers = append(writers, os.Stdout)
	}
	l.SetOutput(io.MultiWriter(writers...))

	return &logger{
		instance: l,
		opt:      option,
		base:     logrus.NewEntry(l),
	}, nil
}

func wrapCallerPrettyfier(f logrus.Formatter) logrus.Formatter {
	switch ff := f.(type) {
	case *logrus.TextFormatter:
		ff.CallerPrettyfier = func(fr *runtime.Frame) (string, string) {
			return shortFunc(fr.Function), fmt.Sprintf("%s:%d", shortFile(fr.File), fr.Line)
		}
		return ff
	case *logrus.JSONFormatter:
		ff.CallerPrettyfier = func(fr *runtime.Frame) (string, string) {
			return shortFunc(fr.Function), fmt.Sprintf("%s:%d", shortFile(fr.File), fr.Line)
		}
		return ff
	default:
		return f
	}
}

func shortFile(f string) string {
	if i := strings.LastIndex(f, "/"); i >= 0 {
		return f[i+1:]
	}
	return f
}
func shortFunc(fn string) string {
	if i := strings.LastIndex(fn, "/"); i >= 0 {
		fn = fn[i+1:]
	}
	return fn
}

func (l *logger) entry(ctx context.Context, extra Fields) *logrus.Entry {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, rid := ensureRequestID(ctx)

	e := l.base.WithField("request_id", rid)

	if l.opt.IncludeStack {
		e = e.WithField("stack", buildStack(4, l.opt.StackMaxDepth))
	}
	for k, v := range extra {
		e = e.WithField(k, v)
	}
	return e
}

func buildStack(skip, max int) string {
	pcs := make([]uintptr, max+skip)
	n := runtime.Callers(skip, pcs)
	frames := runtime.CallersFrames(pcs[:n])

	var b strings.Builder
	depth := 0
	for {
		fr, more := frames.Next()
		if strings.Contains(fr.Function, "runtime.") ||
			strings.Contains(fr.Function, "github.com/sirupsen/logrus") {
			if !more {
				break
			}
			continue
		}
		fmt.Fprintf(&b, "%s (%s:%d)\n", fr.Function, shortFile(fr.File), fr.Line)
		depth++
		if depth >= max || !more {
			break
		}
	}
	return strings.TrimSpace(b.String())
}

// ===== Interface impl (context-aware) =====

func (l *logger) Info(ctx context.Context, args ...interface{}) { l.entry(ctx, nil).Info(args...) }

//go:formatprintf 2
func (l *logger) Infof(ctx context.Context, f string, a ...interface{}) {
	l.entry(ctx, nil).Infof(f, a...)
}

func (l *logger) Debug(ctx context.Context, args ...interface{}) { l.entry(ctx, nil).Debug(args...) }

//go:formatprintf 2
func (l *logger) Debugf(ctx context.Context, f string, a ...interface{}) {
	l.entry(ctx, nil).Debugf(f, a...)
}

func (l *logger) Warning(ctx context.Context, args ...interface{}) { l.entry(ctx, nil).Warn(args...) }

//go:formatprintf 2
func (l *logger) Warningf(ctx context.Context, f string, a ...interface{}) {
	l.entry(ctx, nil).Warnf(f, a...)
}

func (l *logger) Fatal(ctx context.Context, args ...interface{}) { l.entry(ctx, nil).Fatal(args...) }

//go:formatprintf 2
func (l *logger) Fatalf(ctx context.Context, f string, a ...interface{}) {
	l.entry(ctx, nil).Fatalf(f, a...)
}

func (l *logger) Print(ctx context.Context, args ...interface{}) { l.entry(ctx, nil).Print(args...) }

//go:formatprintf 2
func (l *logger) Printf(ctx context.Context, f string, a ...interface{}) {
	l.entry(ctx, nil).Printf(f, a...)
}
func (l *logger) Println(ctx context.Context, args ...interface{}) {
	l.entry(ctx, nil).Println(args...)
}

// Error variants REQUIRE error parameter
func (l *logger) Error(ctx context.Context, err error, args ...interface{}) {
	l.entry(ctx, Fields{"error": err.Error()}).Error(args...)
}

//go:formatprintf 3
func (l *logger) Errorf(ctx context.Context, err error, f string, a ...interface{}) {
	l.entry(ctx, Fields{"error": err.Error()}).Errorf(f, a...)
}

func (l *logger) With(ctx context.Context, fields Fields) Logger {
	return &logger{
		instance: l.instance,
		opt:      l.opt,
		base:     l.entry(ctx, fields),
	}
}

func (l *logger) Instance() interface{} { return l.instance }

// ===== Convenience =====

func DefaultLog() (Logger, error) {
	return New(&Option{
		Level:           Info,
		Formatter:       TextFormatter,
		TimestampFormat: time.RFC3339Nano,
		ReportCaller:    true,
		IncludeStack:    false,
		StackMaxDepth:   6,
		ConsoleAlso:     true,
	})
}

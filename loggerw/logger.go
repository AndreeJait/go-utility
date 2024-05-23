package loggerw

import (
	"context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"net/http"

	"github.com/google/uuid"
)

type (
	Logger interface {
		Info(...interface{})
		Infof(string, ...interface{})
		Debug(...interface{})
		Debugf(string, ...interface{})
		Error(...interface{})
		Errorf(string, ...interface{})
		Warning(...interface{})
		Warningf(string, ...interface{})
		Fatal(...interface{})
		Fatalf(string, ...interface{})
		Print(...interface{})
		Printf(string, ...interface{})
		Println(...interface{})
		Instance() interface{}
	}

	Level     string
	Formatter string

	Option struct {
		Level                       Level
		LogFilePath                 string
		Formatter                   Formatter
		MaxSize, MaxBackups, MaxAge int
		Compress                    bool
	}

	logger struct {
		instance *logrus.Logger
	}
)

const (
	Info  Level = "INFO"
	Debug Level = "DEBUG"
	Error Level = "ERROR"

	JSONFormatter Formatter = "JSON"
	TextFormatter Formatter = "TEXT"
)

func (l *logger) Info(args ...interface{}) {
	l.instance.Info(args...)
}

func (l *logger) Infof(format string, args ...interface{}) {
	l.instance.Infof(format, args...)
}

func (l *logger) Debug(args ...interface{}) {
	l.instance.Debug(args...)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	l.instance.Debugf(format, args...)
}

func (l *logger) Error(args ...interface{}) {
	l.instance.Error(args...)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	l.instance.Errorf(format, args...)
}

func (l *logger) Warning(args ...interface{}) {
	l.instance.Warning(args...)
}

func (l *logger) Warningf(format string, args ...interface{}) {
	l.instance.Warningf(format, args...)
}

func (l *logger) Fatal(args ...interface{}) {
	l.instance.Fatal(args...)
}

func (l *logger) Fatalf(format string, args ...interface{}) {
	l.instance.Fatalf(format, args...)
}

func (l *logger) Print(args ...interface{}) {
	l.instance.Print(args...)
}

func (l *logger) Println(args ...interface{}) {
	l.instance.Println(args...)
}

func (l *logger) Printf(format string, args ...interface{}) {
	l.instance.Printf(format, args...)
}

func (l *logger) Instance() interface{} {
	return l.instance
}
func New(option *Option) (Logger, error) {
	instance := logrus.New()

	if option.Level == Info {
		instance.Level = logrus.InfoLevel
	}

	if option.Level == Debug {
		instance.Level = logrus.DebugLevel
	}

	if option.Level == Error {
		instance.Level = logrus.ErrorLevel
	}

	var formatter logrus.Formatter

	if option.Formatter == JSONFormatter {
		formatter = &logrus.JSONFormatter{}
	} else {
		formatter = &logrus.TextFormatter{}
	}

	instance.Formatter = formatter

	// - check if log file path does exists
	if option.LogFilePath != "" {
		lbj := &lumberjack.Logger{
			Filename:   option.LogFilePath,
			MaxSize:    option.MaxSize,
			MaxAge:     option.MaxAge,
			MaxBackups: option.MaxBackups,
			LocalTime:  true,
			Compress:   option.Compress,
		}

		instance.Hooks.Add(&lumberjackHook{
			lbj:    lbj,
			logrus: instance,
		})
	}

	return &logger{instance}, nil
}

type (
	lumberjackHook struct {
		lbj    *lumberjack.Logger
		logrus *logrus.Logger
	}
)

func (l *lumberjackHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.InfoLevel, logrus.DebugLevel, logrus.ErrorLevel}
}

func (l *lumberjackHook) Fire(entry *logrus.Entry) error {
	b, err := l.logrus.Formatter.Format(entry)

	if err != nil {
		return errors.WithStack(err)
	}

	if _, err := l.lbj.Write(b); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func DefaultLog() (Logger, error) {
	return New(&Option{
		Level:     Info,
		Formatter: TextFormatter,
	})
}

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

// getRequestID extracts the correlation ID from the HTTP request
func getRequestID(req *http.Request) string {
	return req.Header.Get(RequestIDHeader)
}

// WithRequest returns a context which knows the request ID and correlation ID in the given request.
func WithRequest(ctx context.Context, req *http.Request) (context.Context, string) {
	id := getRequestID(req)
	if id == "" {
		id = uuid.New().String()
		req.Header.Set(RequestIDHeader, id)
	}
	ctx = context.WithValue(ctx, requestIDKey, id)
	return ctx, id
}

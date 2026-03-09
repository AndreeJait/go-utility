package muxw

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/AndreeJait/go-utility/v2/responsew"
	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

// Config holds the configuration options for initializing the Gorilla Mux router.
type Config struct {
	DebugMode     bool // Ditambahkan agar seragam dengan Echo dan Gin
	EnableSwagger bool
	// ErrorHandler allows overriding the default JSON error response mechanism.
	ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)
}

// globalErrorHandler holds a reference to the custom handler for use within ApiWrap.
var globalErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

// New initializes a fresh Gorilla Mux router equipped with context-aware logging
// and optional Swagger UI integration.
func New(cfg *Config) *mux.Router {
	r := mux.NewRouter()
	globalErrorHandler = cfg.ErrorHandler

	// Apply global logger middleware
	r.Use(loggerMiddleware)

	// Mount Swagger UI if enabled
	if cfg.EnableSwagger {
		r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)
	}

	return r
}

// loggerMiddleware injects a unique trace ID into the request context
// and logs the execution latency.
func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := logw.InjectLogID(r.Context())
		r = r.WithContext(ctx)

		start := time.Now()

		// Execute the actual handler logic
		next.ServeHTTP(w, r)

		logw.CtxInfof(ctx, "[MUX] %s %s | Latency: %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// API defines the strict contract for a Gorilla Mux handler struct.
type API interface {
	Handle(r *http.Request) (any, error)
}

// Bind converts a standard Handle signature into an http.HandlerFunc.
func Bind(handle func(r *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ApiWrap(w, r, responsew.ExecutorFunc(func() (any, error) {
			return handle(r)
		}))
	}
}

// ApiWrap executes a HandlerExecutor interface. It automatically evaluates the returned data
// and writes the appropriate HTTP response (JSON, File Stream, or Error).
func ApiWrap(w http.ResponseWriter, r *http.Request, exec responsew.HandlerExecutor) {
	data, err := exec.Handle()

	if err != nil {
		if globalErrorHandler != nil {
			globalErrorHandler(w, r, err)
			return
		}

		httpCode, payload := responsew.Error(err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpCode)
		json.NewEncoder(w).Encode(payload)
		return
	}

	switch v := data.(type) {
	case responsew.BaseResponse:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(v)

	case responsew.FileResponse:
		w.Header().Set("Content-Disposition", "attachment; filename="+v.Filename)
		w.Header().Set("Content-Type", v.ContentType)
		_, _ = io.Copy(w, v.Reader)

	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responsew.Success(v, "Success"))
	}
}

// ParsePagination extracts 'page' and 'per_page' parameters from the request URL queries.
func ParsePagination(r *http.Request) responsew.Pagination {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100
	}

	return responsew.Pagination{Page: page, PerPage: perPage}
}

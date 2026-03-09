package elasticw

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/elastic/go-elasticsearch/v8"
)

type contextKey string

const debugKey contextKey = "elasticw-debug"

// Config holds the necessary configuration to establish an Elasticsearch connection.
type Config struct {
	Addresses []string // e.g., []string{"http://localhost:9200"}
	Username  string
	Password  string
	APIKey    string
	CloudID   string
	DebugMode bool // If true, logs all HTTP requests and responses globally
}

// debugTransport intercepts HTTP requests to Elasticsearch for logging purposes.
// It implements the http.RoundTripper interface.
type debugTransport struct {
	baseTransport http.RoundTripper
	globalDebug   bool
}

// RoundTrip executes a single HTTP transaction and logs the request/response if debugging is enabled.
func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	isDebug, _ := ctx.Value(debugKey).(bool)

	// If neither global nor local debug is enabled, proceed normally
	if !t.globalDebug && !isDebug {
		return t.baseTransport.RoundTrip(req)
	}

	// --- LOGGING REQUEST ---
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		// Restore the request body since reading it drains the buffer
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
	}

	start := time.Now()
	res, err := t.baseTransport.RoundTrip(req)
	duration := time.Since(start)

	// If network error occurred
	if err != nil {
		logw.CtxErrorf(ctx, "[ELASTIC FAILED] %s %s | Duration: %v | err: %v", req.Method, req.URL.Path, duration, err)
		return res, err
	}

	// --- LOGGING RESPONSE ---
	var resBody []byte
	if res.Body != nil {
		resBody, _ = io.ReadAll(res.Body)
		// Restore the response body for the elasticsearch client to process
		res.Body = io.NopCloser(bytes.NewBuffer(resBody))
	}

	// Format the log
	logMsg := fmt.Sprintf("[ELASTIC DEBUG] %s %s | Status: %d | Duration: %v", req.Method, req.URL.Path, res.StatusCode, duration)

	if len(reqBody) > 0 {
		logMsg += fmt.Sprintf(" | Req: %s", string(reqBody))
	}
	// Warning: Response body can be huge. We truncate it if it's too long to prevent log flooding.
	if len(resBody) > 0 {
		if len(resBody) > 1000 {
			logMsg += fmt.Sprintf(" | Res: %s...", string(resBody[:1000]))
		} else {
			logMsg += fmt.Sprintf(" | Res: %s", string(resBody))
		}
	}

	// Use Warning level for 4xx and 5xx status codes
	if res.StatusCode >= 400 {
		logw.CtxWarning(ctx, logMsg)
	} else {
		logw.CtxInfo(ctx, logMsg)
	}

	return res, nil
}

// Connect establishes a connection to the Elasticsearch cluster.
// It uses the TypedClient which provides a strongly-typed API for queries.
func Connect(ctx context.Context, cfg *Config) (*elasticsearch.TypedClient, error) {
	if len(cfg.Addresses) == 0 && cfg.CloudID == "" {
		return nil, fmt.Errorf("elasticw: addresses or CloudID is required")
	}

	// Setup custom transport with connection pooling and our debug interceptor
	baseTransport := &http.Transport{
		MaxIdleConnsPerHost:   10,
		ResponseHeaderTimeout: time.Second * 30,
		DialContext:           (&net.Dialer{Timeout: time.Second * 30}).DialContext,
	}

	customTransport := &debugTransport{
		baseTransport: baseTransport,
		globalDebug:   cfg.DebugMode,
	}

	esConfig := elasticsearch.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
		APIKey:    cfg.APIKey,
		CloudID:   cfg.CloudID,
		Transport: customTransport, // Inject our interceptor
	}

	client, err := elasticsearch.NewTypedClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("elasticw: failed to create client: %w", err)
	}

	// Verify connection by Ping
	ok, err := client.Ping().IsSuccess(ctx)
	if err != nil || !ok {
		return nil, fmt.Errorf("elasticw: failed to ping elasticsearch cluster: %w", err)
	}

	logw.CtxInfof(ctx, "Successfully connected to Elasticsearch")
	return client, nil
}

// Disconnect safely handles the cleanup for the Elasticsearch client.
// Since the client relies on the standard HTTP protocol, idle connections
// are managed automatically by the HTTP transport's idle timeout settings.
// It returns a function that implements gracefulw.CleanupFunc.
func Disconnect(client *elasticsearch.TypedClient) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if client != nil {
			// Kita hanya perlu mencetak log, karena koneksi HTTP Elasticsearch
			// akan otomatis diurus oleh garbage collector dan IdleTimeout bawaan Golang.
			logw.CtxInfo(ctx, "Detaching Elasticsearch client...")
		}
		return nil
	}
}

// DebugContext injects a debug flag into the context.
// When this context is passed to any Elasticsearch API call, the HTTP request
// and response will be logged, regardless of the global DebugMode setting.
//
// Usage:
//
//	res, err := client.Search().Index("users").Do(elasticw.DebugContext(ctx))
func DebugContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, debugKey, true)
}

package echow

import (
	"io"
	"net/http"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/AndreeJait/go-utility/v2/responsew"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

// Config holds the configuration options for initializing the Echo HTTP server.
type Config struct {
	DebugMode     bool
	EnableSwagger bool
	ErrorHandler  echo.HTTPErrorHandler
}

// New initializes a new Echo v5 instance equipped with panic recovery,
// context-aware logging, and a centralized standard error handler.
func New(cfg *Config) *echo.Echo {
	e := echo.New()

	if cfg.ErrorHandler != nil {
		e.HTTPErrorHandler = cfg.ErrorHandler
	} else {
		e.HTTPErrorHandler = defaultErrorHandler
	}

	e.Use(middleware.Recover())
	e.Use(loggerMiddleware())

	if cfg.EnableSwagger {
		e.GET("/swagger/*", func(c *echo.Context) error {
			httpSwagger.WrapHandler.ServeHTTP(c.Response(), c.Request())
			return nil
		})
	}

	return e
}

// defaultErrorHandler intercepts errors bubbled up from handlers and formats
// them into the standard JSON response using the responsew utility.
func defaultErrorHandler(c *echo.Context, err error) {
	httpCode, payload := responsew.Error(err)
	_ = c.JSON(httpCode, payload)
}

// loggerMiddleware injects a unique trace ID into the request context,
// logs the execution latency, and records the HTTP status code.
func loggerMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := logw.InjectLogID(req.Context())

			// Inject the enriched context back into the Echo request
			c.SetRequest(req.WithContext(ctx))

			start := time.Now()

			// Execute the actual handler logic
			err := next(c)

			// 🌟 FIX: We assume HTTP 200 OK unless an error is returned.
			// This elegantly bypasses the lack of .Status in Echo v5's http.ResponseWriter.
			status := http.StatusOK

			if err != nil {
				// Infer the status code from the error
				status, _ = responsew.Error(err)
				// Manually trigger the error handler
				c.Echo().HTTPErrorHandler(c, err)
			}

			logw.CtxInfof(ctx, "[ECHO] %s %s | Status: %d | Latency: %v", req.Method, req.URL.Path, status, time.Since(start))

			// Return nil since errors are fully handled above
			return nil
		}
	}
}

// API defines the strict contract for an Echo handler struct.
type API interface {
	Handle(c *echo.Context) (any, error)
}

// Bind converts a standard Handle signature into an echo.HandlerFunc.
func Bind(handle func(c *echo.Context) (any, error)) echo.HandlerFunc {
	return func(c *echo.Context) error {
		return ApiWrap(c, responsew.ExecutorFunc(func() (any, error) {
			return handle(c)
		}))
	}
}

// ApiWrap executes a HandlerExecutor interface and writes the HTTP response.
func ApiWrap(c *echo.Context, exec responsew.HandlerExecutor) error {
	data, err := exec.Handle()
	if err != nil {
		return err // Let the loggerMiddleware catch and process this error
	}

	switch v := data.(type) {
	case responsew.BaseResponse:
		return c.JSON(http.StatusOK, v)

	case responsew.FileResponse:
		c.Response().Header().Set("Content-Disposition", "attachment; filename="+v.Filename)
		c.Response().Header().Set("Content-Type", v.ContentType)
		c.Response().WriteHeader(http.StatusOK)
		_, streamErr := io.Copy(c.Response(), v.Reader)
		return streamErr

	default:
		return c.JSON(http.StatusOK, responsew.Success(v, "Success"))
	}
}

// ParsePagination extracts 'page' and 'per_page' parameters.
func ParsePagination(c *echo.Context) responsew.Pagination {
	req := struct {
		Page    int `query:"page" form:"page" json:"page"`
		PerPage int `query:"per_page" form:"per_page" json:"per_page"`
	}{}
	_ = c.Bind(&req)

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PerPage <= 0 {
		req.PerPage = 10
	}
	if req.PerPage > 100 {
		req.PerPage = 100
	}

	return responsew.Pagination{Page: req.Page, PerPage: req.PerPage}
}

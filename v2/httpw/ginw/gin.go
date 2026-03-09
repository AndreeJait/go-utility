package ginw

import (
	"net/http"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/AndreeJait/go-utility/v2/responsew"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Config holds the configuration options for initializing the Gin HTTP engine.
type Config struct {
	DebugMode     bool
	EnableSwagger bool
	// ErrorHandler allows overriding default error handling.
	// It should return true if the error was fully handled to prevent default processing.
	ErrorHandler func(c *gin.Context, err error) bool
}

// New initializes a fresh Gin engine equipped with panic recovery,
// context-aware logging, and a centralized standard error catcher.
func New(cfg *Config) *gin.Engine {
	if !cfg.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Global Logger and Error Catcher Middleware
	r.Use(func(c *gin.Context) {
		req := c.Request
		ctx := logw.InjectLogID(req.Context())
		c.Request = req.WithContext(ctx)

		start := time.Now()

		// Process the request chain
		c.Next()

		// Catch any errors appended via c.Error() during execution
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			handled := false
			if cfg.ErrorHandler != nil {
				handled = cfg.ErrorHandler(c, err)
			}

			// If not handled by a custom handler, format using the standard response builder
			if !handled {
				httpCode, payload := responsew.Error(err)
				c.JSON(httpCode, payload)
			}
		}

		logw.CtxInfof(ctx, "[GIN] %s %s | Status: %d | Latency: %v", req.Method, req.URL.Path, c.Writer.Status(), time.Since(start))
	})

	// Mount Swagger UI if enabled
	if cfg.EnableSwagger {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	return r
}

// API defines the strict contract for a Gin handler struct.
// It enforces the separation of routing registration and business logic execution.
type API interface {
	Handle(c *gin.Context) (any, error)
}

// Bind is an elegant higher-order function that eliminates the need for the Route() boilerplate.
// It directly converts a standard Handle signature into a gin.HandlerFunc.
func Bind(handle func(c *gin.Context) (any, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		ApiWrap(c, responsew.ExecutorFunc(func() (any, error) {
			return handle(c)
		}))
	}
}

// ApiWrap executes a HandlerExecutor interface. It automatically evaluates the returned data
// and writes the appropriate HTTP response (JSON, File Stream, or Error).
func ApiWrap(c *gin.Context, exec responsew.HandlerExecutor) {
	data, err := exec.Handle()
	if err != nil {
		// Forward error to the Gin middleware for centralized handling
		c.Error(err)
		c.Abort()
		return
	}

	switch v := data.(type) {
	case responsew.BaseResponse:
		c.JSON(http.StatusOK, v)

	case responsew.FileResponse:
		c.Header("Content-Disposition", "attachment; filename="+v.Filename)
		c.DataFromReader(http.StatusOK, -1, v.ContentType, v.Reader, map[string]string{})

	default:
		c.JSON(http.StatusOK, responsew.Success(v, "Success"))
	}
}

// ParsePagination extracts 'page' and 'per_page' parameters from the request URL queries or JSON body.
// It enforces safe defaults and maximum limits to prevent resource exhaustion.
func ParsePagination(c *gin.Context) responsew.Pagination {
	req := struct {
		Page    int `form:"page" json:"page"`
		PerPage int `form:"per_page" json:"per_page"`
	}{}
	_ = c.ShouldBind(&req)

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

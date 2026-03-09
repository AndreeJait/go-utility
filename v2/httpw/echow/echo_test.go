package echow

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AndreeJait/go-utility/v2/responsew"
	"github.com/AndreeJait/go-utility/v2/statusw"
	"github.com/labstack/echo/v5"
)

// setupEcho initializes a fresh Echo instance for testing purposes.
func setupEcho() *echo.Echo {
	return New(&Config{DebugMode: true})
}

// TestEcho_ParsePagination verifies that the pagination extractor correctly
// parses valid inputs, applies defaults, and enforces maximum limits.
func TestEcho_ParsePagination(t *testing.T) {
	e := setupEcho()
	e.GET("/paginate", func(c *echo.Context) error {
		pg := ParsePagination(c)
		return c.JSON(http.StatusOK, pg)
	})

	// Test with explicit values exceeding the maximum limit
	req := httptest.NewRequest(http.MethodGet, "/paginate?page=3&per_page=500", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var pg responsew.Pagination
	if err := json.Unmarshal(rec.Body.Bytes(), &pg); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if pg.Page != 3 {
		t.Errorf("Expected Page 3, got %d", pg.Page)
	}
	// The limit should be capped at 100
	if pg.PerPage != 100 {
		t.Errorf("Expected PerPage to be capped at 100, got %d", pg.PerPage)
	}
}

// TestEcho_BindAndApiWrap validates that the Smart Binder correctly executes the handler,
// processes successful generic data, and handles custom statusw errors.
func TestEcho_BindAndApiWrap(t *testing.T) {
	e := setupEcho()

	// 1. Success Endpoint
	e.GET("/success", Bind(func(c *echo.Context) (any, error) {
		return map[string]string{"status": "ok"}, nil
	}))

	// 2. Error Endpoint
	e.GET("/error", Bind(func(c *echo.Context) (any, error) {
		return nil, statusw.NotFound.WithCustomMessage("Resource not found")
	}))

	// Execute Success Test
	reqSuccess := httptest.NewRequest(http.MethodGet, "/success", nil)
	recSuccess := httptest.NewRecorder()
	e.ServeHTTP(recSuccess, reqSuccess)

	if recSuccess.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", recSuccess.Code)
	}

	// Execute Error Test
	reqErr := httptest.NewRequest(http.MethodGet, "/error", nil)
	recErr := httptest.NewRecorder()
	e.ServeHTTP(recErr, reqErr)

	if recErr.Code != http.StatusNotFound {
		t.Errorf("Expected HTTP 404, got %d", recErr.Code)
	}
}

// TestEcho_FileStream verifies that returning a FileResponse struct
// correctly triggers a file download stream instead of a JSON response.
func TestEcho_FileStream(t *testing.T) {
	e := setupEcho()
	expectedContent := "csv,data,here"

	e.GET("/download", Bind(func(c *echo.Context) (any, error) {
		return responsew.FileResponse{
			ContentType: "text/csv",
			Filename:    "test.csv",
			Reader:      bytes.NewBufferString(expectedContent),
		}, nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/download", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}

	// Validate Headers
	contentDisp := rec.Header().Get("Content-Disposition")
	if contentDisp != "attachment; filename=test.csv" {
		t.Errorf("Expected Content-Disposition header, got %s", contentDisp)
	}

	// Validate Body stream
	bodyBytes, _ := io.ReadAll(rec.Body)
	if string(bodyBytes) != expectedContent {
		t.Errorf("Expected file content '%s', got '%s'", expectedContent, string(bodyBytes))
	}
}

type structHandler struct{}

func (s structHandler) Handle(c *echo.Context) (any, error) {
	return map[string]string{"status": "ok"}, nil
}

func NewStructHandler() API {
	return &structHandler{}
}

func TestEcho_withStructHandler(t *testing.T) {
	e := setupEcho()

	// 1. Success Endpoint
	e.GET("/success", Bind(NewStructHandler().Handle))

	// Execute Success Test
	reqSuccess := httptest.NewRequest(http.MethodGet, "/success", nil)
	recSuccess := httptest.NewRecorder()
	e.ServeHTTP(recSuccess, reqSuccess)

	if recSuccess.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", recSuccess.Code)
	}
}

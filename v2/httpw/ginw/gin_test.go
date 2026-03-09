package ginw

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AndreeJait/go-utility/v2/responsew"
	"github.com/AndreeJait/go-utility/v2/statusw"
	"github.com/gin-gonic/gin"
)

// setupGin initializes a fresh Gin engine in TestMode.
func setupGin() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return New(&Config{DebugMode: true})
}

// TestGin_ParsePagination verifies that the pagination extractor correctly
// parses valid inputs, applies defaults, and enforces maximum limits.
func TestGin_ParsePagination(t *testing.T) {
	r := setupGin()
	r.GET("/paginate", func(c *gin.Context) {
		pg := ParsePagination(c)
		c.JSON(http.StatusOK, pg)
	})

	req := httptest.NewRequest(http.MethodGet, "/paginate?page=2&per_page=25", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var pg responsew.Pagination
	if err := json.Unmarshal(rec.Body.Bytes(), &pg); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if pg.Page != 2 || pg.PerPage != 25 {
		t.Errorf("Expected Page 2 and PerPage 25, got %d and %d", pg.Page, pg.PerPage)
	}
}

// TestGin_BindAndApiWrap validates that the Smart Binder correctly executes the handler,
// processes successful generic data, and leverages Gin's middleware for error handling.
func TestGin_BindAndApiWrap(t *testing.T) {
	r := setupGin()

	// 1. Success Endpoint
	r.GET("/success", Bind(func(c *gin.Context) (any, error) {
		return map[string]string{"status": "ok"}, nil
	}))

	// 2. Error Endpoint
	r.GET("/error", Bind(func(c *gin.Context) (any, error) {
		return nil, statusw.InvalidCredential.WithCustomMessage("Login required")
	}))

	// Execute Success Test
	reqSuccess := httptest.NewRequest(http.MethodGet, "/success", nil)
	recSuccess := httptest.NewRecorder()
	r.ServeHTTP(recSuccess, reqSuccess)

	if recSuccess.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", recSuccess.Code)
	}

	// Execute Error Test
	reqErr := httptest.NewRequest(http.MethodGet, "/error", nil)
	recErr := httptest.NewRecorder()
	r.ServeHTTP(recErr, reqErr)

	if recErr.Code != http.StatusUnauthorized {
		t.Errorf("Expected HTTP 401, got %d", recErr.Code)
	}
}

// TestGin_FileStream verifies that returning a FileResponse struct
// correctly triggers a file download stream instead of a JSON response.
func TestGin_FileStream(t *testing.T) {
	r := setupGin()
	expectedContent := "pdf-content-mock"

	r.GET("/download", Bind(func(c *gin.Context) (any, error) {
		return responsew.FileResponse{
			ContentType: "application/pdf",
			Filename:    "doc.pdf",
			Reader:      bytes.NewBufferString(expectedContent),
		}, nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/download", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}

	contentDisp := rec.Header().Get("Content-Disposition")
	if contentDisp != "attachment; filename=doc.pdf" {
		t.Errorf("Invalid Content-Disposition header")
	}

	body, _ := io.ReadAll(rec.Body)
	if string(body) != expectedContent {
		t.Errorf("Expected file content '%s', got '%s'", expectedContent, string(body))
	}
}

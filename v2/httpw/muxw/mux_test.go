package muxw

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AndreeJait/go-utility/v2/responsew"
	"github.com/AndreeJait/go-utility/v2/statusw"
	"github.com/gorilla/mux"
)

// setupMux initializes a fresh Gorilla Mux router for testing.
func setupMux() *mux.Router {
	return New(&Config{DebugMode: true})
}

// TestMux_ParsePagination verifies that the pagination extractor correctly
// parses valid URL query inputs, applies defaults, and enforces maximum limits.
func TestMux_ParsePagination(t *testing.T) {
	r := setupMux()
	r.HandleFunc("/paginate", func(w http.ResponseWriter, req *http.Request) {
		pg := ParsePagination(req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pg)
	}).Methods(http.MethodGet)

	req := httptest.NewRequest(http.MethodGet, "/paginate?page=4&per_page=12", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var pg responsew.Pagination
	if err := json.Unmarshal(rec.Body.Bytes(), &pg); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if pg.Page != 4 || pg.PerPage != 12 {
		t.Errorf("Expected Page 4 and PerPage 12, got %d and %d", pg.Page, pg.PerPage)
	}
}

// TestMux_BindAndApiWrap validates that the Smart Binder correctly executes the handler,
// processes successful generic data, and formats errors natively.
func TestMux_BindAndApiWrap(t *testing.T) {
	r := setupMux()

	// 1. Success Endpoint
	r.HandleFunc("/success", Bind(func(req *http.Request) (any, error) {
		return map[string]string{"status": "ok"}, nil
	})).Methods(http.MethodGet)

	// 2. Error Endpoint
	r.HandleFunc("/error", Bind(func(req *http.Request) (any, error) {
		return nil, statusw.InvalidReqParam.WithCustomMessage("Bad request provided")
	})).Methods(http.MethodGet)

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

	if recErr.Code != http.StatusBadRequest {
		t.Errorf("Expected HTTP 400, got %d", recErr.Code)
	}
}

// TestMux_FileStream verifies that returning a FileResponse struct
// correctly triggers a file download stream instead of a JSON response.
func TestMux_FileStream(t *testing.T) {
	r := setupMux()
	expectedContent := "mux-file-content"

	r.HandleFunc("/download", Bind(func(req *http.Request) (any, error) {
		return responsew.FileResponse{
			ContentType: "text/plain",
			Filename:    "mux.txt",
			Reader:      bytes.NewBufferString(expectedContent),
		}, nil
	})).Methods(http.MethodGet)

	req := httptest.NewRequest(http.MethodGet, "/download", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}

	contentDisp := rec.Header().Get("Content-Disposition")
	if contentDisp != "attachment; filename=mux.txt" {
		t.Errorf("Invalid Content-Disposition header")
	}

	body, _ := io.ReadAll(rec.Body)
	if string(body) != expectedContent {
		t.Errorf("Expected file content '%s', got '%s'", expectedContent, string(body))
	}
}

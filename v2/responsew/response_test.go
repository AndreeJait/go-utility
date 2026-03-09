package responsew

import (
	"errors"
	"net/http"
	"testing"

	"github.com/AndreeJait/go-utility/v2/statusw"
)

func TestSuccess(t *testing.T) {
	resp := Success(map[string]string{"user": "alice"}, "")

	if resp.StatusCode != StatusSuccess {
		t.Errorf("Expected status code 0, got %d", resp.StatusCode)
	}
	if resp.Message != "Success" {
		t.Errorf("Expected message 'Success', got '%s'", resp.Message)
	}
	if resp.Error != nil {
		t.Errorf("Expected error to be nil")
	}
}

func TestPaginationLogic(t *testing.T) {
	pg := Pagination{Page: 3, PerPage: 15}

	if pg.GetLimit() != 15 {
		t.Errorf("Expected limit 15, got %d", pg.GetLimit())
	}
	// Offset for page 3 with 15 items per page should be (3-1)*15 = 30
	if pg.GetOffset() != 30 {
		t.Errorf("Expected offset 30, got %d", pg.GetOffset())
	}

	resp := pg.ToResponse([]string{"a", "b"}, 100, "Fetched")
	data := resp.Data.(PaginatedData)

	if data.Page != 3 || data.PageSize != 15 || data.TotalCount != 100 {
		t.Errorf("Pagination data mapping failed")
	}
	if !data.HasNextPage {
		t.Errorf("Expected has_next_page to be true (45 < 100)")
	}
}

func TestErrorResponse_WithStatusw(t *testing.T) {
	customErr := statusw.NotFound.
		WithCustomCode("USER-404").
		WithCustomMessage("User not found")

	httpCode, resp := Error(customErr)

	if httpCode != http.StatusNotFound {
		t.Errorf("Expected HTTP 404, got %d", httpCode)
	}
	if resp.StatusCode != StatusError {
		t.Errorf("Expected AppStatus 1, got %d", resp.StatusCode)
	}
	if resp.Error.Code != "USER-404" {
		t.Errorf("Expected error code 'USER-404', got %v", resp.Error.Code)
	}
	if resp.Error.Details != "User not found" {
		t.Errorf("Expected error details 'User not found', got %v", resp.Error.Details)
	}
}

func TestErrorResponse_WithStandardError(t *testing.T) {
	stdErr := errors.New("unexpected database panic")

	httpCode, resp := Error(stdErr)

	if httpCode != http.StatusInternalServerError {
		t.Errorf("Expected HTTP 500, got %d", httpCode)
	}
	if resp.Error.Code != http.StatusInternalServerError {
		t.Errorf("Expected error code 500, got %v", resp.Error.Code)
	}
}

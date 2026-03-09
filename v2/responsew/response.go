package responsew

import (
	"errors"
	"io"
	"net/http"

	"github.com/AndreeJait/go-utility/v2/statusw"
)

// AppStatus defines the application-level status code.
// 0 indicates Success, 1 indicates an Error.
type AppStatus int

const (
	StatusSuccess AppStatus = 0
	StatusError   AppStatus = 1
)

// ErrorDetail represents the structured error payload.
type ErrorDetail struct {
	Code    any    `json:"code"`    // Business logic error code (e.g., "USER-404" or 500)
	Details string `json:"details"` // Human-readable error message
}

// BaseResponse represents the standard JSON structure for all API responses.
type BaseResponse struct {
	StatusCode AppStatus    `json:"status_code"` // 0 for success, 1 for error
	Message    string       `json:"message"`
	Error      *ErrorDetail `json:"error"` // Nullable
	Data       any          `json:"data"`  // Nullable or Object
}

// PaginatedData represents the "data" object for list responses requiring pagination.
type PaginatedData struct {
	Items       any   `json:"items"`
	TotalCount  int64 `json:"total_count"`
	Page        int   `json:"page"`
	PageSize    int   `json:"page_size"`
	HasNextPage bool  `json:"has_next_page"`
}

// FileResponse is a special wrapper used to signal the HTTP framework
// to stream a file download instead of returning JSON.
type FileResponse struct {
	ContentType string
	Filename    string
	Reader      io.Reader
}

// Pagination holds the standard pagination parameters extracted from a request.
type Pagination struct {
	Page    int
	PerPage int
}

// GetLimit returns the SQL limit (which is equal to PerPage).
func (p *Pagination) GetLimit() int {
	return p.PerPage
}

// GetOffset returns the SQL offset calculated from Page and PerPage.
func (p *Pagination) GetOffset() int {
	return (p.Page - 1) * p.PerPage
}

// ToResponse formats the retrieved data into a PaginatedData BaseResponse.
func (p *Pagination) ToResponse(items any, totalCount int64, msg string) BaseResponse {
	return SuccessPaginated(items, totalCount, p.Page, p.PerPage, msg)
}

// HandlerExecutor defines the contract for executing API logic.
// The ApiWrap function calls this interface to retrieve data or an error.
type HandlerExecutor interface {
	Handle() (any, error)
}

// ExecutorFunc is an adapter to allow the use of ordinary functions as HandlerExecutors.
type ExecutorFunc func() (any, error)

// Handle calls the underlying function.
func (f ExecutorFunc) Handle() (any, error) {
	return f()
}

// Success builds a standard successful response payload.
func Success(data any, msg string) BaseResponse {
	if msg == "" {
		msg = "Success"
	}
	if data == nil {
		data = map[string]interface{}{}
	}

	return BaseResponse{
		StatusCode: StatusSuccess,
		Message:    msg,
		Error:      nil,
		Data:       data,
	}
}

// SuccessPaginated builds a standard paginated response payload.
func SuccessPaginated(items any, totalCount int64, page, pageSize int, msg string) BaseResponse {
	if msg == "" {
		msg = "Success"
	}
	if items == nil {
		items = []interface{}{}
	}

	hasNextPage := (int64(page) * int64(pageSize)) < totalCount

	return BaseResponse{
		StatusCode: StatusSuccess,
		Message:    msg,
		Error:      nil,
		Data: PaginatedData{
			Items:       items,
			TotalCount:  totalCount,
			Page:        page,
			PageSize:    pageSize,
			HasNextPage: hasNextPage,
		},
	}
}

// Error builds a standard error response and determines the appropriate HTTP status code.
// It seamlessly integrates with the statusw package.
func Error(err error) (httpStatusCode int, resp BaseResponse) {
	var statusErr *statusw.Error

	resp = BaseResponse{
		StatusCode: StatusError,
		Message:    "Error occurred",
	}
	httpStatusCode = http.StatusInternalServerError
	if errors.As(err, &statusErr) {
		code := int(statusErr.LogicalCode)

		for code > 999 {
			code /= 10
		}
		
		if code >= 100 && code <= 599 {
			httpStatusCode = code
		}
		resp.Error = &ErrorDetail{
			Code:    statusErr.CustomCode,
			Details: statusErr.Message,
		}
		resp.Data = statusErr.Data
		return httpStatusCode, resp
	}

	resp.Error = &ErrorDetail{
		Code:    http.StatusInternalServerError,
		Details: err.Error(),
	}
	resp.Data = nil

	return httpStatusCode, resp
}

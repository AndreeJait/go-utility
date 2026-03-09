package statusw

import (
	"errors"
	"testing"
)

func TestErrorChainingAndImmutability(t *testing.T) {
	baseErr := NotFound

	customErr := baseErr.
		WithCustomCode("USER-404").
		WithCustomMessage("User ID 123 is not found").
		WithError(errors.New("db record not found")).
		WithData(map[string]string{"user_id": "123"})

	if customErr.CustomCode != "USER-404" {
		t.Errorf("Expected customErr code to be USER-404, got %s", customErr.CustomCode)
	}
	if customErr.Message != "User ID 123 is not found" {
		t.Errorf("Expected customErr message to be updated, got %s", customErr.Message)
	}
	if customErr.Cause == nil || customErr.Cause.Error() != "db record not found" {
		t.Errorf("Expected customErr to wrap the original db error")
	}
	if customErr.Data == nil {
		t.Errorf("Expected customErr to contain Data")
	}

	// Verify the global base error was NOT mutated
	if baseErr.CustomCode != "NOT_FOUND" {
		t.Errorf("Global NotFound error was mutated! Expected NOT_FOUND, got %s", baseErr.CustomCode)
	}
	if baseErr.Data != nil {
		t.Errorf("Global NotFound Data was mutated! Expected nil")
	}
}

func TestErrorFormatting_WithData(t *testing.T) {
	validationErrors := map[string]string{
		"email":    "invalid format",
		"password": "too short",
	}

	err := InvalidReqParam.
		WithCustomCode("VAL-001").
		WithCustomMessage("Validation failed").
		WithData(validationErrors)

	resp := err.ToResponse()
	errMap := resp["error"].(map[string]interface{})

	if errMap["code"] != "VAL-001" {
		t.Errorf("Expected code VAL-001")
	}

	// Check if data exists and is mapped correctly
	data, ok := errMap["data"].(map[string]string)
	if !ok || data["email"] != "invalid format" {
		t.Errorf("ToResponse formatting failed to include data correctly: %v", resp)
	}
}

func TestErrorFormatting_WithoutData(t *testing.T) {
	err := InvalidReqParam.
		WithCustomCode("VAL-002").
		WithCustomMessage("Validation failed")

	resp := err.ToResponse()
	errMap := resp["error"].(map[string]interface{})

	// Check that 'data' key does NOT exist
	if _, exists := errMap["data"]; exists {
		t.Errorf("Expected 'data' key to be omitted when nil, but it was present: %v", resp)
	}
}

func TestErrorUnwrapping(t *testing.T) {
	rootCause := errors.New("connection timeout")
	err := InternalServerError.WithError(rootCause)

	if !errors.Is(err, rootCause) {
		t.Errorf("errors.Is failed to unwrap and match the root cause")
	}
}

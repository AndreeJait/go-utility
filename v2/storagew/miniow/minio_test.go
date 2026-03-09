package miniow

import (
	"testing"

	"github.com/AndreeJait/go-utility/v2/storagew"
)

// TestInterfaceCompliance ensures minioStorage strictly adheres to storagew.Storage.
// If the interface methods drift, the compiler will catch it here immediately.
func TestInterfaceCompliance(t *testing.T) {
	var _ storagew.Storage = (*minioStorage)(nil)
}

func TestNewConfigValidation(t *testing.T) {
	cfg := &Config{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "admin",
		SecretAccessKey: "password",
		UseSSL:          false,
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MinIO client struct: %v", err)
	}
	if client == nil {
		t.Errorf("Expected client to be initialized, got nil")
	}
}

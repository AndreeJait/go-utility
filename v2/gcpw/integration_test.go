//go:build integration

package gcpw

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

const testAudience = ""

// TestIntegration_InvokeOllama_GCloud calls the Cloud Run Ollama service using
// an identity token obtained via "gcloud auth print-identity-token".
//
// Prerequisites:
//   - gcloud CLI installed and authenticated (gcloud auth login)
//   - The gcloud account must have roles/run.invoker on the target service
//
// Run:
//
//	cd v2 && go test ./gcpw/ -tags=integration -run TestIntegration_InvokeOllama_GCloud -v -timeout 120s
func TestIntegration_InvokeOllama_GCloud(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tp := NewGCloudTokenProvider(nil)
	defer tp.Close()

	client := AuthenticatedHTTPClient(tp, &http.Client{Timeout: 90 * time.Second})

	body := bytes.NewBufferString(`{"model":"qwen3.5:9b","prompt":"Hello this is testing! can you answer this question for me?","stream":false}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, testAudience+"/api/generate", body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	t.Logf("response status: %d", resp.StatusCode)
	t.Logf("response body: %s", string(respBody))

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

package outline

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestRetryAfterMessage(t *testing.T) {
	if got := retryAfterMessage("3"); got != "3s" {
		t.Fatalf("retryAfterMessage() = %q, want %q", got, "3s")
	}
	if got := retryAfterMessage("Wed, 21 Oct 2015 07:28:00 GMT"); got != "Wed, 21 Oct 2015 07:28:00 GMT" {
		t.Fatalf("retryAfterMessage() = %q", got)
	}
}

func TestResponseErrorUnauthorized(t *testing.T) {
	err := responseError(&http.Response{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized", Header: http.Header{}}, nil)
	if err == nil {
		t.Fatal("responseError() expected error")
	}
	if got := err.Error(); got != "outline request failed: unauthorized; check the API key with 'outline auth login'" {
		t.Fatalf("error = %q", got)
	}
}

func TestErrorMessageHandlesTopLevelError(t *testing.T) {
	got := errorMessage([]byte(`{"error":"invalid request"}`))
	if got != "invalid request" {
		t.Fatalf("errorMessage() = %q, want %q", got, "invalid request")
	}
}

func TestErrorMessageHandlesNestedErrorEnvelope(t *testing.T) {
	got := errorMessage([]byte(`{"error":{"message":"Validation failed","name":"invalid_request"}}`))
	if got != "Validation failed" {
		t.Fatalf("errorMessage() = %q, want %q", got, "Validation failed")
	}
}

func TestDocumentText(t *testing.T) {
	text, err := DocumentText(map[string]any{"data": map[string]any{"text": "# Hello"}})
	if err != nil {
		t.Fatalf("DocumentText() error = %v", err)
	}
	if text != "# Hello" {
		t.Fatalf("DocumentText() = %q, want %q", text, "# Hello")
	}
}

func TestWriteMultipartFile(t *testing.T) {
	tmp := t.TempDir() + "/import.md"
	if err := os.WriteFile(tmp, []byte("# Hello"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writeMultipartFile(writer, FilePart{FieldName: "file", Path: tmp, ContentType: "text/markdown"}); err != nil {
		t.Fatalf("writeMultipartFile() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	content := body.String()
	if !strings.Contains(content, `name="file"`) {
		t.Fatalf("multipart body missing file field: %q", content)
	}
	if !strings.Contains(content, "# Hello") {
		t.Fatalf("multipart body missing file contents: %q", content)
	}
}

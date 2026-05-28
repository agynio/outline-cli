package outline

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestPostNilPayloadSendsJSONObject(t *testing.T) {
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		requestBody = string(body)
		_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	if _, err := client.Post(t.Context(), "collections.list", nil); err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if requestBody != "{}" {
		t.Fatalf("request body = %q, want {}", requestBody)
	}
}

func TestRetryAfterMessage(t *testing.T) {
	if got := retryAfterMessage("3"); got != "3s" {
		t.Fatalf("retryAfterMessage() = %q, want %q", got, "3s")
	}
	if got := retryAfterMessage("Wed, 21 Oct 2015 07:28:00 GMT"); got != "Wed, 21 Oct 2015 07:28:00 GMT" {
		t.Fatalf("retryAfterMessage() = %q", got)
	}
}

func TestResponseErrorUnauthorized(t *testing.T) {
	err := responseError(&http.Response{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized", Header: http.Header{}}, nil, RequestContext{})
	if err == nil {
		t.Fatal("responseError() expected error")
	}
	if got := err.Error(); got != "outline request failed: unauthorized; check the API key with 'outline auth login'" {
		t.Fatalf("error = %q", got)
	}
}

func TestResponseErrorIncludesMethodAndParams(t *testing.T) {
	err := responseError(
		&http.Response{StatusCode: http.StatusForbidden, Status: "403 Forbidden", Header: http.Header{}},
		nil,
		RequestContext{Method: "comments.list", Params: map[string]any{"documentId": "doc123"}},
	)
	if err == nil {
		t.Fatal("responseError() expected error")
	}
	got := err.Error()
	for _, want := range []string{"method=comments.list", "documentId=doc123", "forbidden"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error = %q, want substring %q", got, want)
		}
	}
}

func TestResponseErrorNotFoundDoesNotAlwaysSayUnsupported(t *testing.T) {
	err := responseError(
		&http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Header: http.Header{}},
		[]byte(`{"error":"Resource not found"}`),
		RequestContext{Method: "shares.info", Params: map[string]any{"id": "dummy"}},
	)
	if err == nil {
		t.Fatal("responseError() expected error")
	}
	got := err.Error()
	if strings.Contains(got, "unsupported") {
		t.Fatalf("error = %q, should not say unsupported", got)
	}
	for _, want := range []string{"method=shares.info", "id=dummy", "not found"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error = %q, want substring %q", got, want)
		}
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

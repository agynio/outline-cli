package outline

import (
	"net/http"
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

func TestDocumentText(t *testing.T) {
	text, err := DocumentText(map[string]any{"data": map[string]any{"text": "# Hello"}})
	if err != nil {
		t.Fatalf("DocumentText() error = %v", err)
	}
	if text != "# Hello" {
		t.Fatalf("DocumentText() = %q, want %q", text, "# Hello")
	}
}

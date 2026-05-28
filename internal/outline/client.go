package outline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{Transport: &authTransport{
			base:  http.DefaultTransport,
			token: token,
		}},
	}
}

func (c *Client) Post(ctx context.Context, method string, payload any) (map[string]any, error) {
	body := []byte("{}")
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		body = encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL(method), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("outline request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError(resp, data)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return decoded, nil
}

func (c *Client) methodURL(method string) string {
	return strings.TrimRight(c.baseURL, "/") + "/" + strings.TrimLeft(method, "/")
}

type authTransport struct {
	base  http.RoundTripper
	token string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.base.RoundTrip(req)
}

func responseError(resp *http.Response, body []byte) error {
	message := strings.TrimSpace(string(body))
	if extracted := errorMessage(body); extracted != "" {
		message = extracted
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("outline request failed: unauthorized; check the API key with 'outline auth login'")
	case http.StatusForbidden:
		return fmt.Errorf("outline request failed: forbidden; the API key does not have access to this resource")
	case http.StatusTooManyRequests:
		if retryAfter := retryAfterMessage(resp.Header.Get("Retry-After")); retryAfter != "" {
			return fmt.Errorf("outline request failed: rate limited; retry after %s", retryAfter)
		}
		return fmt.Errorf("outline request failed: rate limited")
	case http.StatusNotFound:
		if message == "" {
			return fmt.Errorf("outline request failed: unsupported on this server or not found")
		}
		return fmt.Errorf("outline request failed: unsupported on this server or not found: %s", message)
	}

	if message == "" {
		return fmt.Errorf("outline request failed: %s", resp.Status)
	}
	return fmt.Errorf("outline request failed: %s: %s", resp.Status, message)
}

func errorMessage(body []byte) string {
	var decoded struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ""
	}
	if decoded.Message != "" {
		return decoded.Message
	}
	return decoded.Error
}

func retryAfterMessage(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if seconds, err := strconv.Atoi(trimmed); err == nil {
		return (time.Duration(seconds) * time.Second).String()
	}
	return trimmed
}

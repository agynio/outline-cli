package outline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type FilePart struct {
	FieldName   string
	Path        string
	ContentType string
}

type BinaryResponse struct {
	Body        []byte
	ContentType string
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

	resp, err := c.do(ctx, method, bytes.NewReader(body), "application/json", "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := readResponse(resp)
	if err != nil {
		return nil, err
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return decoded, nil
}

func (c *Client) PostMultipart(ctx context.Context, method string, fields map[string]string, filePart FilePart) (map[string]any, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("write multipart field: %w", err)
		}
	}

	if err := writeMultipartFile(writer, filePart); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart body: %w", err)
	}

	resp, err := c.do(ctx, method, &body, writer.FormDataContentType(), "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := readResponse(resp)
	if err != nil {
		return nil, err
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return decoded, nil
}

func (c *Client) PostBinary(ctx context.Context, method string, payload any, accept string) (BinaryResponse, error) {
	body := []byte("{}")
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return BinaryResponse{}, fmt.Errorf("encode request: %w", err)
		}
		body = encoded
	}

	resp, err := c.do(ctx, method, bytes.NewReader(body), "application/json", accept)
	if err != nil {
		return BinaryResponse{}, err
	}
	defer resp.Body.Close()

	data, err := readResponse(resp)
	if err != nil {
		return BinaryResponse{}, err
	}
	return BinaryResponse{Body: data, ContentType: resp.Header.Get("Content-Type")}, nil
}

func (c *Client) do(ctx context.Context, method string, body io.Reader, contentType, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL(method), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("outline request: %w", err)
	}
	return resp, nil
}

func readResponse(resp *http.Response) ([]byte, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError(resp, data)
	}
	return data, nil
}

func writeMultipartFile(writer *multipart.Writer, filePart FilePart) error {
	fieldName := strings.TrimSpace(filePart.FieldName)
	if fieldName == "" {
		return fmt.Errorf("multipart file field name is required")
	}
	filePath := strings.TrimSpace(filePart.Path)
	if filePath == "" {
		return fmt.Errorf("multipart file path is required")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open multipart file: %w", err)
	}
	defer file.Close()

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(fieldName), escapeQuotes(filepath.Base(filePath))))
	contentType := strings.TrimSpace(filePart.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	header.Set("Content-Type", contentType)

	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create multipart file part: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copy multipart file: %w", err)
	}
	return nil
}

func escapeQuotes(value string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, `\"`).Replace(value)
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
		Message string          `json:"message"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ""
	}
	if decoded.Message != "" {
		return decoded.Message
	}
	if len(decoded.Error) == 0 {
		return ""
	}

	var errorText string
	if err := json.Unmarshal(decoded.Error, &errorText); err == nil {
		return errorText
	}

	var errorEnvelope struct {
		Message string `json:"message"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(decoded.Error, &errorEnvelope); err != nil {
		return ""
	}
	if errorEnvelope.Message != "" {
		return errorEnvelope.Message
	}
	return errorEnvelope.Name
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

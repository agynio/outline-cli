package outline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type RequestContext struct {
	Method string
	Params map[string]any
}

type StatusError struct {
	StatusCode int
	Status     string
	Message    string
	Request    RequestContext
	RetryAfter string
}

func (e *StatusError) Error() string {
	return responseErrorMessage(e.StatusCode, e.Status, e.Message, e.Request, e.RetryAfter)
}

func IsNotFound(err error) bool {
	var statusError *StatusError
	return errors.As(err, &statusError) && statusError.StatusCode == http.StatusNotFound
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
	body, err := encodeJSONBody(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, method, bytes.NewReader(body), "application/json", "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := readResponse(resp, RequestContext{Method: method, Params: payloadParams(payload)})
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

	data, err := readResponse(resp, RequestContext{Method: method, Params: stringMapParams(fields)})
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
	body, err := encodeJSONBody(payload)
	if err != nil {
		return BinaryResponse{}, err
	}

	resp, err := c.do(ctx, method, bytes.NewReader(body), "application/json", accept)
	if err != nil {
		return BinaryResponse{}, err
	}
	defer resp.Body.Close()

	data, err := readResponse(resp, RequestContext{Method: method, Params: payloadParams(payload)})
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

func encodeJSONBody(payload any) ([]byte, error) {
	if payload == nil {
		return []byte("{}"), nil
	}
	if payloadMap, ok := payload.(map[string]any); ok && len(payloadMap) == 0 {
		return []byte("{}"), nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	if bytes.Equal(encoded, []byte("null")) {
		return []byte("{}"), nil
	}
	return encoded, nil
}

func readResponse(resp *http.Response, request RequestContext) ([]byte, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError(resp, data, request)
	}
	return data, nil
}

func payloadParams(payload any) map[string]any {
	if payloadMap, ok := payload.(map[string]any); ok {
		return payloadMap
	}
	return nil
}

func stringMapParams(values map[string]string) map[string]any {
	params := make(map[string]any, len(values))
	for key, value := range values {
		params[key] = value
	}
	return params
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

func responseError(resp *http.Response, body []byte, request RequestContext) error {
	message := strings.TrimSpace(string(body))
	if extracted := errorMessage(body); extracted != "" {
		message = extracted
	}
	return &StatusError{StatusCode: resp.StatusCode, Status: resp.Status, Message: message, Request: request, RetryAfter: resp.Header.Get("Retry-After")}
}

func responseErrorMessage(statusCode int, status string, message string, request RequestContext, retryAfterValue string) string {
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Sprintf("outline request failed%s: unauthorized; check the API key with 'outline auth login'", requestContextLabel(request))
	case http.StatusForbidden:
		return fmt.Sprintf("outline request failed%s: forbidden; the API key does not have access to this resource", requestContextLabel(request))
	case http.StatusTooManyRequests:
		if retryAfter := retryAfterMessage(retryAfterValue); retryAfter != "" {
			return fmt.Sprintf("outline request failed%s: rate limited; retry after %s", requestContextLabel(request), retryAfter)
		}
		return fmt.Sprintf("outline request failed%s: rate limited", requestContextLabel(request))
	case http.StatusNotFound:
		if message == "" {
			if optionalEndpoint(request.Method) {
				return fmt.Sprintf("outline request failed%s: unsupported on this server", requestContextLabel(request))
			}
			return fmt.Sprintf("outline request failed%s: not found", requestContextLabel(request))
		}
		if optionalEndpoint(request.Method) || strings.Contains(strings.ToLower(message), "unsupported") {
			return fmt.Sprintf("outline request failed%s: unsupported on this server: %s", requestContextLabel(request), message)
		}
		return fmt.Sprintf("outline request failed%s: not found: %s", requestContextLabel(request), message)
	}

	if message == "" {
		return fmt.Sprintf("outline request failed%s: %s", requestContextLabel(request), status)
	}
	return fmt.Sprintf("outline request failed%s: %s: %s", requestContextLabel(request), status, message)
}

func optionalEndpoint(method string) bool {
	switch method {
	case "dataAttributes.list", "documents.insights", "documents.answerQuestion":
		return true
	default:
		return false
	}
}

func requestContextLabel(request RequestContext) string {
	parts := []string{}
	if strings.TrimSpace(request.Method) != "" {
		parts = append(parts, "method="+request.Method)
	}
	for _, key := range []string{"id", "documentId", "collectionId", "userId", "groupId", "clientId", "oauthClientId"} {
		value, ok := request.Params[key]
		if !ok {
			continue
		}
		formatted := strings.TrimSpace(fmt.Sprint(value))
		if formatted != "" {
			parts = append(parts, key+"="+formatted)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, " ") + ")"
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

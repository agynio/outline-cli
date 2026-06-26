package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/agynio/outline-cli/internal/config"
	"github.com/agynio/outline-cli/internal/output"
	"github.com/spf13/cobra"
)

func TestWithRunContext(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(withRunContext(context.Background(), &RunContext{OutputFormat: output.FormatYAML}))

	runContext, err := RunContextFrom(cmd)
	if err != nil {
		t.Fatalf("RunContextFrom() error = %v", err)
	}
	if runContext.OutputFormat != output.FormatYAML {
		t.Fatalf("OutputFormat = %q, want %q", runContext.OutputFormat, output.FormatYAML)
	}
}

func TestMarkdownInputRequiresSingleSource(t *testing.T) {
	if _, err := markdownInput("", ""); err == nil {
		t.Fatal("markdownInput() expected error for missing source")
	}
	if _, err := markdownInput("file.md", "text"); err == nil {
		t.Fatal("markdownInput() expected error for multiple sources")
	}
	got, err := markdownInput("", "# Title")
	if err != nil {
		t.Fatalf("markdownInput() error = %v", err)
	}
	if got != "# Title" {
		t.Fatalf("markdownInput() = %q, want %q", got, "# Title")
	}
}

func TestLoginConfigPreservesConfiguredOutput(t *testing.T) {
	got := loginConfig(&config.Config{BaseURL: "https://old.example.com/api", Output: "json"}, "https://wiki.example.com/api")
	if got.BaseURL != "https://wiki.example.com/api" {
		t.Fatalf("BaseURL = %q, want %q", got.BaseURL, "https://wiki.example.com/api")
	}
	if got.Output != "json" {
		t.Fatalf("Output = %q, want %q", got.Output, "json")
	}
}

func TestLoginConfigDefaultsOutput(t *testing.T) {
	got := loginConfig(nil, "https://wiki.example.com/api")
	if got.Output != config.DefaultOutput {
		t.Fatalf("Output = %q, want %q", got.Output, config.DefaultOutput)
	}
}

func TestPrintResponseUsesCommandOutput(t *testing.T) {
	cmd := &cobra.Command{}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetContext(withRunContext(context.Background(), &RunContext{OutputFormat: output.FormatYAML}))

	if err := printResponse(cmd, map[string]string{"id": "doc"}); err != nil {
		t.Fatalf("printResponse() error = %v", err)
	}
	if stdout.String() != "id: doc\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "id: doc\n")
	}
}

func TestRootCommandUsesEnvOnlyAuthWithoutWritingFiles(t *testing.T) {
	var authorization string
	var requestPath string
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"doc-1"}]}`))
	}))
	defer server.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(config.EnvBaseURL, server.URL)
	t.Setenv(config.EnvAPIKey, "env-token")

	root := newRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stdout)
	root.SetArgs([]string{"documents", "search", "--query", "handbook", "-o", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, stdout.String())
	}

	if requestPath != "/api/documents.search" {
		t.Fatalf("request path = %q, want /api/documents.search", requestPath)
	}
	if authorization != "Bearer env-token" {
		t.Fatalf("Authorization = %q, want Bearer env-token", authorization)
	}
	if payload["query"] != "handbook" {
		t.Fatalf("query payload = %v, want handbook", payload["query"])
	}
	assertNoAuthFiles(t, home)
}

func TestRootBaseURLFlagOverridesEnv(t *testing.T) {
	envServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("env server received request: %s", r.URL.Path)
	}))
	defer envServer.Close()

	var requestPath string
	flagServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"doc-1"}]}`))
	}))
	defer flagServer.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.EnvBaseURL, envServer.URL)
	t.Setenv(config.EnvAPIKey, "env-token")

	root := newRootCmd()
	root.SetArgs([]string{"--base-url", flagServer.URL, "documents", "search", "--query", "handbook"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if requestPath != "/api/documents.search" {
		t.Fatalf("request path = %q, want /api/documents.search", requestPath)
	}
}

func TestRootUsesEnvTokenWithSavedBaseURL(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"doc-1"}]}`))
	}))
	defer server.Close()

	home := t.TempDir()
	writeConfigFile(t, home, server.URL)
	t.Setenv("HOME", home)
	t.Setenv(config.EnvBaseURL, "")
	t.Setenv(config.EnvAPIKey, "env-token")

	root := newRootCmd()
	root.SetArgs([]string{"documents", "search", "--query", "handbook"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if authorization != "Bearer env-token" {
		t.Fatalf("Authorization = %q, want Bearer env-token", authorization)
	}
}

func TestRootUsesEnvBaseURLWithSavedToken(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"doc-1"}]}`))
	}))
	defer server.Close()

	home := t.TempDir()
	writeTokenFile(t, home, "saved-token\n")
	t.Setenv("HOME", home)
	t.Setenv(config.EnvBaseURL, server.URL)
	t.Setenv(config.EnvAPIKey, "")

	root := newRootCmd()
	root.SetArgs([]string{"documents", "search", "--query", "handbook"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if authorization != "Bearer saved-token" {
		t.Fatalf("Authorization = %q, want Bearer saved-token", authorization)
	}
}

func TestAuthConfigUsesEnvBaseURLWithoutToken(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth.config" {
			t.Fatalf("request path = %q, want /api/auth.config", r.URL.Path)
		}
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"name":"Example"}}`))
	}))
	defer server.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.EnvBaseURL, server.URL)
	t.Setenv(config.EnvAPIKey, "")

	root := newRootCmd()
	root.SetArgs([]string{"auth", "config"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if authorization != "" {
		t.Fatalf("Authorization = %q, want empty", authorization)
	}
}

func TestAuthLogoutDoesNotAffectEnvAuth(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(config.EnvBaseURL, "https://wiki.example.com")
	t.Setenv(config.EnvAPIKey, "env-token")

	root := newRootCmd()
	root.SetArgs([]string{"auth", "logout"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := os.Getenv(config.EnvAPIKey); got != "env-token" {
		t.Fatalf("%s = %q, want env-token", config.EnvAPIKey, got)
	}
	assertNoAuthFiles(t, home)
}

func writeConfigFile(t *testing.T, home string, baseURL string) {
	t.Helper()
	dir := filepath.Join(home, config.ConfigDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	data := []byte("base_url: " + baseURL + "\noutput: yaml\n")
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFile), data, 0600); err != nil {
		t.Fatalf("WriteFile() config error = %v", err)
	}
}

func writeTokenFile(t *testing.T, home string, token string) {
	t.Helper()
	dir := filepath.Join(home, config.ConfigDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.TokenFile), []byte(token), 0600); err != nil {
		t.Fatalf("WriteFile() token error = %v", err)
	}
}

func assertNoAuthFiles(t *testing.T, home string) {
	t.Helper()
	for _, name := range []string{config.ConfigFile, config.TokenFile} {
		path := filepath.Join(home, config.ConfigDir, name)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("%s exists; env auth should not write files", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("Stat() error = %v", err)
		}
	}
}

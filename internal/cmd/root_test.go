package cmd

import (
	"bytes"
	"context"
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

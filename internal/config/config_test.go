package config

import (
	"strings"
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "root", in: "https://wiki.example.com", want: "https://wiki.example.com/api"},
		{name: "root trailing slash", in: "https://wiki.example.com/", want: "https://wiki.example.com/api"},
		{name: "api", in: "https://wiki.example.com/api", want: "https://wiki.example.com/api"},
		{name: "api trailing slash", in: "https://wiki.example.com/api/", want: "https://wiki.example.com/api"},
		{name: "nested self hosted", in: "https://wiki.example.com/outline", want: "https://wiki.example.com/outline/api"},
		{name: "drops query and fragment", in: "https://wiki.example.com/?x=1#top", want: "https://wiki.example.com/api"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeBaseURL(test.in)
			if err != nil {
				t.Fatalf("NormalizeBaseURL() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("NormalizeBaseURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeBaseURLRejectsInvalidURL(t *testing.T) {
	for _, value := range []string{"", "wiki.example.com", "://wiki.example.com"} {
		if _, err := NormalizeBaseURL(value); err == nil {
			t.Fatalf("NormalizeBaseURL(%q) expected error", value)
		}
	}
}

func TestResolveBaseURLUsesFlagBeforeEnvAndConfig(t *testing.T) {
	t.Setenv(EnvBaseURL, "https://env.example.com")

	got, err := ResolveBaseURL(&Config{BaseURL: "https://saved.example.com"}, "https://flag.example.com")
	if err != nil {
		t.Fatalf("ResolveBaseURL() error = %v", err)
	}
	if got != "https://flag.example.com/api" {
		t.Fatalf("ResolveBaseURL() = %q, want flag URL", got)
	}
}

func TestResolveBaseURLUsesEnvBeforeConfig(t *testing.T) {
	t.Setenv(EnvBaseURL, "https://env.example.com")

	got, err := ResolveBaseURL(&Config{BaseURL: "https://saved.example.com"}, "")
	if err != nil {
		t.Fatalf("ResolveBaseURL() error = %v", err)
	}
	if got != "https://env.example.com/api" {
		t.Fatalf("ResolveBaseURL() = %q, want env URL", got)
	}
}

func TestResolveBaseURLUsesConfigWhenEnvUnset(t *testing.T) {
	t.Setenv(EnvBaseURL, "")

	got, err := ResolveBaseURL(&Config{BaseURL: "https://saved.example.com"}, "")
	if err != nil {
		t.Fatalf("ResolveBaseURL() error = %v", err)
	}
	if got != "https://saved.example.com/api" {
		t.Fatalf("ResolveBaseURL() = %q, want saved URL", got)
	}
}

func TestResolveBaseURLTreatsWhitespaceEnvAsUnset(t *testing.T) {
	t.Setenv(EnvBaseURL, " \t\n ")

	got, err := ResolveBaseURL(&Config{BaseURL: "https://saved.example.com"}, "")
	if err != nil {
		t.Fatalf("ResolveBaseURL() error = %v", err)
	}
	if got != "https://saved.example.com/api" {
		t.Fatalf("ResolveBaseURL() = %q, want saved URL", got)
	}
}

func TestResolveBaseURLRejectsInvalidEnvWithoutFallback(t *testing.T) {
	t.Setenv(EnvBaseURL, "wiki.example.com")

	if _, err := ResolveBaseURL(&Config{BaseURL: "https://saved.example.com"}, ""); err == nil {
		t.Fatal("ResolveBaseURL() expected error")
	}
}

func TestResolveBaseURLMissingErrorMentionsEnv(t *testing.T) {
	t.Setenv(EnvBaseURL, "")

	_, err := ResolveBaseURL(&Config{}, "")
	if err == nil {
		t.Fatal("ResolveBaseURL() expected error")
	}
	if !strings.Contains(err.Error(), EnvBaseURL) {
		t.Fatalf("error = %q, want %s", err.Error(), EnvBaseURL)
	}
}

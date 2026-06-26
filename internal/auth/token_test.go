package auth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agynio/outline-cli/internal/config"
)

func TestLoadTokenUsesEnvWithoutTokenFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.EnvAPIKey, " env-token \n")

	got, err := LoadToken(TokenOptions{})
	if err != nil {
		t.Fatalf("LoadToken() error = %v", err)
	}
	if got != "env-token" {
		t.Fatalf("LoadToken() = %q, want env token", got)
	}
}

func TestLoadTokenUsesEnvBeforeSavedToken(t *testing.T) {
	home := t.TempDir()
	writeTokenFile(t, home, "saved-token\n")
	t.Setenv("HOME", home)
	t.Setenv(config.EnvAPIKey, "env-token")

	got, err := LoadToken(TokenOptions{})
	if err != nil {
		t.Fatalf("LoadToken() error = %v", err)
	}
	if got != "env-token" {
		t.Fatalf("LoadToken() = %q, want env token", got)
	}
}

func TestLoadTokenTreatsWhitespaceEnvAsUnset(t *testing.T) {
	home := t.TempDir()
	writeTokenFile(t, home, "saved-token\n")
	t.Setenv("HOME", home)
	t.Setenv(config.EnvAPIKey, " \t\n ")

	got, err := LoadToken(TokenOptions{})
	if err != nil {
		t.Fatalf("LoadToken() error = %v", err)
	}
	if got != "saved-token" {
		t.Fatalf("LoadToken() = %q, want saved token", got)
	}
}

func TestLoadTokenMissingErrorMentionsEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.EnvAPIKey, "")

	_, err := LoadToken(TokenOptions{})
	if err == nil {
		t.Fatal("LoadToken() expected error")
	}
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("LoadToken() error = %v, want ErrTokenNotFound", err)
	}
	if !strings.Contains(err.Error(), config.EnvAPIKey) {
		t.Fatalf("error = %q, want %s", err.Error(), config.EnvAPIKey)
	}
}

func TestLoadTokenAllowMissingReturnsEmptyOnlyWhenEnvAndFileAbsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.EnvAPIKey, "")

	got, err := LoadToken(TokenOptions{AllowMissing: true})
	if err != nil {
		t.Fatalf("LoadToken() error = %v", err)
	}
	if got != "" {
		t.Fatalf("LoadToken() = %q, want empty token", got)
	}
}

func TestLoadTokenAllowMissingStillUsesEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.EnvAPIKey, "env-token")

	got, err := LoadToken(TokenOptions{AllowMissing: true})
	if err != nil {
		t.Fatalf("LoadToken() error = %v", err)
	}
	if got != "env-token" {
		t.Fatalf("LoadToken() = %q, want env token", got)
	}
}

func writeTokenFile(t *testing.T, home string, token string) {
	t.Helper()
	dir := filepath.Join(home, config.ConfigDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.TokenFile), []byte(token), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

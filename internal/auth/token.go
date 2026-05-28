package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agynio/outline-cli/internal/config"
)

var ErrTokenNotFound = errors.New("token not found")

type tokenNotFoundError struct {
	path string
}

func (e tokenNotFoundError) Error() string {
	return fmt.Sprintf("no token found; run 'outline auth login' or place an API key in %s", e.path)
}

func (e tokenNotFoundError) Is(target error) bool {
	return target == ErrTokenNotFound
}

type TokenOptions struct {
	AllowMissing bool
}

func LoadToken(opts TokenOptions) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}

	path := filepath.Join(home, config.ConfigDir, config.TokenFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if opts.AllowMissing {
				return "", nil
			}
			return "", tokenNotFoundError{path: path}
		}
		return "", fmt.Errorf("read token: %w", err)
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("empty token file: %s", path)
	}
	return token, nil
}

func SaveToken(token string) error {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return fmt.Errorf("api key is required")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	dir := filepath.Join(home, config.ConfigDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.TokenFile), []byte(trimmed+"\n"), 0600); err != nil {
		return fmt.Errorf("write token: %w", err)
	}
	return nil
}

func DeleteToken() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	path := filepath.Join(home, config.ConfigDir, config.TokenFile)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove token: %w", err)
	}
	return nil
}

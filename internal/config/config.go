package config

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDir  = ".outline-cli"
	ConfigFile = "config.yaml"
	TokenFile  = "token"

	DefaultOutput = "yaml"
)

type Config struct {
	BaseURL string `json:"base_url" yaml:"base_url"`
	Output  string `json:"output" yaml:"output"`
}

func Load() (*Config, error) {
	cfg := &Config{Output: DefaultOutput}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}

	data, err := os.ReadFile(filepath.Join(home, ConfigDir, ConfigFile))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Output == "" {
		cfg.Output = DefaultOutput
	}
	if cfg.BaseURL != "" {
		normalized, err := NormalizeBaseURL(cfg.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("normalize configured base_url: %w", err)
		}
		cfg.BaseURL = normalized
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config missing")
	}
	normalized, err := NormalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return err
	}
	output := cfg.Output
	if output == "" {
		output = DefaultOutput
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	dir := filepath.Join(home, ConfigDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	payload, err := yaml.Marshal(Config{BaseURL: normalized, Output: output})
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), payload, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func ResolveBaseURL(cfg *Config, flagURL string) (string, error) {
	if strings.TrimSpace(flagURL) != "" {
		return NormalizeBaseURL(flagURL)
	}
	if cfg == nil || strings.TrimSpace(cfg.BaseURL) == "" {
		return "", fmt.Errorf("base URL is not configured; run 'outline auth login --base-url <url> --api-key <key>'")
	}
	return NormalizeBaseURL(cfg.BaseURL)
}

func NormalizeBaseURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("base URL is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base URL must include scheme and host")
	}

	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.EscapedPath(), "/")
	if parsed.Path == "" {
		parsed.Path = "/api"
	} else if path.Base(parsed.Path) != "api" {
		parsed.Path = path.Join(parsed.Path, "api")
	}

	return parsed.String(), nil
}

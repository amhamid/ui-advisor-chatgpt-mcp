package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	OpenAIAPIKey        string  `yaml:"openai_api_key"`
	ReviewModel         string  `yaml:"review_model"`
	ImageModel          string  `yaml:"image_model"`
	ImageModelCheap     string  `yaml:"image_model_cheap"`
	MaxBudgetUSD        float64 `yaml:"max_budget_usd"`
	DailyLimitImages    int     `yaml:"daily_limit_images"`
	DefaultImageQuality string  `yaml:"default_image_quality"`
	DefaultImageSize    string  `yaml:"default_image_size"`
	AssetQuality        string  `yaml:"asset_quality"`
	SavePath            string  `yaml:"save_path"`

	// BaseDir is the directory where config.yaml lives; used to resolve relative paths.
	BaseDir string `yaml:"-"`
}

func LoadConfig(dir string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading config.yaml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config.yaml: %w", err)
	}

	cfg.BaseDir = dir
	cfg.OpenAIAPIKey = resolveEnv(cfg.OpenAIAPIKey)

	if !filepath.IsAbs(cfg.SavePath) {
		cfg.SavePath = filepath.Join(dir, cfg.SavePath)
	}

	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("openai_api_key is required (set OPENAI_API_KEY env var)")
	}

	return &cfg, nil
}

// resolveEnv replaces ${VAR} or $VAR with the environment variable value.
func resolveEnv(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		return os.Getenv(s[2 : len(s)-1])
	}
	if strings.HasPrefix(s, "$") {
		return os.Getenv(s[1:])
	}
	return s
}

// findProjectDir locates the directory containing config.yaml.
func findProjectDir() (string, error) {
	// 1. Current working directory
	if _, err := os.Stat("config.yaml"); err == nil {
		abs, _ := filepath.Abs(".")
		return abs, nil
	}

	// 2. Directory of the running executable (works for compiled binary)
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err == nil {
			return dir, nil
		}
	}

	// 3. Well-known project location
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, "dev", "ui-advisor-chatgpt-mcp")
		if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err == nil {
			return dir, nil
		}
	}

	return "", fmt.Errorf("config.yaml not found; run from the project directory or install the compiled binary next to config.yaml")
}

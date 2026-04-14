package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveEnv(t *testing.T) {
	os.Setenv("TEST_UI_ADVISOR_KEY", "sk-test-123")
	defer os.Unsetenv("TEST_UI_ADVISOR_KEY")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"dollar brace syntax", "${TEST_UI_ADVISOR_KEY}", "sk-test-123"},
		{"dollar prefix syntax", "$TEST_UI_ADVISOR_KEY", "sk-test-123"},
		{"literal value", "my-literal-key", "my-literal-key"},
		{"empty env var", "${NONEXISTENT_UI_ADVISOR_VAR}", ""},
		{"whitespace trimmed", "  ${TEST_UI_ADVISOR_KEY}  ", "sk-test-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEnv(tt.input)
			if got != tt.want {
				t.Errorf("resolveEnv(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()

	configContent := `
openai_api_key: "test-key-literal"
review_model: "gpt-5.4-mini"
image_model: "gpt-image-1"
image_model_cheap: "gpt-image-1-mini"
max_budget_usd: 5.00
daily_limit_images: 10
default_image_quality: "medium"
default_image_size: "1024x1024"
asset_quality: "high"
save_path: "./outputs"
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.OpenAIAPIKey != "test-key-literal" {
		t.Errorf("OpenAIAPIKey = %q, want %q", cfg.OpenAIAPIKey, "test-key-literal")
	}
	if cfg.ReviewModel != "gpt-5.4-mini" {
		t.Errorf("ReviewModel = %q, want %q", cfg.ReviewModel, "gpt-5.4-mini")
	}
	if cfg.MaxBudgetUSD != 5.00 {
		t.Errorf("MaxBudgetUSD = %f, want 5.00", cfg.MaxBudgetUSD)
	}
	if cfg.DailyLimitImages != 10 {
		t.Errorf("DailyLimitImages = %d, want 10", cfg.DailyLimitImages)
	}
	if cfg.DefaultImageQuality != "medium" {
		t.Errorf("DefaultImageQuality = %q, want %q", cfg.DefaultImageQuality, "medium")
	}
	if cfg.AssetQuality != "high" {
		t.Errorf("AssetQuality = %q, want %q", cfg.AssetQuality, "high")
	}

	// save_path should be resolved to absolute
	expectedSavePath := filepath.Join(dir, "outputs")
	if cfg.SavePath != expectedSavePath {
		t.Errorf("SavePath = %q, want %q", cfg.SavePath, expectedSavePath)
	}
	if cfg.BaseDir != dir {
		t.Errorf("BaseDir = %q, want %q", cfg.BaseDir, dir)
	}
}

func TestLoadConfigEnvResolution(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("TEST_OPENAI_KEY_CFG", "sk-from-env")
	defer os.Unsetenv("TEST_OPENAI_KEY_CFG")

	configContent := `
openai_api_key: "${TEST_OPENAI_KEY_CFG}"
review_model: "gpt-5.4-mini"
image_model: "gpt-image-1"
image_model_cheap: "gpt-image-1-mini"
max_budget_usd: 10.00
daily_limit_images: 30
default_image_quality: "medium"
default_image_size: "1024x1024"
asset_quality: "high"
save_path: "./outputs"
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.OpenAIAPIKey != "sk-from-env" {
		t.Errorf("OpenAIAPIKey = %q, want %q", cfg.OpenAIAPIKey, "sk-from-env")
	}
}

func TestLoadConfigMissingKey(t *testing.T) {
	dir := t.TempDir()

	configContent := `
openai_api_key: "${NONEXISTENT_KEY_FOR_TEST}"
review_model: "gpt-5.4-mini"
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for missing config.yaml, got nil")
	}
}

func TestLoadConfigAbsoluteSavePath(t *testing.T) {
	dir := t.TempDir()
	configContent := `
openai_api_key: "test-key"
review_model: "gpt-5.4-mini"
image_model: "gpt-image-1"
image_model_cheap: "gpt-image-1-mini"
max_budget_usd: 10.00
daily_limit_images: 30
default_image_quality: "medium"
default_image_size: "1024x1024"
asset_quality: "high"
save_path: "/tmp/absolute-output-path"
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.SavePath != "/tmp/absolute-output-path" {
		t.Errorf("SavePath = %q, want %q", cfg.SavePath, "/tmp/absolute-output-path")
	}
}

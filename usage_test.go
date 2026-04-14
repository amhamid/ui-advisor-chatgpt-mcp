package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func testConfig(dir string) *Config {
	return &Config{
		BaseDir:          dir,
		MaxBudgetUSD:     10.00,
		DailyLimitImages: 5,
		SavePath:         filepath.Join(dir, "outputs"),
	}
}

func TestNewUsage(t *testing.T) {
	u := newUsage()
	now := time.Now()

	if u.Month != now.Format("2006-01") {
		t.Errorf("Month = %q, want %q", u.Month, now.Format("2006-01"))
	}
	if u.DailyDate != now.Format("2006-01-02") {
		t.Errorf("DailyDate = %q, want %q", u.DailyDate, now.Format("2006-01-02"))
	}
	if u.MonthlyTotal != 0 {
		t.Errorf("MonthlyTotal = %f, want 0", u.MonthlyTotal)
	}
	if u.DailyImageCount != 0 {
		t.Errorf("DailyImageCount = %d, want 0", u.DailyImageCount)
	}
}

func TestUsageRecord(t *testing.T) {
	u := newUsage()

	u.Record("design_review", "gpt-5.4-mini", 0.01, false)
	if u.MonthlyTotal != 0.01 {
		t.Errorf("MonthlyTotal = %f, want 0.01", u.MonthlyTotal)
	}
	if u.DailyCost != 0.01 {
		t.Errorf("DailyCost = %f, want 0.01", u.DailyCost)
	}
	if u.DailyImageCount != 0 {
		t.Errorf("DailyImageCount = %d, want 0 (non-image tool)", u.DailyImageCount)
	}
	if len(u.Calls) != 1 {
		t.Errorf("len(Calls) = %d, want 1", len(u.Calls))
	}

	u.Record("generate_mockup", "gpt-image-1", 0.042, true)
	if !approxEqual(u.MonthlyTotal, 0.052) {
		t.Errorf("MonthlyTotal = %f, want 0.052", u.MonthlyTotal)
	}
	if u.DailyImageCount != 1 {
		t.Errorf("DailyImageCount = %d, want 1", u.DailyImageCount)
	}
	if len(u.Calls) != 2 {
		t.Errorf("len(Calls) = %d, want 2", len(u.Calls))
	}
	if len(u.AllMonthCalls) != 2 {
		t.Errorf("len(AllMonthCalls) = %d, want 2", len(u.AllMonthCalls))
	}
}

func TestUsageCheckLimitsMonthlyBudget(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.MaxBudgetUSD = 1.00

	u := newUsage()
	u.MonthlyTotal = 1.00

	err := u.CheckLimits(cfg, false)
	if err == nil {
		t.Fatal("expected budget exceeded error, got nil")
	}

	// Under budget should pass
	u.MonthlyTotal = 0.99
	err = u.CheckLimits(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUsageCheckLimitsDailyImages(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.DailyLimitImages = 3

	u := newUsage()
	u.DailyImageCount = 3

	// Image tool should be blocked
	err := u.CheckLimits(cfg, true)
	if err == nil {
		t.Fatal("expected daily limit error, got nil")
	}

	// Non-image tool should pass even at image limit
	err = u.CheckLimits(cfg, false)
	if err != nil {
		t.Fatalf("unexpected error for non-image tool: %v", err)
	}

	// Under limit should pass
	u.DailyImageCount = 2
	err = u.CheckLimits(cfg, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUsageDailyReset(t *testing.T) {
	u := newUsage()
	u.DailyDate = "2020-01-01" // old date
	u.DailyImageCount = 10
	u.DailyCost = 5.00
	u.Calls = []UsageCall{{Timestamp: "old", Tool: "test", Model: "test", Cost: 1.0}}
	u.MonthlyTotal = 5.00 // should NOT be reset (same month check happens separately)

	u.resetIfNeeded()

	now := time.Now()
	if u.DailyDate != now.Format("2006-01-02") {
		t.Errorf("DailyDate not reset, got %q", u.DailyDate)
	}
	if u.DailyImageCount != 0 {
		t.Errorf("DailyImageCount not reset, got %d", u.DailyImageCount)
	}
	if u.DailyCost != 0 {
		t.Errorf("DailyCost not reset, got %f", u.DailyCost)
	}
	if len(u.Calls) != 0 {
		t.Errorf("Calls not reset, got %d", len(u.Calls))
	}
}

func TestUsageMonthlyReset(t *testing.T) {
	u := newUsage()
	u.Month = "2020-01" // old month
	u.DailyDate = "2020-01-15"
	u.MonthlyTotal = 9.99
	u.DailyImageCount = 10
	u.AllMonthCalls = []UsageCall{{Timestamp: "old", Tool: "test", Model: "test", Cost: 1.0}}

	u.resetIfNeeded()

	now := time.Now()
	if u.Month != now.Format("2006-01") {
		t.Errorf("Month not reset, got %q", u.Month)
	}
	if u.MonthlyTotal != 0 {
		t.Errorf("MonthlyTotal not reset, got %f", u.MonthlyTotal)
	}
	if u.DailyImageCount != 0 {
		t.Errorf("DailyImageCount not reset, got %d", u.DailyImageCount)
	}
	if len(u.AllMonthCalls) != 0 {
		t.Errorf("AllMonthCalls not reset, got %d", len(u.AllMonthCalls))
	}
}

func TestUsageSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)

	u := newUsage()
	u.filePath = usagePath(cfg)
	u.Record("generate_mockup", "gpt-image-1", 0.042, true)
	u.Record("design_review", "gpt-5.4-mini", 0.01, false)

	if err := u.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(usagePath(cfg)); os.IsNotExist(err) {
		t.Fatal("usage.json was not created")
	}

	// Load and verify
	loaded, err := LoadUsage(cfg)
	if err != nil {
		t.Fatalf("LoadUsage failed: %v", err)
	}

	if loaded.MonthlyTotal != u.MonthlyTotal {
		t.Errorf("MonthlyTotal = %f, want %f", loaded.MonthlyTotal, u.MonthlyTotal)
	}
	if loaded.DailyImageCount != u.DailyImageCount {
		t.Errorf("DailyImageCount = %d, want %d", loaded.DailyImageCount, u.DailyImageCount)
	}
	if len(loaded.Calls) != 2 {
		t.Errorf("len(Calls) = %d, want 2", len(loaded.Calls))
	}
}

func TestUsageLoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)

	u, err := LoadUsage(cfg)
	if err != nil {
		t.Fatalf("LoadUsage for nonexistent file should not error: %v", err)
	}
	if u.MonthlyTotal != 0 {
		t.Errorf("MonthlyTotal = %f, want 0", u.MonthlyTotal)
	}
}

func TestUsageSummary(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)

	u := newUsage()
	u.Record("design_review", "gpt-5.4-mini", 0.01, false)
	u.Record("generate_mockup", "gpt-image-1", 0.042, true)

	summary := u.Summary(cfg)
	if summary == "" {
		t.Fatal("Summary returned empty string")
	}
	// Check that it contains key information
	for _, want := range []string{"Usage Summary", "Monthly spend", "Images generated today", "design_review", "generate_mockup"} {
		if !containsStr(summary, want) {
			t.Errorf("Summary missing %q", want)
		}
	}
}

func TestEstimateImageCost(t *testing.T) {
	tests := []struct {
		model, quality, size string
		want                 float64
	}{
		{"gpt-image-1", "medium", "1024x1024", 0.042},
		{"gpt-image-1", "high", "1024x1024", 0.167},
		{"gpt-image-1", "low", "1024x1024", 0.011},
		{"gpt-image-1-mini", "medium", "1024x1024", 0.019},
		{"gpt-image-1", "high", "1536x1024", 0.250},
		{"unknown-model", "medium", "1024x1024", 0.05}, // default
	}

	for _, tt := range tests {
		t.Run(tt.model+"/"+tt.quality+"/"+tt.size, func(t *testing.T) {
			got := estimateImageCost(tt.model, tt.quality, tt.size)
			if got != tt.want {
				t.Errorf("estimateImageCost(%q, %q, %q) = %f, want %f", tt.model, tt.quality, tt.size, got, tt.want)
			}
		})
	}
}

func TestEstimateVisionCost(t *testing.T) {
	cost := estimateVisionCost(1000, 500)
	if cost <= 0 {
		t.Errorf("estimateVisionCost(1000, 500) = %f, want > 0", cost)
	}
	// Expected: 1000 * 0.00000015 + 500 * 0.0000006 = 0.00015 + 0.0003 = 0.00045
	expected := 0.00045
	if cost != expected {
		t.Errorf("estimateVisionCost(1000, 500) = %f, want %f", cost, expected)
	}
}

func TestUsageJSON(t *testing.T) {
	u := newUsage()
	u.Record("test_tool", "test_model", 0.05, true)

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var loaded UsageData
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if loaded.MonthlyTotal != u.MonthlyTotal {
		t.Errorf("MonthlyTotal round-trip: got %f, want %f", loaded.MonthlyTotal, u.MonthlyTotal)
	}
	if loaded.DailyImageCount != u.DailyImageCount {
		t.Errorf("DailyImageCount round-trip: got %d, want %d", loaded.DailyImageCount, u.DailyImageCount)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

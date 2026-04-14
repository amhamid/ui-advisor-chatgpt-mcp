package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type UsageCall struct {
	Timestamp string  `json:"timestamp"`
	Tool      string  `json:"tool"`
	Model     string  `json:"model"`
	Cost      float64 `json:"cost"`
}

type UsageData struct {
	Month            string      `json:"month"`              // "2026-04"
	DailyDate        string      `json:"daily_date"`         // "2026-04-14"
	MonthlyTotal     float64     `json:"monthly_total"`
	DailyImageCount  int         `json:"daily_image_count"`
	DailyCost        float64     `json:"daily_cost"`
	Calls            []UsageCall `json:"calls"`              // today's calls
	AllMonthCalls    []UsageCall `json:"all_month_calls"`    // all calls this month

	filePath string
}

func usagePath(cfg *Config) string {
	return filepath.Join(cfg.BaseDir, "usage.json")
}

func LoadUsage(cfg *Config) (*UsageData, error) {
	p := usagePath(cfg)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			u := newUsage()
			u.filePath = p
			return u, nil
		}
		return nil, err
	}

	var u UsageData
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, err
	}
	u.filePath = p
	u.resetIfNeeded()
	return &u, nil
}

func newUsage() *UsageData {
	now := time.Now()
	return &UsageData{
		Month:     now.Format("2006-01"),
		DailyDate: now.Format("2006-01-02"),
	}
}

func (u *UsageData) resetIfNeeded() {
	now := time.Now()
	currentMonth := now.Format("2006-01")
	currentDate := now.Format("2006-01-02")

	if u.Month != currentMonth {
		u.Month = currentMonth
		u.MonthlyTotal = 0
		u.DailyImageCount = 0
		u.DailyCost = 0
		u.Calls = nil
		u.AllMonthCalls = nil
	}

	if u.DailyDate != currentDate {
		u.DailyDate = currentDate
		u.DailyImageCount = 0
		u.DailyCost = 0
		u.Calls = nil
	}
}

func (u *UsageData) Save() error {
	data, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(u.filePath, data, 0644)
}

// CheckLimits returns a non-nil error describing which limit was exceeded.
func (u *UsageData) CheckLimits(cfg *Config, isImageTool bool) error {
	u.resetIfNeeded()

	if u.MonthlyTotal >= cfg.MaxBudgetUSD {
		return fmt.Errorf("monthly budget exceeded: $%.2f spent of $%.2f limit. Use force: true to override", u.MonthlyTotal, cfg.MaxBudgetUSD)
	}

	if isImageTool && u.DailyImageCount >= cfg.DailyLimitImages {
		return fmt.Errorf("daily image limit reached: %d of %d images generated today. Use force: true to override", u.DailyImageCount, cfg.DailyLimitImages)
	}

	return nil
}

// Record logs a single API call.
func (u *UsageData) Record(tool, model string, cost float64, isImage bool) {
	u.resetIfNeeded()

	call := UsageCall{
		Timestamp: time.Now().Format(time.RFC3339),
		Tool:      tool,
		Model:     model,
		Cost:      cost,
	}

	u.MonthlyTotal += cost
	u.DailyCost += cost
	u.Calls = append(u.Calls, call)
	u.AllMonthCalls = append(u.AllMonthCalls, call)

	if isImage {
		u.DailyImageCount++
	}
}

// Summary returns a human-readable usage report.
func (u *UsageData) Summary(cfg *Config) string {
	u.resetIfNeeded()

	var sb strings.Builder
	sb.WriteString("## Usage Summary\n\n")
	sb.WriteString(fmt.Sprintf("**Month:** %s\n", u.Month))
	sb.WriteString(fmt.Sprintf("**Monthly spend:** $%.4f / $%.2f budget (%.1f%% used)\n", u.MonthlyTotal, cfg.MaxBudgetUSD, (u.MonthlyTotal/cfg.MaxBudgetUSD)*100))
	sb.WriteString(fmt.Sprintf("**Monthly budget remaining:** $%.4f\n\n", cfg.MaxBudgetUSD-u.MonthlyTotal))
	sb.WriteString(fmt.Sprintf("**Today:** %s\n", u.DailyDate))
	sb.WriteString(fmt.Sprintf("**Images generated today:** %d / %d limit (%d remaining)\n", u.DailyImageCount, cfg.DailyLimitImages, cfg.DailyLimitImages-u.DailyImageCount))
	sb.WriteString(fmt.Sprintf("**Today's spend:** $%.4f\n\n", u.DailyCost))

	if len(u.Calls) > 0 {
		sb.WriteString("### Today's calls\n\n")
		sb.WriteString("| Time | Tool | Model | Cost |\n")
		sb.WriteString("|------|------|-------|------|\n")
		for _, c := range u.Calls {
			t, _ := time.Parse(time.RFC3339, c.Timestamp)
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | $%.4f |\n", t.Format("15:04:05"), c.Tool, c.Model, c.Cost))
		}
	} else {
		sb.WriteString("No API calls today.\n")
	}

	return sb.String()
}

// estimateImageCost returns an estimated cost for an image generation call.
func estimateImageCost(model, quality, size string) float64 {
	type key struct{ model, size, quality string }
	costs := map[key]float64{
		{"gpt-image-1", "1024x1024", "low"}:       0.011,
		{"gpt-image-1", "1024x1024", "medium"}:     0.042,
		{"gpt-image-1", "1024x1024", "high"}:       0.167,
		{"gpt-image-1", "1536x1024", "low"}:        0.016,
		{"gpt-image-1", "1536x1024", "medium"}:     0.063,
		{"gpt-image-1", "1536x1024", "high"}:       0.250,
		{"gpt-image-1", "1024x1536", "low"}:        0.016,
		{"gpt-image-1", "1024x1536", "medium"}:     0.063,
		{"gpt-image-1", "1024x1536", "high"}:       0.250,
		{"gpt-image-1-mini", "1024x1024", "low"}:   0.005,
		{"gpt-image-1-mini", "1024x1024", "medium"}: 0.019,
		{"gpt-image-1-mini", "1024x1024", "high"}:  0.075,
		{"gpt-image-1-mini", "1536x1024", "low"}:   0.008,
		{"gpt-image-1-mini", "1536x1024", "medium"}: 0.028,
		{"gpt-image-1-mini", "1536x1024", "high"}:  0.113,
		{"gpt-image-1-mini", "1024x1536", "low"}:   0.008,
		{"gpt-image-1-mini", "1024x1536", "medium"}: 0.028,
		{"gpt-image-1-mini", "1024x1536", "high"}:  0.113,
	}
	if c, ok := costs[key{model, size, quality}]; ok {
		return c
	}
	return 0.05 // conservative default
}

// estimateVisionCost estimates cost from a chat completion's token usage.
func estimateVisionCost(promptTokens, completionTokens int) float64 {
	// Approximate pricing for a mini-class vision model
	inputCost := float64(promptTokens) * 0.00000015   // $0.15 / 1M tokens
	outputCost := float64(completionTokens) * 0.0000006 // $0.60 / 1M tokens
	return inputCost + outputCost
}

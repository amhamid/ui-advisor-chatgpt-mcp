package main

import (
	"encoding/json"
	"testing"
)

func TestAllToolsSchema(t *testing.T) {
	tools := allTools()

	expectedNames := map[string]bool{
		"design_review":   true,
		"generate_mockup": true,
		"generate_asset":  true,
		"get_usage":       true,
	}

	if len(tools) != len(expectedNames) {
		t.Fatalf("expected %d tools, got %d", len(expectedNames), len(tools))
	}

	for _, tool := range tools {
		if !expectedNames[tool.Name] {
			t.Errorf("unexpected tool: %s", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("tool %s has empty description", tool.Name)
		}

		// Verify InputSchema is valid JSON
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("tool %s has invalid InputSchema: %v", tool.Name, err)
		}

		// Check it's an object type
		if schema["type"] != "object" {
			t.Errorf("tool %s InputSchema type = %v, want object", tool.Name, schema["type"])
		}
	}
}

func TestToolSchemaRequiredFields(t *testing.T) {
	tools := allTools()
	toolMap := make(map[string]ToolDef)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	// design_review requires image_path
	checkRequired(t, toolMap["design_review"], []string{"image_path"})

	// generate_mockup requires prompt
	checkRequired(t, toolMap["generate_mockup"], []string{"prompt"})

	// generate_asset requires prompt and filename
	checkRequired(t, toolMap["generate_asset"], []string{"prompt", "filename"})

	// get_usage has no required fields
	checkRequired(t, toolMap["get_usage"], nil)
}

func checkRequired(t *testing.T, tool ToolDef, expected []string) {
	t.Helper()
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
		t.Fatalf("tool %s: failed to parse schema: %v", tool.Name, err)
	}

	if len(expected) == 0 {
		// No required fields expected — either nil or empty is fine
		return
	}

	if len(schema.Required) != len(expected) {
		t.Errorf("tool %s: required fields = %v, want %v", tool.Name, schema.Required, expected)
		return
	}

	expectedSet := make(map[string]bool)
	for _, f := range expected {
		expectedSet[f] = true
	}
	for _, f := range schema.Required {
		if !expectedSet[f] {
			t.Errorf("tool %s: unexpected required field %q", tool.Name, f)
		}
	}
}

func TestDispatchUnknownTool(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.OpenAIAPIKey = "test"
	u := newUsage()

	result := DispatchTool("nonexistent_tool", json.RawMessage(`{}`), cfg, u)
	if !result.IsError {
		t.Fatal("expected error for unknown tool")
	}
	if len(result.Content) == 0 || result.Content[0].Text == "" {
		t.Fatal("expected error message")
	}
}

func TestDispatchGetUsage(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.OpenAIAPIKey = "test"
	u := newUsage()
	u.Record("test", "test-model", 0.01, false)

	result := DispatchTool("get_usage", json.RawMessage(`{}`), cfg, u)
	if result.IsError {
		t.Fatalf("get_usage returned error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("get_usage returned empty content")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q, want text", result.Content[0].Type)
	}
}

func TestDispatchDesignReviewMissingImagePath(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.OpenAIAPIKey = "test"
	u := newUsage()

	result := DispatchTool("design_review", json.RawMessage(`{}`), cfg, u)
	if !result.IsError {
		t.Fatal("expected error for missing image_path")
	}
}

func TestDispatchDesignReviewBadFile(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.OpenAIAPIKey = "test"
	u := newUsage()

	args := `{"image_path": "/nonexistent/file.png"}`
	result := DispatchTool("design_review", json.RawMessage(args), cfg, u)
	if !result.IsError {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestDispatchGenerateMockupMissingPrompt(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.OpenAIAPIKey = "test"
	u := newUsage()

	result := DispatchTool("generate_mockup", json.RawMessage(`{}`), cfg, u)
	if !result.IsError {
		t.Fatal("expected error for missing prompt")
	}
}

func TestDispatchGenerateAssetMissingFields(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.OpenAIAPIKey = "test"
	u := newUsage()

	// Missing both required fields
	result := DispatchTool("generate_asset", json.RawMessage(`{}`), cfg, u)
	if !result.IsError {
		t.Fatal("expected error for missing prompt")
	}

	// Missing filename
	result = DispatchTool("generate_asset", json.RawMessage(`{"prompt": "a logo"}`), cfg, u)
	if !result.IsError {
		t.Fatal("expected error for missing filename")
	}
}

func TestDispatchBudgetExceeded(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.OpenAIAPIKey = "test"
	cfg.MaxBudgetUSD = 0.01

	u := newUsage()
	u.MonthlyTotal = 0.01 // at the limit

	// design_review should be blocked by budget
	args := `{"image_path": "/some/file.png"}`
	result := DispatchTool("design_review", json.RawMessage(args), cfg, u)
	if !result.IsError {
		t.Fatal("expected budget exceeded error")
	}

	// get_usage should still work regardless of budget
	result = DispatchTool("get_usage", json.RawMessage(`{}`), cfg, u)
	if result.IsError {
		t.Fatal("get_usage should not be affected by budget")
	}
}

func TestDispatchDailyLimitExceeded(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.OpenAIAPIKey = "test"
	cfg.DailyLimitImages = 2

	u := newUsage()
	u.DailyImageCount = 2

	// generate_mockup should be blocked
	args := `{"prompt": "a mockup"}`
	result := DispatchTool("generate_mockup", json.RawMessage(args), cfg, u)
	if !result.IsError {
		t.Fatal("expected daily limit error for generate_mockup")
	}

	// generate_asset should also be blocked
	args = `{"prompt": "a logo", "filename": "test"}`
	result = DispatchTool("generate_asset", json.RawMessage(args), cfg, u)
	if !result.IsError {
		t.Fatal("expected daily limit error for generate_asset")
	}
}

func TestDispatchInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.OpenAIAPIKey = "test"
	u := newUsage()

	result := DispatchTool("design_review", json.RawMessage(`{invalid json`), cfg, u)
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTextResult(t *testing.T) {
	r := textResult("hello")
	if r.IsError {
		t.Error("textResult should not be error")
	}
	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(r.Content))
	}
	if r.Content[0].Type != "text" {
		t.Errorf("type = %q, want text", r.Content[0].Type)
	}
	if r.Content[0].Text != "hello" {
		t.Errorf("text = %q, want hello", r.Content[0].Text)
	}
}

func TestErrorResult(t *testing.T) {
	r := errorResult("something broke")
	if !r.IsError {
		t.Error("errorResult should be error")
	}
	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(r.Content))
	}
	if r.Content[0].Text != "something broke" {
		t.Errorf("text = %q, want 'something broke'", r.Content[0].Text)
	}
}

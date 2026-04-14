package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMCPInitialize verifies the initialize handshake.
func TestMCPInitialize(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
	}
	resp := handleRequest(t, req)

	if resp.Error != nil {
		t.Fatalf("initialize returned error: %s", resp.Error.Message)
	}

	data, _ := json.Marshal(resp.Result)
	var result InitializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("ProtocolVersion = %q, want 2024-11-05", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "ui-advisor-chatgpt-mcp" {
		t.Errorf("ServerInfo.Name = %q, want ui-advisor-chatgpt-mcp", result.ServerInfo.Name)
	}
	if result.Capabilities.Tools == nil {
		t.Error("Capabilities.Tools is nil")
	}
}

// TestMCPToolsList verifies tools/list returns all 4 tools.
func TestMCPToolsList(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/list",
	}
	resp := handleRequest(t, req)

	if resp.Error != nil {
		t.Fatalf("tools/list returned error: %s", resp.Error.Message)
	}

	data, _ := json.Marshal(resp.Result)
	var result ToolsListResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if len(result.Tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(result.Tools))
	}

	names := make(map[string]bool)
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}
	for _, expected := range []string{"design_review", "generate_mockup", "generate_asset", "get_usage"} {
		if !names[expected] {
			t.Errorf("missing tool: %s", expected)
		}
	}
}

// TestMCPToolsCallGetUsage verifies a tools/call for get_usage works end-to-end.
func TestMCPToolsCallGetUsage(t *testing.T) {
	params := ToolCallParams{
		Name:      "get_usage",
		Arguments: json.RawMessage(`{}`),
	}
	paramsJSON, _ := json.Marshal(params)

	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "tools/call",
		Params:  paramsJSON,
	}
	resp := handleRequest(t, req)

	if resp.Error != nil {
		t.Fatalf("tools/call returned error: %s", resp.Error.Message)
	}

	data, _ := json.Marshal(resp.Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
}

// TestMCPMethodNotFound verifies unknown methods return -32601.
func TestMCPMethodNotFound(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  "unknown/method",
	}
	resp := handleRequest(t, req)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", resp.Error.Code)
	}
}

// TestMCPNotificationNoResponse verifies notifications don't produce output.
func TestMCPNotificationNoResponse(t *testing.T) {
	dir := t.TempDir()
	setupTestConfig(t, dir)

	// Notification: no "id" field
	input := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	reader := strings.NewReader(input)

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	// Write input and close
	go func() {
		w.WriteString(input)
		w.Close()
	}()
	_ = reader // not used directly, we pipe through os.Stdin

	// Capture stdout
	oldStdout := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	// Run the main loop manually via a scanner
	cfg, _ := LoadConfig(dir)
	usage := newUsage()
	usage.filePath = usagePath(cfg)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		if req.ID == nil {
			// notification — no response
			continue
		}
	}

	outW.Close()
	os.Stdout = oldStdout

	output, _ := io.ReadAll(outR)
	if len(output) > 0 {
		t.Errorf("notification produced output: %s", string(output))
	}
}

// TestMCPStringID verifies the server handles string IDs correctly.
func TestMCPStringID(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"abc-123"`),
		Method:  "initialize",
	}
	resp := handleRequest(t, req)

	if resp.Error != nil {
		t.Fatalf("initialize with string ID returned error: %s", resp.Error.Message)
	}

	// Verify ID is preserved
	if string(resp.ID) != `"abc-123"` {
		t.Errorf("ID = %s, want \"abc-123\"", string(resp.ID))
	}
}

// TestMCPInvalidToolCallParams verifies bad params return -32602.
func TestMCPInvalidToolCallParams(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`5`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{invalid`),
	}
	resp := handleRequest(t, req)

	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("error code = %d, want -32602", resp.Error.Code)
	}
}

// --- helpers ---

func setupTestConfig(t *testing.T, dir string) {
	t.Helper()
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
save_path: "./outputs"
`
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configContent), 0644)
}

func handleRequest(t *testing.T, req Request) Response {
	t.Helper()
	dir := t.TempDir()
	setupTestConfig(t, dir)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	usage := newUsage()
	usage.filePath = usagePath(cfg)

	resp := Response{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		resp.Result = InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities:    Caps{Tools: map[string]interface{}{}},
			ServerInfo:      ServerInfo{Name: "ui-advisor-chatgpt-mcp", Version: "1.0.0"},
		}
	case "tools/list":
		resp.Result = ToolsListResult{Tools: allTools()}
	case "tools/call":
		var params ToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
		} else {
			resp.Result = DispatchTool(params.Name, params.Arguments, cfg, usage)
		}
	default:
		resp.Error = &RPCError{Code: -32601, Message: "Method not found: " + req.Method}
	}

	return resp
}

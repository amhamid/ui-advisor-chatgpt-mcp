package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// --- JSON-RPC types ---

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- MCP types ---

type InitializeResult struct {
	ProtocolVersion string     `json:"protocolVersion"`
	Capabilities    Caps       `json:"capabilities"`
	ServerInfo      ServerInfo `json:"serverInfo"`
}

type Caps struct {
	Tools map[string]interface{} `json:"tools"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ToolsListResult struct {
	Tools []ToolDef `json:"tools"`
}

type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("[ui-advisor] ")

	dir, err := findProjectDir()
	if err != nil {
		log.Fatalf("Failed to find project directory: %v", err)
	}
	log.Printf("project dir: %s", dir)

	cfg, err := LoadConfig(dir)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	usage, err := LoadUsage(cfg)
	if err != nil {
		log.Printf("Warning: failed to load usage, starting fresh: %v", err)
		usage = newUsage()
		usage.filePath = usagePath(cfg)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0), 10*1024*1024) // 10 MB buffer for large messages

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			log.Printf("Failed to parse request: %v", err)
			continue
		}

		// Notifications (no id) don't get a response.
		if req.ID == nil {
			log.Printf("notification: %s", req.Method)
			continue
		}

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
				resp.Error = &RPCError{Code: -32602, Message: fmt.Sprintf("Invalid params: %v", err)}
			} else {
				resp.Result = DispatchTool(params.Name, params.Arguments, cfg, usage)
			}

		default:
			resp.Error = &RPCError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
		}

		out, _ := json.Marshal(resp)
		fmt.Fprintf(os.Stdout, "%s\n", out)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin read error: %v", err)
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"log"
)

// ToolDef is the MCP tool definition returned by tools/list.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

func allTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "design_review",
			Description: "Review a UI screenshot using GPT vision. Returns specific, actionable design feedback with concrete values (padding, colors, font weights).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"image_path": {
						"type": "string",
						"description": "Path to the screenshot or image file to review"
					},
					"context": {
						"type": "string",
						"description": "What this screen is and what app it's for"
					},
					"focus": {
						"type": "string",
						"description": "Specific aspect to review: spacing, typography, color, hierarchy, or overall",
						"enum": ["spacing", "typography", "color", "hierarchy", "overall"]
					},
					"force": {
						"type": "boolean",
						"description": "If true, bypass budget and daily limit checks",
						"default": false
					}
				},
				"required": ["image_path"]
			}`),
		},
		{
			Name:        "generate_mockup",
			Description: "Generate a UI mockup image using GPT Image. Returns the absolute file path of the saved PNG.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"prompt": {
						"type": "string",
						"description": "Description of the UI to generate"
					},
					"reference_image_path": {
						"type": "string",
						"description": "Existing screenshot to use as reference for redesign"
					},
					"size": {
						"type": "string",
						"description": "Image size: 1024x1024, 1536x1024, or 1024x1536",
						"enum": ["1024x1024", "1536x1024", "1024x1536"]
					},
					"quality": {
						"type": "string",
						"description": "Image quality: low (uses cheaper model), medium, or high",
						"enum": ["low", "medium", "high"]
					},
					"filename": {
						"type": "string",
						"description": "Custom filename (without extension), otherwise auto-generated with timestamp"
					},
					"force": {
						"type": "boolean",
						"description": "If true, bypass budget and daily limit checks",
						"default": false
					}
				},
				"required": ["prompt"]
			}`),
		},
		{
			Name:        "generate_asset",
			Description: "Generate a design asset (logo, icon, etc.) using GPT Image at high quality. Returns the absolute file path of the saved PNG.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"prompt": {
						"type": "string",
						"description": "Description of the asset to generate (logo, icon, etc.)"
					},
					"background": {
						"type": "string",
						"description": "Background type: transparent or opaque",
						"enum": ["transparent", "opaque"],
						"default": "transparent"
					},
					"size": {
						"type": "string",
						"description": "Image size, default 1024x1024",
						"enum": ["1024x1024", "1536x1024", "1024x1536"]
					},
					"filename": {
						"type": "string",
						"description": "Filename for the asset (without extension), e.g. crusd-logo"
					},
					"force": {
						"type": "boolean",
						"description": "If true, bypass budget and daily limit checks",
						"default": false
					}
				},
				"required": ["prompt", "filename"]
			}`),
		},
		{
			Name:        "get_usage",
			Description: "Get current spending summary: monthly total, remaining budget, images generated today, and a breakdown of today's calls.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
	}
}

// DispatchTool routes a tools/call request to the right handler.
func DispatchTool(name string, rawArgs json.RawMessage, cfg *Config, usage *UsageData) ToolResult {
	switch name {
	case "design_review":
		return dispatchDesignReview(rawArgs, cfg, usage)
	case "generate_mockup":
		return dispatchGenerateMockup(rawArgs, cfg, usage)
	case "generate_asset":
		return dispatchGenerateAsset(rawArgs, cfg, usage)
	case "get_usage":
		return dispatchGetUsage(cfg, usage)
	default:
		return errorResult(fmt.Sprintf("Unknown tool: %s", name))
	}
}

func dispatchDesignReview(rawArgs json.RawMessage, cfg *Config, usage *UsageData) ToolResult {
	var args struct {
		ImagePath string `json:"image_path"`
		Context   string `json:"context"`
		Focus     string `json:"focus"`
		Force     bool   `json:"force"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return errorResult(fmt.Sprintf("Invalid arguments: %v", err))
	}
	if args.ImagePath == "" {
		return errorResult("image_path is required")
	}

	log.Printf("tool: design_review image=%s focus=%s force=%v", args.ImagePath, args.Focus, args.Force)

	result, err := DesignReview(cfg, usage, args.ImagePath, args.Context, args.Focus, args.Force)
	if err != nil {
		return errorResult(err.Error())
	}
	return textResult(result)
}

func dispatchGenerateMockup(rawArgs json.RawMessage, cfg *Config, usage *UsageData) ToolResult {
	var args struct {
		Prompt            string `json:"prompt"`
		ReferenceImagePath string `json:"reference_image_path"`
		Size              string `json:"size"`
		Quality           string `json:"quality"`
		Filename          string `json:"filename"`
		Force             bool   `json:"force"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return errorResult(fmt.Sprintf("Invalid arguments: %v", err))
	}
	if args.Prompt == "" {
		return errorResult("prompt is required")
	}

	log.Printf("tool: generate_mockup prompt=%q ref=%s force=%v", args.Prompt, args.ReferenceImagePath, args.Force)

	path, err := GenerateMockup(cfg, usage, args.Prompt, args.ReferenceImagePath, args.Size, args.Quality, args.Filename, args.Force)
	if err != nil {
		return errorResult(err.Error())
	}
	return textResult(fmt.Sprintf("Mockup saved to: %s", path))
}

func dispatchGenerateAsset(rawArgs json.RawMessage, cfg *Config, usage *UsageData) ToolResult {
	var args struct {
		Prompt     string `json:"prompt"`
		Background string `json:"background"`
		Size       string `json:"size"`
		Filename   string `json:"filename"`
		Force      bool   `json:"force"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return errorResult(fmt.Sprintf("Invalid arguments: %v", err))
	}
	if args.Prompt == "" {
		return errorResult("prompt is required")
	}
	if args.Filename == "" {
		return errorResult("filename is required")
	}

	log.Printf("tool: generate_asset prompt=%q filename=%s force=%v", args.Prompt, args.Filename, args.Force)

	path, err := GenerateAsset(cfg, usage, args.Prompt, args.Background, args.Size, args.Filename, args.Force)
	if err != nil {
		return errorResult(err.Error())
	}
	return textResult(fmt.Sprintf("Asset saved to: %s", path))
}

func dispatchGetUsage(cfg *Config, usage *UsageData) ToolResult {
	log.Printf("tool: get_usage")
	return textResult(usage.Summary(cfg))
}

func textResult(text string) ToolResult {
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}
}

func errorResult(msg string) ToolResult {
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: msg}},
		IsError: true,
	}
}

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const designReviewSystemPrompt = `You are a senior UI/UX designer specializing in iOS mobile apps. You review screenshots and give specific, actionable feedback. Always include concrete values: exact padding in points, font weights, specific hex colors, corner radii. Structure your response as:
1. What works well (brief)
2. Issues found (specific, with fixes)
3. Suggested changes (ordered by impact)
Reference iOS HIG and modern fitness app patterns (Strava, Runna, Apple Fitness+).`

func DesignReview(cfg *Config, usage *UsageData, imagePath, context, focus string, force bool) (string, error) {
	if !force {
		if err := usage.CheckLimits(cfg, false); err != nil {
			return "", err
		}
	}

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("reading image %s: %w", imagePath, err)
	}
	b64 := base64.StdEncoding.EncodeToString(imgData)
	mime := detectMIME(imagePath)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mime, b64)

	userPrompt := "Review this UI screenshot."
	if context != "" {
		userPrompt += " Context: " + context
	}
	if focus != "" {
		userPrompt += " Focus on: " + focus + "."
	}

	reqBody := map[string]interface{}{
		"model":      cfg.ReviewModel,
		"max_completion_tokens": 1500,
		"messages": []map[string]interface{}{
			{"role": "system", "content": designReviewSystemPrompt},
			{"role": "user", "content": []map[string]interface{}{
				{"type": "text", "text": userPrompt},
				{"type": "image_url", "image_url": map[string]string{"url": dataURI}},
			}},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshalling request: %w", err)
	}

	log.Printf("design_review: calling %s for %s", cfg.ReviewModel, imagePath)

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	cost := estimateVisionCost(result.Usage.PromptTokens, result.Usage.CompletionTokens)
	usage.Record("design_review", cfg.ReviewModel, cost, false)
	if err := usage.Save(); err != nil {
		log.Printf("warning: failed to save usage: %v", err)
	}

	log.Printf("design_review: done, tokens=%d+%d, cost=$%.4f",
		result.Usage.PromptTokens, result.Usage.CompletionTokens, cost)

	return result.Choices[0].Message.Content, nil
}

func detectMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

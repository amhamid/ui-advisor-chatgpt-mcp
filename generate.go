package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// GenerateMockup creates a UI mockup image via the OpenAI image API.
func GenerateMockup(cfg *Config, usage *UsageData, prompt, refImagePath, size, quality, filename string, force bool) (string, error) {
	if !force {
		if err := usage.CheckLimits(cfg, true); err != nil {
			return "", err
		}
	}

	if size == "" {
		size = cfg.DefaultImageSize
	}
	if quality == "" {
		quality = cfg.DefaultImageQuality
	}

	model := cfg.ImageModel
	if quality == "low" {
		model = cfg.ImageModelCheap
	}

	if filename == "" {
		filename = fmt.Sprintf("mockup_%s", time.Now().Format("20060102_150405"))
	}
	outPath := filepath.Join(cfg.SavePath, filename+".png")

	if err := os.MkdirAll(cfg.SavePath, 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	var b64Image string
	var err error

	if refImagePath != "" {
		b64Image, err = callImageEdit(cfg, model, prompt, refImagePath, size)
	} else {
		b64Image, err = callImageGeneration(cfg, model, prompt, size, quality, "")
	}
	if err != nil {
		return "", err
	}

	imgData, err := base64.StdEncoding.DecodeString(b64Image)
	if err != nil {
		return "", fmt.Errorf("decoding image data: %w", err)
	}

	if err := os.WriteFile(outPath, imgData, 0644); err != nil {
		return "", fmt.Errorf("saving image: %w", err)
	}

	absPath, _ := filepath.Abs(outPath)

	cost := estimateImageCost(model, quality, size)
	usage.Record("generate_mockup", model, cost, true)
	if err := usage.Save(); err != nil {
		log.Printf("warning: failed to save usage: %v", err)
	}

	log.Printf("generate_mockup: saved %s, model=%s, quality=%s, cost=$%.4f", absPath, model, quality, cost)

	return absPath, nil
}

// GenerateAsset creates a design asset (logo, icon, etc.) via the OpenAI image API.
func GenerateAsset(cfg *Config, usage *UsageData, prompt, background, size, filename string, force bool) (string, error) {
	if !force {
		if err := usage.CheckLimits(cfg, true); err != nil {
			return "", err
		}
	}

	if size == "" {
		size = "1024x1024"
	}
	if background == "" {
		background = "transparent"
	}

	model := cfg.ImageModel
	quality := cfg.AssetQuality

	outPath := filepath.Join(cfg.SavePath, filename+".png")

	if err := os.MkdirAll(cfg.SavePath, 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	b64Image, err := callImageGeneration(cfg, model, prompt, size, quality, background)
	if err != nil {
		return "", err
	}

	imgData, err := base64.StdEncoding.DecodeString(b64Image)
	if err != nil {
		return "", fmt.Errorf("decoding image data: %w", err)
	}

	if err := os.WriteFile(outPath, imgData, 0644); err != nil {
		return "", fmt.Errorf("saving image: %w", err)
	}

	absPath, _ := filepath.Abs(outPath)

	cost := estimateImageCost(model, quality, size)
	usage.Record("generate_asset", model, cost, true)
	if err := usage.Save(); err != nil {
		log.Printf("warning: failed to save usage: %v", err)
	}

	log.Printf("generate_asset: saved %s, model=%s, quality=%s, cost=$%.4f", absPath, model, quality, cost)

	return absPath, nil
}

// callImageGeneration calls POST /v1/images/generations.
func callImageGeneration(cfg *Config, model, prompt, size, quality, background string) (string, error) {
	reqBody := map[string]interface{}{
		"model":         model,
		"prompt":        prompt,
		"n":             1,
		"size":          size,
		"quality":       quality,
		"output_format": "png",
	}
	if background != "" {
		reqBody["background"] = background
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshalling request: %w", err)
	}

	log.Printf("image_generation: calling %s, size=%s, quality=%s", model, size, quality)

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/images/generations", bytes.NewReader(body))
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

	return parseImageResponse(respBody)
}

// callImageEdit calls POST /v1/images/edits with a reference image (multipart form).
func callImageEdit(cfg *Config, model, prompt, imagePath, size string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Add image file
	imgFile, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("opening reference image: %w", err)
	}
	defer imgFile.Close()

	part, err := w.CreateFormFile("image", filepath.Base(imagePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, imgFile); err != nil {
		return "", err
	}

	w.WriteField("prompt", prompt)
	w.WriteField("model", model)
	w.WriteField("n", "1")
	w.WriteField("size", size)
	w.WriteField("output_format", "png")
	w.Close()

	log.Printf("image_edit: calling %s with reference %s, size=%s", model, imagePath, size)

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/images/edits", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
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

	return parseImageResponse(respBody)
}

func parseImageResponse(body []byte) (string, error) {
	var result struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing image response: %w", err)
	}
	if len(result.Data) == 0 || result.Data[0].B64JSON == "" {
		return "", fmt.Errorf("no image data in response")
	}
	return result.Data[0].B64JSON, nil
}

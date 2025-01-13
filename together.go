package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ImageModelConfig stores configuration for each image generation model
type ImageModelConfig struct {
	Name  string
	Steps int
}

// Map of model configurations
var imageModels = map[string]ImageModelConfig{
	"black-forest-labs/FLUX.1-schnell": {
		Name:  "black-forest-labs/FLUX.1-schnell",
		Steps: 4,
	},
	"black-forest-labs/FLUX.1-dev": {
		Name:  "black-forest-labs/FLUX.1-dev",
		Steps: 28,
	},
}


type ImageGenerationRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Steps          int    `json:"steps"`
	N              int    `json:"n"`
	ResponseFormat string `json:"response_format"`
}

type ImageGenerationResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
}

func generateImage(ctx context.Context, userID int64, username string, prompt string, model string) ([]byte, error) {
	// Log the request
	logMessage(userID, username, "image_request", prompt)

	// Get model configuration
	modelConfig, ok := imageModels[model]
	if !ok {
		return nil, fmt.Errorf("undefined image model configuration for: %s", model)
	}

	// Prepare the request body
	reqBody := ImageGenerationRequest{
		Model:          modelConfig.Name,
		Prompt:         prompt,
		Width:          1024,
		Height:         768,
		Steps:          modelConfig.Steps,
		N:              1,
		ResponseFormat: "b64_json",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.together.xyz/v1/images/generations", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Authorization", "Bearer "+config.TogetherAPIKey)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s (status code: %d)", string(body), resp.StatusCode)
	}

	// Parse the response
	var result ImageGenerationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if we got any results
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no image data in response")
	}

	// Decode the base64 image
	imageData, err := base64.StdEncoding.DecodeString(result.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image data: %w", err)
	}

	// Log success
	logMessage(userID, username, "image_generated", "Image generated successfully")

	return imageData, nil
}

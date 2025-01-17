package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ModelInfo represents information about an AI model including its price
type ModelInfo struct {
	ID       string
	PriceIn  float64 // Price per 1M input tokens in USD
	PriceOut float64 // Price per 1M output tokens in USD
}

// Message represents a chat message structure
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenRouterRequest represents the request structure for OpenRouter API
type OpenRouterRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// OpenRouterErrorResponse represents the error response structure from OpenRouter API
type OpenRouterErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type OpenRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// OpenRouterModelsResponse represents the response from OpenRouter's models endpoint
type OpenRouterModelsResponse struct {
	Data []struct {
		ID     string `json:"id"`
		Pricing struct {
			Prompt   float64 `json:"prompt"`
			Completion float64 `json:"completion"`
		} `json:"pricing"`
	} `json:"data"`
}

// FetchModelPricing gets pricing information for available models from OpenRouter
func FetchModelPricing() (map[string]ModelInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.OpenRouterAPIKey)
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var modelsResp OpenRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	modelPricing := make(map[string]ModelInfo)
	for _, model := range modelsResp.Data {
		modelPricing[model.ID] = ModelInfo{
			ID:       model.ID,
			PriceIn:  model.Pricing.Prompt,
			PriceOut: model.Pricing.Completion,
		}
	}

	return modelPricing, nil
}

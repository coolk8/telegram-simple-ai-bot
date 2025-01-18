package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	ID      string `json:"id"`      // Unique message ID
	Role    string `json:"role"`    // Role (user/assistant/system)
	Content string `json:"content"` // Message content
	Model   string `json:"model"`   // Model that generated this message (for assistant messages)
}

// OpenRouterRequest represents the request structure for OpenRouter API
type OpenRouterRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
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
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Pricing     struct {
			Prompt     string `json:"prompt"`
			Completion string `json:"completion"`
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

	req.Header.Set("Accept", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp OpenRouterModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w, body: %s", err, string(body))
	}

	// Get list of our configured models from environment
	configuredModels := make(map[string]bool)
	if models := os.Getenv("AVAILABLE_MODELS"); models != "" {
		for _, model := range strings.Split(models, ",") {
			if trimmed := strings.TrimSpace(model); trimmed != "" {
				configuredModels[trimmed] = true
			}
		}
	} else {
		// Use default model if none configured
		configuredModels["google/gemini-flash-1.5"] = true
	}

	modelPricing := make(map[string]ModelInfo)
	for _, model := range modelsResp.Data {
		// Only process models that are in our configuration
		if !configuredModels[model.ID] {
			continue
		}

		// Parse pricing from scientific notation strings to float64 and convert to price per million tokens
		promptPrice, err := strconv.ParseFloat(model.Pricing.Prompt, 64)
		if err != nil {
			log.Printf("[Warning] Failed to parse prompt price for model %s: %v", model.ID, err)
			continue
		}
		// Convert to price per million tokens
		promptPrice = promptPrice * 1_000_000

		completionPrice, err := strconv.ParseFloat(model.Pricing.Completion, 64)
		if err != nil {
			log.Printf("[Warning] Failed to parse completion price for model %s: %v", model.ID, err)
			continue
		}
		// Convert to price per million tokens
		completionPrice = completionPrice * 1_000_000

		modelPricing[model.ID] = ModelInfo{
			ID:       model.ID,
			PriceIn:  promptPrice,     // Price per 1M input tokens
			PriceOut: completionPrice, // Price per 1M output tokens
		}
	}

	return modelPricing, nil
}

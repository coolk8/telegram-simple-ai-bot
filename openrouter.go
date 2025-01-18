package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func callOpenRouter(ctx context.Context, userID int64, username string, messages []Message, model string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// If no model is specified, use the default from config
	if model == "" {
		model = config.OpenRouterModel
	}

	reqBody := OpenRouterRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: 4000, // Limit response to 4000 tokens
	}
	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use the logging functions from logger.go
	logMessage(userID, username, "openrouter_request", fmt.Sprintf("Model: %s, Messages: %d", reqBody.Model, len(reqBody.Messages)))

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(reqData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.OpenRouterAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call OpenRouter API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Use the logging functions from logger.go
	logMessage(userID, username, "openrouter_response", fmt.Sprintf("Status: %d, Response length: %d", resp.StatusCode, len(respBody)))

	if resp.StatusCode != http.StatusOK {
		// Try to parse error response
		var errorResp OpenRouterErrorResponse
		if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.Error.Message != "" {
			logMessage(userID, username, "openrouter_error", fmt.Sprintf("Status: %d, Error type: %s, message: %s", 
				resp.StatusCode, errorResp.Error.Type, errorResp.Error.Message))
			return "", fmt.Errorf("OpenRouter API error (status %d): %s - %s", 
				resp.StatusCode, errorResp.Error.Type, errorResp.Error.Message)
		}
		// If error parsing fails, log raw response
		logMessage(userID, username, "openrouter_error", fmt.Sprintf("Status: %d, Raw response: %s", 
			resp.StatusCode, string(respBody)))
		return "", fmt.Errorf("OpenRouter API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var openRouterResp OpenRouterResponse
	if err := json.Unmarshal(respBody, &openRouterResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(openRouterResp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenRouter API")
	}

	return openRouterResp.Choices[0].Message.Content, nil
}

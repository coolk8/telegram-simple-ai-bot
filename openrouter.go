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

func callOpenRouter(ctx context.Context, userID int64, username string, messages []Message) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	reqBody := OpenRouterRequest{
		Model:    config.OpenRouterModel,
		Messages: messages,
	}
	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	logOpenRouterRequest(userID, username, reqBody)

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

	logOpenRouterResponse(userID, username, resp.StatusCode, respBody)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenRouter API returned status: %d", resp.StatusCode)
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

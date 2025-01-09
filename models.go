package main

// Config holds environment variables
type Config struct {
	TelegramToken    string
	OpenRouterAPIKey string
	OpenRouterModel  string
	SystemPrompt     string
	RedisHost        string
	RedisPort        string
	RedisDB          string
	RedisPass        string
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

// OpenRouterResponse represents the response structure from OpenRouter API
type OpenRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

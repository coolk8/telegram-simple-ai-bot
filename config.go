package main

import (
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken       string
	OpenRouterAPIKey    string
	OpenRouterModel     string
	SystemPrompt        string
	RedisHost           string
	RedisPort           string
	RedisDB             string
	RedisPass           string
	AvailableModels     []ModelInfo // Changed from []string to []ModelInfo
	AllowedUsers        []int64
	TogetherAPIKey      string
	TogetherModel       string
	AvailableImgModels  []string
}

var config Config

func initConfig() {
	// Load environment variables from .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("[System] No .env file found, using system environment variables")
	}

	// Parse and fetch pricing for available models
	var availableModels []ModelInfo
	defaultModel := "google/gemini-flash-1.5"
	
	modelsList := []string{defaultModel}
	if models := os.Getenv("AVAILABLE_MODELS"); models != "" {
		modelsList = strings.Split(models, ",")
	}

	// Clean up model IDs
	cleanModelsList := make([]string, 0, len(modelsList))
	for _, model := range modelsList {
		if trimmed := strings.TrimSpace(model); trimmed != "" {
			cleanModelsList = append(cleanModelsList, trimmed)
		}
	}

	// Fetch pricing information from OpenRouter
	modelPricing, err := FetchModelPricing()
	if err != nil {
		log.Printf("[Warning] Failed to fetch model pricing: %v", err)
		// Create default models without pricing info
		for _, modelID := range cleanModelsList {
			availableModels = append(availableModels, ModelInfo{
				ID: modelID,
			})
		}
	} else {
		// Create models with pricing info and sort by average price
		for _, modelID := range cleanModelsList {
			if info, ok := modelPricing[modelID]; ok {
				availableModels = append(availableModels, info)
			} else {
				// Include model without pricing if not found in OpenRouter response
				availableModels = append(availableModels, ModelInfo{
					ID: modelID,
				})
			}
		}

		// Sort models by average price (input + output / 2)
		sort.Slice(availableModels, func(i, j int) bool {
			avgPriceI := (availableModels[i].PriceIn + availableModels[i].PriceOut) / 2
			avgPriceJ := (availableModels[j].PriceIn + availableModels[j].PriceOut) / 2
			return avgPriceI < avgPriceJ
		})
	}

	// Parse allowed users from environment variable
	var allowedUsers []int64
	if users := os.Getenv("ALLOWED_USERS"); users != "" {
		userStrings := strings.Split(users, ",")
		for _, userStr := range userStrings {
			if userID, err := strconv.ParseInt(strings.TrimSpace(userStr), 10, 64); err == nil {
				allowedUsers = append(allowedUsers, userID)
			}
		}
	}

	// Parse available image models from environment variable
	imgModels := []string{"black-forest-labs/FLUX.1-schnell"} // default model
	if models := os.Getenv("AVAILABLE_IMG_MODELS"); models != "" {
		// Split by comma and filter out empty strings
		modelsList := strings.Split(models, ",")
		imgModels = make([]string, 0, len(modelsList))
		for _, model := range modelsList {
			if trimmed := strings.TrimSpace(model); trimmed != "" {
				imgModels = append(imgModels, trimmed)
			}
		}
	}

	config = Config{
		TelegramToken:       os.Getenv("TELEGRAM_BOT_TOKEN"),
		OpenRouterAPIKey:    os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:     os.Getenv("OPENROUTER_MODEL"),
		SystemPrompt:        os.Getenv("SYSTEM_PROMPT"),
		RedisHost:           os.Getenv("REDIS_HOST"),
		RedisPort:           os.Getenv("REDIS_PORT"),
		RedisDB:            os.Getenv("REDIS_DB"),
		RedisPass:          os.Getenv("REDIS_PASS"),
		AvailableModels:    availableModels,
		AllowedUsers:       allowedUsers,
		TogetherAPIKey:     os.Getenv("TOGETHER_API_KEY"),
		TogetherModel:      os.Getenv("TOGETHER_MODEL"),
		AvailableImgModels: imgModels,
	}

	// Validate required environment variables
	if config.TelegramToken == "" || config.OpenRouterAPIKey == "" || config.RedisPass == "" {
		log.Fatal("[Error] Missing required environment variables")
	}

	// Validate image models configuration if image generation is enabled
	if config.TogetherAPIKey != "" {
		for _, model := range config.AvailableImgModels {
			if _, ok := imageModels[model]; !ok {
				log.Fatalf("[Error] Missing configuration for image model: %s", model)
			}
		}
	}
}

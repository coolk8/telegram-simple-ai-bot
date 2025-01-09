package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var config Config

func initConfig() {
	// Load environment variables from .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("[System] No .env file found, using system environment variables")
	}

	config = Config{
		TelegramToken:    os.Getenv("TELEGRAM_BOT_TOKEN"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:  os.Getenv("OPENROUTER_MODEL"),
		SystemPrompt:     os.Getenv("SYSTEM_PROMPT"),
		RedisHost:        os.Getenv("REDIS_HOST"),
		RedisPort:        os.Getenv("REDIS_PORT"),
		RedisDB:          os.Getenv("REDIS_DB"),
		RedisPass:        os.Getenv("REDIS_PASS"),
	}

	// Validate required environment variables
	if config.TelegramToken == "" || config.OpenRouterAPIKey == "" || config.RedisPass == "" {
		log.Fatal("[Error] Missing required environment variables")
	}
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

// Configuration struct to hold environment variables
type Config struct {
	TelegramToken    string
	OpenRouterAPIKey string
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

var (
	config Config
	rdb    *redis.Client
)

func init() {
	// Load environment variables from .env file if it exists
	godotenv.Load() // Ignore error - will fall back to system env vars if file not found

	config = Config{
		TelegramToken:    os.Getenv("TELEGRAM_BOT_TOKEN"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		RedisHost:        os.Getenv("REDIS_HOST"),
		RedisPort:        os.Getenv("REDIS_PORT"),
		RedisDB:          os.Getenv("REDIS_DB"),
		RedisPass:        os.Getenv("REDIS_PASS"),
	}

	// Validate required environment variables
	if config.TelegramToken == "" || config.OpenRouterAPIKey == "" || config.RedisPass == "" {
		log.Fatal("Missing required environment variables")
	}

	// Initialize Redis client
	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort),
		Password: config.RedisPass,
		DB:       0,
	})
}

func logMessage(userID int64, username, messageType, content string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] User %d (@%s) | %s: %s\n", timestamp, userID, username, messageType, content)

	f, err := os.OpenFile("message_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Error opening log file: %v", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(logEntry); err != nil {
		log.Printf("Error writing to log file: %v", err)
	}
}

func getConversationHistory(ctx context.Context, userID int64) ([]Message, error) {
	key := fmt.Sprintf("conversation:%d", userID)
	data, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return []Message{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get error: %w", err)
	}

	var messages []Message
	if err := json.Unmarshal([]byte(data), &messages); err != nil {
		return nil, fmt.Errorf("json unmarshal error: %w", err)
	}
	return messages, nil
}

func saveConversationHistory(ctx context.Context, userID int64, messages []Message) error {
	key := fmt.Sprintf("conversation:%d", userID)
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}
	return rdb.Set(ctx, key, string(data), 0).Err()
}

func clearConversationHistory(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("conversation:%d", userID)
	return rdb.Del(ctx, key).Err()
}

func getRestartKeyboard() *gotgbot.ReplyKeyboardMarkup {
	return &gotgbot.ReplyKeyboardMarkup{
		Keyboard: [][]gotgbot.KeyboardButton{
			{
				{Text: "🔄 Restart Conversation"},
			},
		},
		ResizeKeyboard: true,
	}
}

func handleMessage(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userID := msg.From.Id
	username := msg.From.Username
	if username == "" {
		username = "unknown"
	}

	// Handle restart conversation button
	if msg.Text == "🔄 Restart Conversation" {
		if err := clearConversationHistory(context.Background(), userID); err != nil {
			log.Printf("Error clearing conversation: %v", err)
		}
		logMessage(userID, username, "system", "Conversation reset")
		_, err := msg.Reply(b, "Conversation has been reset. Send a new message to start.", &gotgbot.SendMessageOpts{
			ReplyMarkup: getRestartKeyboard(),
		})
		return err
	}

	// Log user message
	logMessage(userID, username, "user_message", msg.Text)

	// Get conversation history
	history, err := getConversationHistory(context.Background(), userID)
	if err != nil {
		log.Printf("Error getting conversation history: %v", err)
		history = []Message{}
	}

	// Add user message to history
	history = append(history, Message{Role: "user", Content: msg.Text})

	// Prepare OpenRouter API request
	client := &http.Client{Timeout: 30 * time.Second}
	reqBody := OpenRouterRequest{
		Model:    "mistralai/mistral-7b-instruct",
		Messages: history,
	}
	reqData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Error marshaling request: %v", err)
		_, err := msg.Reply(b, "Sorry, I encountered an error processing your request.", &gotgbot.SendMessageOpts{
			ReplyMarkup: getRestartKeyboard(),
		})
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(reqData))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		_, err := msg.Reply(b, "Sorry, I encountered an error processing your request.", &gotgbot.SendMessageOpts{
			ReplyMarkup: getRestartKeyboard(),
		})
		return err
	}

	req.Header.Set("Authorization", "Bearer "+config.OpenRouterAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error calling OpenRouter API: %v", err)
		_, err := msg.Reply(b, "Sorry, I encountered an error processing your request.", &gotgbot.SendMessageOpts{
			ReplyMarkup: getRestartKeyboard(),
		})
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("OpenRouter API returned status: %d", resp.StatusCode)
		_, err := msg.Reply(b, "Sorry, I encountered an error processing your request.", &gotgbot.SendMessageOpts{
			ReplyMarkup: getRestartKeyboard(),
		})
		return err
	}

	var openRouterResp OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&openRouterResp); err != nil {
		log.Printf("Error decoding response: %v", err)
		_, err := msg.Reply(b, "Sorry, I encountered an error processing your request.", &gotgbot.SendMessageOpts{
			ReplyMarkup: getRestartKeyboard(),
		})
		return err
	}

	var responseText string
	if len(openRouterResp.Choices) > 0 {
		aiResponse := openRouterResp.Choices[0].Message.Content
		responseText = aiResponse

		// Add AI response to history
		history = append(history, Message{Role: "assistant", Content: aiResponse})

		// Save updated conversation history
		if err := saveConversationHistory(context.Background(), userID, history); err != nil {
			log.Printf("Error saving conversation history: %v", err)
		}

		// Log AI response
		logMessage(userID, username, "ai_response", aiResponse)
	} else {
		responseText = "Sorry, I couldn't generate a response."
		logMessage(userID, username, "error", "No response from OpenRouter API")
	}

	_, err = msg.Reply(b, responseText, &gotgbot.SendMessageOpts{
		ReplyMarkup: getRestartKeyboard(),
	})
	return err
}

func handleStart(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	logMessage(msg.From.Id, msg.From.Username, "command", "/start")
	_, err := msg.Reply(b, "Hi! I am your AI assistant. Send me a message and I will respond using AI.", &gotgbot.SendMessageOpts{
		ReplyMarkup: getRestartKeyboard(),
	})
	return err
}

func handleHelp(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	logMessage(msg.From.Id, msg.From.Username, "command", "/help")
	_, err := msg.Reply(b, "Send me any message and I will respond using AI. Use \"🔄 Restart Conversation\" to start a new conversation.", &gotgbot.SendMessageOpts{
		ReplyMarkup: getRestartKeyboard(),
	})
	return err
}

func main() {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test Redis connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Error connecting to Redis:", err)
	}

	// Create bot instance
	b, err := gotgbot.NewBot(config.TelegramToken, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create dispatcher
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occurred while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
	})

	// Add handlers
	dispatcher.AddHandler(handlers.NewCommand("start", handleStart))
	dispatcher.AddHandler(handlers.NewCommand("help", handleHelp))
	dispatcher.AddHandler(handlers.NewMessage(nil, handleMessage))

	// Create updater
	updater := ext.NewUpdater(dispatcher, &ext.UpdaterOpts{
		ErrorLog: nil,
	})

	// Start receiving updates
	err = updater.StartPolling(b, &ext.PollingOpts{
		DropPendingUpdates: true,
	})
	if err != nil {
		log.Fatal("failed to start polling:", err)
	}
	log.Printf("Bot started as @%s", b.User.Username)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Wait for interrupt signal
	<-sigChan
	log.Println("Shutting down...")
	cancel()
}

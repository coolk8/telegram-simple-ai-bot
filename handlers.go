package main

import (
	"context"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

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
			logMessage(userID, username, "error", "Failed to clear conversation")
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
		logMessage(userID, username, "error", "Failed to get conversation history")
		history = []Message{}
	}

	// If history is empty, add system prompt if configured
	if len(history) == 0 && config.SystemPrompt != "" {
		history = append(history, Message{Role: "system", Content: config.SystemPrompt})
	}

	// Add user message to history
	history = append(history, Message{Role: "user", Content: msg.Text})

	// Call OpenRouter API
	aiResponse, err := callOpenRouter(context.Background(), userID, username, history)
	if err != nil {
		logMessage(userID, username, "error", err.Error())
		_, err := msg.Reply(b, "Sorry, I encountered an error processing your request.", &gotgbot.SendMessageOpts{
			ReplyMarkup: getRestartKeyboard(),
		})
		return err
	}

	// Add AI response to history
	history = append(history, Message{Role: "assistant", Content: aiResponse})

	// Save updated conversation history
	if err := saveConversationHistory(context.Background(), userID, history); err != nil {
		logMessage(userID, username, "error", "Failed to save conversation history")
	}

	// Log AI response
	logMessage(userID, username, "ai_response", aiResponse)

	_, err = msg.Reply(b, aiResponse, &gotgbot.SendMessageOpts{
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

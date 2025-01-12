package main

import (
	"context"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func isUserAllowed(userID int64) bool {
	// If no allowed users are configured, allow everyone
	if len(config.AllowedUsers) == 0 {
		return true
	}

	// Check if user is in allowed list
	for _, allowedID := range config.AllowedUsers {
		if allowedID == userID {
			return true
		}
	}
	return false
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

	// Check if user is allowed
	if !isUserAllowed(userID) {
		logMessage(userID, username, "access_denied", "User not in allowed list")
		_, err := msg.Reply(b, "Sorry, you are not authorized to use this bot.", nil)
		return err
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

	// Get user's preferred model
	userModel, err := getUserModel(context.Background(), userID)
	if err != nil {
		logMessage(userID, username, "error", "Failed to get user model")
		userModel = config.OpenRouterModel // fallback to default
	}

	// Call OpenRouter API with user's model
	aiResponse, err := callOpenRouter(context.Background(), userID, username, history, userModel)
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
	userID := msg.From.Id
	username := msg.From.Username

	// Check if user is allowed
	if !isUserAllowed(userID) {
		logMessage(userID, username, "access_denied", "User not in allowed list")
		_, err := msg.Reply(b, "Sorry, you are not authorized to use this bot.", nil)
		return err
	}

	logMessage(userID, username, "command", "/start")
	_, err := msg.Reply(b, "Hi! I am your AI assistant. Send me a message and I will respond using AI.", &gotgbot.SendMessageOpts{
		ReplyMarkup: getRestartKeyboard(),
	})
	return err
}

func handleHelp(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userID := msg.From.Id
	username := msg.From.Username

	// Check if user is allowed
	if !isUserAllowed(userID) {
		logMessage(userID, username, "access_denied", "User not in allowed list")
		_, err := msg.Reply(b, "Sorry, you are not authorized to use this bot.", nil)
		return err
	}

	logMessage(userID, username, "command", "/help")
	_, err := msg.Reply(b, "Available commands:\n"+
		"/start - Start the bot\n"+
		"/help - Show this help message\n"+
		"/set_models - Select AI model\n\n"+
		"Use \"🔄 Restart Conversation\" to start a new conversation.", &gotgbot.SendMessageOpts{
		ReplyMarkup: getRestartKeyboard(),
	})
	return err
}

func handleSetModels(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userID := msg.From.Id
	username := msg.From.Username

	// Check if user is allowed
	if !isUserAllowed(userID) {
		logMessage(userID, username, "access_denied", "User not in allowed list")
		_, err := msg.Reply(b, "Sorry, you are not authorized to use this bot.", nil)
		return err
	}

	logMessage(userID, username, "command", "/set_models")

	// Get user's current model
	currentModel, err := getUserModel(context.Background(), userID)
	if err != nil {
		logMessage(userID, username, "error", "Failed to get current model")
		currentModel = ""
	}

	// Create inline keyboard with model options
	var buttons [][]gotgbot.InlineKeyboardButton
	for _, model := range config.AvailableModels {
		// Add checkmark for current model
		modelText := model
		if model == currentModel {
			modelText = "✅ " + model
		}
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{Text: modelText, CallbackData: "model:" + model},
		})
	}

	_, err = msg.Reply(b, "Choose a model:", &gotgbot.SendMessageOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
	return err
}

func handleCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	callback := ctx.CallbackQuery
	userID := callback.From.Id
	username := callback.From.Username

	// Check if user is allowed
	if !isUserAllowed(userID) {
		logMessage(userID, username, "access_denied", "User not in allowed list")
		_, err := callback.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "Sorry, you are not authorized to use this bot.",
			ShowAlert: true,
		})
		return err
	}

	data := callback.Data
	if len(data) > 6 && data[:6] == "model:" {
		selectedModel := data[6:]

		// Save user's model preference
		if err := setUserModel(context.Background(), userID, selectedModel); err != nil {
			logMessage(userID, username, "error", "Failed to save model preference")
			_, err := callback.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
				Text:      "Error saving model preference",
				ShowAlert: true,
			})
			return err
		}

		// Get the original message
		msg := callback.Message
		if msg == nil {
			return fmt.Errorf("callback message is nil")
		}

		// Update the message to show selected model
		_, _, err := b.EditMessageText("Selected model: "+selectedModel, &gotgbot.EditMessageTextOpts{
			ChatId:      msg.GetChat().Id,
			MessageId:   msg.GetMessageId(),
			ParseMode:   "HTML",
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{},
		})
		if err != nil {
			return err
		}

		// Show confirmation to user
		_, err = callback.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "Model set to: " + selectedModel,
			ShowAlert: true,
		})
		return err
	}

	return nil
}

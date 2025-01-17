package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func isImageGenerationEnabled() bool {
	return config.TogetherAPIKey != ""
}

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

func getKeyboard(mode string) *gotgbot.ReplyKeyboardMarkup {
	var buttons []gotgbot.KeyboardButton
	buttons = append(buttons, gotgbot.KeyboardButton{Text: "🔄 Restart Conversation"})
	
	if isImageGenerationEnabled() {
		modeButton := "🖼 Image Mode"
		if mode == "image" {
			modeButton = "📝 Text Mode"
		}
		buttons = append(buttons, gotgbot.KeyboardButton{Text: modeButton})
	}
	
	return &gotgbot.ReplyKeyboardMarkup{
		Keyboard: [][]gotgbot.KeyboardButton{buttons},
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

	// Get user's current mode
	userMode, err := getUserMode(context.Background(), userID)
	if err != nil {
		logMessage(userID, username, "error", "Failed to get user mode")
		userMode = "text" // fallback to text mode
	}

	// Handle mode switching
	if msg.Text == "🖼 Image Mode" && isImageGenerationEnabled() {
		if err := setUserMode(context.Background(), userID, "image"); err != nil {
			logMessage(userID, username, "error", "Failed to set user mode")
		}
		logMessage(userID, username, "system", "Switched to image mode")
		_, err := msg.Reply(b, "Switched to image generation mode. Send a text prompt to generate an image.", &gotgbot.SendMessageOpts{
			ReplyMarkup: getKeyboard("image"),
		})
		return err
	} else if msg.Text == "📝 Text Mode" {
		if err := setUserMode(context.Background(), userID, "text"); err != nil {
			logMessage(userID, username, "error", "Failed to set user mode")
		}
		logMessage(userID, username, "system", "Switched to text mode")
		_, err := msg.Reply(b, "Switched to text mode. Send a message to chat with AI.", &gotgbot.SendMessageOpts{
			ReplyMarkup: getKeyboard("text"),
		})
		return err
	}

	// Handle restart conversation button
	if msg.Text == "🔄 Restart Conversation" {
		if err := clearConversationHistory(context.Background(), userID); err != nil {
			logMessage(userID, username, "error", "Failed to clear conversation")
		}
		logMessage(userID, username, "system", "Conversation reset")
		_, err := msg.Reply(b, "Conversation has been reset. Send a new message to start.", &gotgbot.SendMessageOpts{
			ReplyMarkup: getKeyboard(userMode),
		})
		return err
	}

	// Log user message
	logMessage(userID, username, "user_message", msg.Text)

	if userMode == "image" && isImageGenerationEnabled() {
		prompt := msg.Text
		// If user's language is not English, translate the prompt
		if msg.From.LanguageCode != "" && msg.From.LanguageCode != "en" {
			// Create a translation prompt
			translationPrompt := fmt.Sprintf("Translate the following text from %s to English, respond with only the translation without any additional text: %s", msg.From.LanguageCode, msg.Text)
			
			// Call OpenRouter for translation
			history := []Message{
				{Role: "user", Content: translationPrompt},
			}
			translatedPrompt, err := callOpenRouter(context.Background(), userID, username, history, config.OpenRouterModel)
			if err != nil {
				logMessage(userID, username, "error", fmt.Sprintf("Translation failed: %v", err))
				_, err = msg.Reply(b, "Sorry, I encountered an error translating your prompt.", &gotgbot.SendMessageOpts{
					ReplyMarkup: getKeyboard(userMode),
				})
				return err
			}
			prompt = translatedPrompt
			
			// Inform user about translation
			_, err = msg.Reply(b, fmt.Sprintf("Translated prompt: %s", prompt), &gotgbot.SendMessageOpts{
				ReplyMarkup: getKeyboard(userMode),
			})
			if err != nil {
				return err
			}
		}

		// Get user's preferred image model
		userImageModel, err := getUserImageModel(context.Background(), userID)
		if err != nil {
			logMessage(userID, username, "error", "Failed to get user image model")
			userImageModel = config.TogetherModel // fallback to default
		}

		// Generate image with translated prompt
		imageData, err := generateImage(context.Background(), userID, username, prompt, userImageModel)
		if err != nil {
			errMsg := "Sorry, I encountered an error generating the image."
			if strings.Contains(err.Error(), "undefined image model configuration") {
				errMsg = "Sorry, this image model is not properly configured. Please try a different model or contact the administrator."
			}
			logMessage(userID, username, "error", fmt.Sprintf("Image generation failed: %v", err))
			_, err = msg.Reply(b, errMsg, &gotgbot.SendMessageOpts{
				ReplyMarkup: getKeyboard(userMode),
			})
			return err
		}

		// Send the generated image
		_, err = msg.Reply(b, "Here's your generated image:", &gotgbot.SendMessageOpts{
			ReplyMarkup: getKeyboard(userMode),
		})
		if err != nil {
			return err
		}

		// Send the image and get its file ID
		resp, err := b.SendPhoto(msg.Chat.Id, imageData, &gotgbot.SendPhotoOpts{
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: msg.MessageId,
			},
		})
		if err != nil {
			return err
		}

		// Save the image file ID with both original and translated prompts if they differ
		promptInfo := msg.Text
		if prompt != msg.Text {
			promptInfo = fmt.Sprintf("%s\nTranslated to: %s", msg.Text, prompt)
		}
		if err := saveUserImage(context.Background(), userID, resp.Photo[0].FileId, promptInfo); err != nil {
			logMessage(userID, username, "error", fmt.Sprintf("Failed to save image: %v", err))
		}
		return nil
	}

	// Text mode - handle normal conversation
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
			ReplyMarkup: getKeyboard(userMode),
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
		ReplyMarkup: getKeyboard(userMode),
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
	userMode, err := getUserMode(context.Background(), userID)
	if err != nil {
		logMessage(userID, username, "error", "Failed to get user mode")
		userMode = "text" // fallback to text mode
	}
	_, err = msg.Reply(b, "Hi! I am your AI assistant. Send me a message and I will respond using AI.", &gotgbot.SendMessageOpts{
		ReplyMarkup: getKeyboard(userMode),
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
	userMode, err := getUserMode(context.Background(), userID)
	if err != nil {
		logMessage(userID, username, "error", "Failed to get user mode")
		userMode = "text" // fallback to text mode
	}
	helpText := "Available commands:\n" +
		"/start - Start the bot\n" +
		"/help - Show this help message\n" +
		"/set_models - Select AI model for text chat\n"

	if isImageGenerationEnabled() {
		helpText += "/set_image_models - Select AI model for image generation\n" +
			"/my_images - Show your generated images\n"
	}

	helpText += "\nUse \"🔄 Restart Conversation\" to start a new conversation."
	if isImageGenerationEnabled() {
		helpText += "\nUse mode buttons to switch between text and image generation."
	}

	_, err = msg.Reply(b, helpText, &gotgbot.SendMessageOpts{
		ReplyMarkup: getKeyboard(userMode),
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

	// Create inline keyboard with model options including pricing
	var buttons [][]gotgbot.InlineKeyboardButton
	for _, modelInfo := range config.AvailableModels {
		// Add checkmark and pricing for current model
		modelText := modelInfo.ID
		if modelInfo.PriceIn > 0 || modelInfo.PriceOut > 0 {
			modelText = fmt.Sprintf("%s (In: $%.2f, Out: $%.2f per 1M tokens)", 
				modelInfo.ID, modelInfo.PriceIn, modelInfo.PriceOut)
		}
		if modelInfo.ID == currentModel {
			modelText = "✅ " + modelText
		}
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{Text: modelText, CallbackData: "model:" + modelInfo.ID},
		})
	}

	_, err = msg.Reply(b, "Choose a model for text chat:", &gotgbot.SendMessageOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
	return err
}

func handleSetImageModels(b *gotgbot.Bot, ctx *ext.Context) error {
	// Skip if image generation is not enabled
	if !isImageGenerationEnabled() {
		return nil
	}
	msg := ctx.EffectiveMessage
	userID := msg.From.Id
	username := msg.From.Username

	// Check if user is allowed
	if !isUserAllowed(userID) {
		logMessage(userID, username, "access_denied", "User not in allowed list")
		_, err := msg.Reply(b, "Sorry, you are not authorized to use this bot.", nil)
		return err
	}

	logMessage(userID, username, "command", "/set_image_models")

	// Get user's current image model
	currentModel, err := getUserImageModel(context.Background(), userID)
	if err != nil {
		logMessage(userID, username, "error", "Failed to get current image model")
		currentModel = ""
	}

	// Create inline keyboard with model options
	var buttons [][]gotgbot.InlineKeyboardButton
	for _, model := range config.AvailableImgModels {
		// Add checkmark for current model
		modelText := model
		if model == currentModel {
			modelText = "✅ " + model
		}
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{Text: modelText, CallbackData: "img_model:" + model},
		})
	}

	_, err = msg.Reply(b, "Choose an image generation model:", &gotgbot.SendMessageOpts{
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
	if len(data) > 10 && data[:10] == "img_model:" {
		selectedModel := data[10:]

		// Save user's image model preference
		if err := setUserImageModel(context.Background(), userID, selectedModel); err != nil {
			logMessage(userID, username, "error", "Failed to save image model preference")
			_, err := callback.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
				Text:      "Error saving image model preference",
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
		_, _, err := b.EditMessageText("Selected image model: "+selectedModel, &gotgbot.EditMessageTextOpts{
			ChatId:      msg.GetChat().Id,
			MessageId:   msg.GetMessageId(),
			ParseMode:   "HTML",
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{},
		})
		if err != nil {
			return err
		}

		// Acknowledge the callback without showing alert
		_, err = callback.Answer(b, nil)
		return err
	} else if len(data) > 6 && data[:6] == "model:" {
		selectedModel := data[6:]

		// Find selected model info
		var selectedModelInfo ModelInfo
		for _, info := range config.AvailableModels {
			if info.ID == selectedModel {
				selectedModelInfo = info
				break
			}
		}

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

		// Update the message to show selected model with pricing
		modelText := selectedModel
		if selectedModelInfo.PriceIn > 0 || selectedModelInfo.PriceOut > 0 {
			modelText = fmt.Sprintf("%s (In: $%.2f, Out: $%.2f per 1M tokens)", 
				selectedModel, selectedModelInfo.PriceIn, selectedModelInfo.PriceOut)
		}
		_, _, err := b.EditMessageText("Selected model: "+modelText, &gotgbot.EditMessageTextOpts{
			ChatId:      msg.GetChat().Id,
			MessageId:   msg.GetMessageId(),
			ParseMode:   "HTML",
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{},
		})
		if err != nil {
			return err
		}

		// Acknowledge the callback without showing alert
		_, err = callback.Answer(b, nil)
		return err
	}

	return nil
}

func handleMyImages(b *gotgbot.Bot, ctx *ext.Context) error {
	// Skip if image generation is not enabled
	if !isImageGenerationEnabled() {
		return nil
	}
	msg := ctx.EffectiveMessage
	userID := msg.From.Id
	username := msg.From.Username

	// Check if user is allowed
	if !isUserAllowed(userID) {
		logMessage(userID, username, "access_denied", "User not in allowed list")
		_, err := msg.Reply(b, "Sorry, you are not authorized to use this bot.", nil)
		return err
	}

	logMessage(userID, username, "command", "/my_images")

	// Get user's current mode for keyboard
	userMode, err := getUserMode(context.Background(), userID)
	if err != nil {
		logMessage(userID, username, "error", "Failed to get user mode")
		userMode = "text" // fallback to text mode
	}

	// Get user's images
	images, err := getUserImages(context.Background(), userID)
	if err != nil {
		logMessage(userID, username, "error", fmt.Sprintf("Failed to get images: %v", err))
		_, err = msg.Reply(b, "Sorry, I encountered an error retrieving your images.", &gotgbot.SendMessageOpts{
			ReplyMarkup: getKeyboard(userMode),
		})
		return err
	}

	if len(images) == 0 {
		_, err = msg.Reply(b, "You haven't generated any images yet. Switch to image mode and send a prompt to generate one!", &gotgbot.SendMessageOpts{
			ReplyMarkup: getKeyboard(userMode),
		})
		return err
	}

	// Send initial message
	_, err = msg.Reply(b, fmt.Sprintf("You have generated %d images. Here they are:", len(images)), &gotgbot.SendMessageOpts{
		ReplyMarkup: getKeyboard(userMode),
	})
	if err != nil {
		return err
	}

	// Send each image with its prompt and date
	for _, img := range images {
		_, err = b.SendPhoto(msg.Chat.Id, img.FileID, &gotgbot.SendPhotoOpts{
			Caption: fmt.Sprintf("Prompt: %s\nDate: %s", img.Prompt, img.Date),
		})
		if err != nil {
			logMessage(userID, username, "error", fmt.Sprintf("Failed to send image: %v", err))
			continue
		}
	}

	return nil
}

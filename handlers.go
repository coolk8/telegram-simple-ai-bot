package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// splitMessage splits a message into parts that fit within maxLength while preserving markdown
func splitMessage(message string, maxLength int) []string {
	if len(message) <= maxLength {
		return []string{message}
	}

	var parts []string
	currentPart := ""
	lines := strings.Split(message, "\n")
	
	// First line contains model name in italics, handle separately
	modelLine := lines[0]
	currentPart = modelLine + "\n\n"
	
	for i := 2; i < len(lines); i++ { // Start from 2 to skip empty line after model name
		line := lines[i]
		if len(currentPart)+len(line)+1 > maxLength {
			// If adding this line would exceed maxLength, start new part
			parts = append(parts, strings.TrimSpace(currentPart))
			currentPart = modelLine + "\n\n" + line + "\n" // Include model name in new part
		} else {
			currentPart += line + "\n"
		}
	}
	
	if len(currentPart) > 0 {
		parts = append(parts, strings.TrimSpace(currentPart))
	}
	
	return parts
}

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
		// Get user's selected models
		selectedModels, err := getUserModels(context.Background(), userID)
		if err != nil {
			logMessage(userID, username, "error", "Failed to get user models")
			selectedModels = []string{config.OpenRouterModel} // fallback to default
		}

		// Clear conversation history for all models
		for _, model := range selectedModels {
			if err := clearConversationHistory(context.Background(), userID, model); err != nil {
				logMessage(userID, username, "error", fmt.Sprintf("Failed to clear conversation for model %s", model))
			}
		}

		logMessage(userID, username, "system", "Conversation reset")
		_, err = msg.Reply(b, "Conversation has been reset. Send a new message to start.", &gotgbot.SendMessageOpts{
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
	logMessage(userID, username, "debug", "Starting text mode handling")

	// Get user's selected models
	selectedModels, err := getUserModels(context.Background(), userID)
	if err != nil {
		logMessage(userID, username, "error", "Failed to get user models")
		selectedModels = []string{config.OpenRouterModel} // fallback to default
	}
	logMessage(userID, username, "debug", fmt.Sprintf("Selected models: %v", selectedModels))

	// Check if this is a reply to a model's response
	replyToMsg := msg.ReplyToMessage
	var targetModel string
	if replyToMsg != nil {
		logMessage(userID, username, "debug", fmt.Sprintf("Reply detected. Bot ID: %d, Message From ID: %d", b.Id, replyToMsg.From.Id))
		if replyToMsg.From.Id == b.Id {
			// Get model from message ID mapping
			model, err := getMessageModel(context.Background(), replyToMsg.MessageId)
			if err != nil {
				logMessage(userID, username, "error", fmt.Sprintf("Failed to get model for message %d: %v", replyToMsg.MessageId, err))
			} else {
				targetModel = model
				logMessage(userID, username, "debug", fmt.Sprintf("Found model %s for message %d", model, replyToMsg.MessageId))
			}

			// Verify the model is still in user's selected models
			isValidModel := false
			for _, model := range selectedModels {
				if model == targetModel {
					isValidModel = true
					break
				}
			}
			if !isValidModel {
				logMessage(userID, username, "debug", fmt.Sprintf("Model %s is not in selected models", targetModel))
				targetModel = ""
			}
		} else {
			logMessage(userID, username, "debug", "Reply is not to a bot message")
		}
	} else {
		logMessage(userID, username, "debug", "Not a reply message")
	}

	// If we have a valid target model (replying to a specific model's message)
	if targetModel != "" {
		// Get conversation history for this model
		history, err := getConversationHistory(context.Background(), userID, targetModel)
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

		// Call OpenRouter API with the target model
		aiResponse, err := callOpenRouter(context.Background(), userID, username, history, targetModel)
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
		if err := saveConversationHistory(context.Background(), userID, targetModel, history); err != nil {
			logMessage(userID, username, "error", "Failed to save conversation history")
		}

		// Log AI response
		logMessage(userID, username, "ai_response", fmt.Sprintf("[%s] %s", targetModel, aiResponse))

		// Format response with model name in italics
		formattedResponse := fmt.Sprintf("_%s_\n\n%s", targetModel, aiResponse)
		
		// Split message if it's too long (Telegram limit is 4096 characters)
		const maxLength = 4000 // Leave some room for formatting
		var resp *gotgbot.Message
		
		if len(formattedResponse) > maxLength {
			parts := splitMessage(formattedResponse, maxLength)
			for i, part := range parts {
				opts := &gotgbot.SendMessageOpts{
					ReplyMarkup: getKeyboard(userMode),
					ParseMode:   "Markdown",
				}
				
				// Only first part replies to original message
				if i == 0 {
					opts.ReplyParameters = &gotgbot.ReplyParameters{
						MessageId: msg.MessageId,
					}
				}
				
				partResp, err := msg.Reply(b, part, opts)
				if err != nil {
					logMessage(userID, username, "error", fmt.Sprintf("Failed to send message part %d: %v", i+1, err))
					return fmt.Errorf("failed to send message part %d: %w", i+1, err)
				}
				resp = partResp // Keep track of last response for message model mapping
			}
		} else {
			var err error
			resp, err = msg.Reply(b, formattedResponse, &gotgbot.SendMessageOpts{
				ReplyMarkup: getKeyboard(userMode),
				ParseMode:   "Markdown",
			})
			if err != nil {
				logMessage(userID, username, "error", fmt.Sprintf("Failed to send message: %v", err))
				return fmt.Errorf("failed to send message: %w", err)
			}
		}

		// Save the message ID with its associated model
		if err := saveMessageModel(context.Background(), resp.MessageId, targetModel); err != nil {
			logMessage(userID, username, "error", "Failed to save message model mapping")
		}
		return nil
	}

	// If no target model (not replying to a model's message)
	if targetModel == "" {
		logMessage(userID, username, "debug", "No target model, checking if this is first message")
		
		// Check if any model has conversation history
		hasHistory := false
		for _, model := range selectedModels {
			history, err := getConversationHistory(context.Background(), userID, model)
			if err != nil {
				logMessage(userID, username, "error", fmt.Sprintf("Failed to get history for model %s: %v", model, err))
				continue
			}
			if len(history) > 0 {
				hasHistory = true
				break
			}
		}
		
		// If there's existing conversation and multiple models are selected, ask to reply to a specific model
		if hasHistory && len(selectedModels) > 1 {
			logMessage(userID, username, "debug", "Multiple models with existing conversation found, requesting reply to specific model")
			_, err = msg.Reply(b, "Please reply to a specific model's message to continue the conversation.", &gotgbot.SendMessageOpts{
				ReplyMarkup: getKeyboard(userMode),
			})
			return err
		}

		// If only one model is selected, use that model for direct messages
		if len(selectedModels) == 1 {
			model := selectedModels[0]
			logMessage(userID, username, "debug", fmt.Sprintf("Single model selected (%s), continuing conversation", model))
			
			// Get conversation history for this model
			history, err := getConversationHistory(context.Background(), userID, model)
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

			// Call OpenRouter API with this model
			aiResponse, err := callOpenRouter(context.Background(), userID, username, history, model)
			if err != nil {
				logMessage(userID, username, "error", fmt.Sprintf("[%s] %s", model, err.Error()))
				_, err := msg.Reply(b, "Sorry, I encountered an error processing your request.", &gotgbot.SendMessageOpts{
					ReplyMarkup: getKeyboard(userMode),
				})
				return err
			}

			// Add AI response to history
			history = append(history, Message{Role: "assistant", Content: aiResponse})

			// Save updated conversation history
			if err := saveConversationHistory(context.Background(), userID, model, history); err != nil {
				logMessage(userID, username, "error", fmt.Sprintf("[%s] Failed to save conversation history", model))
			}

			// Log AI response
			logMessage(userID, username, "ai_response", fmt.Sprintf("[%s] %s", model, aiResponse))

			// Format response with model name in italics
			formattedResponse := fmt.Sprintf("_%s_\n\n%s", model, aiResponse)
			
			// Split message if it's too long
			const maxLength = 4000 // Leave some room for formatting
			var resp *gotgbot.Message
			
			if len(formattedResponse) > maxLength {
				parts := splitMessage(formattedResponse, maxLength)
				for i, part := range parts {
					opts := &gotgbot.SendMessageOpts{
						ReplyMarkup: getKeyboard(userMode),
						ParseMode:   "Markdown",
					}
					
					// Only first part replies to original message
					if i == 0 {
						opts.ReplyParameters = &gotgbot.ReplyParameters{
							MessageId: msg.MessageId,
						}
					}
					
					partResp, err := msg.Reply(b, part, opts)
					if err != nil {
						logMessage(userID, username, "error", fmt.Sprintf("[%s] Failed to send message part %d: %v", model, i+1, err))
						continue
					}
					resp = partResp // Keep track of last response for message model mapping
				}
			} else {
				var err error
				resp, err = msg.Reply(b, formattedResponse, &gotgbot.SendMessageOpts{
					ReplyMarkup: getKeyboard(userMode),
					ParseMode:   "Markdown",
				})
				if err != nil {
					logMessage(userID, username, "error", fmt.Sprintf("[%s] Failed to send response: %v", model, err))
					return err
				}
			}

			// Save the message ID with its associated model
			if err := saveMessageModel(context.Background(), resp.MessageId, model); err != nil {
				logMessage(userID, username, "error", fmt.Sprintf("[%s] Failed to save message model mapping", model))
			}
			return nil
		}

		// This is the first message, use all selected models
		logMessage(userID, username, "debug", "No existing conversation, using all models")
		for _, model := range selectedModels {
			// Get conversation history for this model
			history, err := getConversationHistory(context.Background(), userID, model)
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

			// Call OpenRouter API with this model
			aiResponse, err := callOpenRouter(context.Background(), userID, username, history, model)
			if err != nil {
				logMessage(userID, username, "error", fmt.Sprintf("[%s] %s", model, err.Error()))
				continue // Try next model instead of failing completely
			}

			// Add AI response to history
			history = append(history, Message{Role: "assistant", Content: aiResponse})

			// Save updated conversation history
			if err := saveConversationHistory(context.Background(), userID, model, history); err != nil {
				logMessage(userID, username, "error", fmt.Sprintf("[%s] Failed to save conversation history", model))
			}

			// Log AI response
			logMessage(userID, username, "ai_response", fmt.Sprintf("[%s] %s", model, aiResponse))

			// Format response with model name in italics
			formattedResponse := fmt.Sprintf("_%s_\n\n%s", model, aiResponse)
			
			// Split message if it's too long
			const maxLength = 4000 // Leave some room for formatting
			var resp *gotgbot.Message
			
			if len(formattedResponse) > maxLength {
				parts := splitMessage(formattedResponse, maxLength)
				for i, part := range parts {
					opts := &gotgbot.SendMessageOpts{
						ReplyMarkup: getKeyboard(userMode),
						ParseMode:   "Markdown",
					}
					
					// Only first part replies to original message
					if i == 0 {
						opts.ReplyParameters = &gotgbot.ReplyParameters{
							MessageId: msg.MessageId,
						}
					}
					
					partResp, err := msg.Reply(b, part, opts)
					if err != nil {
						logMessage(userID, username, "error", fmt.Sprintf("[%s] Failed to send message part %d: %v", model, i+1, err))
						continue
					}
					resp = partResp // Keep track of last response for message model mapping
				}
			} else {
				var err error
				resp, err = msg.Reply(b, formattedResponse, &gotgbot.SendMessageOpts{
					ReplyMarkup: getKeyboard(userMode),
					ParseMode:   "Markdown",
				})
				if err != nil {
					logMessage(userID, username, "error", fmt.Sprintf("[%s] Failed to send response: %v", model, err))
					continue
				}
			}

			// Save the message ID with its associated model
			if err := saveMessageModel(context.Background(), resp.MessageId, model); err != nil {
				logMessage(userID, username, "error", fmt.Sprintf("[%s] Failed to save message model mapping", model))
			}
		}
		return nil
	}
	return nil
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
		"/set_models - Select AI models for text chat (you can select multiple)\n"

	if isImageGenerationEnabled() {
		helpText += "/set_image_models - Select AI model for image generation\n" +
			"/my_images - Show your generated images\n"
	}

	helpText += "\nUse \"🔄 Restart Conversation\" to start a new conversation."
	if isImageGenerationEnabled() {
		helpText += "\nUse mode buttons to switch between text and image generation."
	}

	helpText += "\n\nIn text mode:\n" +
		"- First message: All selected models will respond\n" +
		"- To continue with a specific model: Reply to its message"

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

	// Get user's current models
	currentModels, err := getUserModels(context.Background(), userID)
	if err != nil {
		logMessage(userID, username, "error", "Failed to get current models")
		currentModels = []string{}
	}

	// Create map for O(1) lookup
	selectedModels := make(map[string]bool)
	for _, model := range currentModels {
		selectedModels[model] = true
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
		if selectedModels[modelInfo.ID] {
			modelText = "✅ " + modelText
		}
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{Text: modelText, CallbackData: "model:" + modelInfo.ID},
		})
	}

	// Add a "Done" button at the bottom
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "✨ Done", CallbackData: "models:done"},
	})

	_, err = msg.Reply(b, "Choose models for text chat (select multiple):", &gotgbot.SendMessageOpts{
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

		// Get user's current models
		currentModels, err := getUserModels(context.Background(), userID)
		if err != nil {
			logMessage(userID, username, "error", "Failed to get current models")
			_, err := callback.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
				Text:      "Error getting current models",
				ShowAlert: true,
			})
			return err
		}

		// Create map for O(1) lookup
		selectedModels := make(map[string]bool)
		for _, model := range currentModels {
			selectedModels[model] = true
		}

		// Toggle model selection
		if selectedModels[selectedModel] {
			if err := removeUserModel(context.Background(), userID, selectedModel); err != nil {
				logMessage(userID, username, "error", "Failed to remove model")
				_, err := callback.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
					Text:      "Error removing model",
					ShowAlert: true,
				})
				return err
			}
			delete(selectedModels, selectedModel)
		} else {
			if err := addUserModel(context.Background(), userID, selectedModel); err != nil {
				logMessage(userID, username, "error", "Failed to add model")
				_, err := callback.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
					Text:      "Error adding model",
					ShowAlert: true,
				})
				return err
			}
			selectedModels[selectedModel] = true
		}

		// Update the message with new selection state
		var buttons [][]gotgbot.InlineKeyboardButton
		for _, modelInfo := range config.AvailableModels {
			modelText := modelInfo.ID
			if modelInfo.PriceIn > 0 || modelInfo.PriceOut > 0 {
				modelText = fmt.Sprintf("%s (In: $%.2f, Out: $%.2f per 1M tokens)", 
					modelInfo.ID, modelInfo.PriceIn, modelInfo.PriceOut)
			}
			if selectedModels[modelInfo.ID] {
				modelText = "✅ " + modelText
			}
			buttons = append(buttons, []gotgbot.InlineKeyboardButton{
				{Text: modelText, CallbackData: "model:" + modelInfo.ID},
			})
		}

		// Add "Done" button
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{Text: "✨ Done", CallbackData: "models:done"},
		})

		// Update message with new keyboard
		_, _, err = b.EditMessageText("Choose models for text chat (select multiple):", &gotgbot.EditMessageTextOpts{
			ChatId:      callback.Message.GetChat().Id,
			MessageId:   callback.Message.GetMessageId(),
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons},
		})
		if err != nil {
			return err
		}

		// Acknowledge the callback without showing alert
		_, err = callback.Answer(b, nil)
		return err
	} else if data == "models:done" {
		// Get user's selected models
		selectedModels, err := getUserModels(context.Background(), userID)
		if err != nil {
			logMessage(userID, username, "error", "Failed to get selected models")
			_, err := callback.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
				Text:      "Error getting selected models",
				ShowAlert: true,
			})
			return err
		}

		// Update message to show final selection
		modelsList := strings.Join(selectedModels, "\n")
		_, _, err = b.EditMessageText(fmt.Sprintf("Selected models:\n%s", modelsList), &gotgbot.EditMessageTextOpts{
			ChatId:      callback.Message.GetChat().Id,
			MessageId:   callback.Message.GetMessageId(),
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

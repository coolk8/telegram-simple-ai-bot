package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

func main() {
	// Initialize configuration
	initConfig()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Redis
	initRedis()

	// Test Redis connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("[Error] Failed to connect to Redis: ", err)
	}

	// Create bot instance
	b, err := gotgbot.NewBot(config.TelegramToken, nil)
	if err != nil {
		log.Fatal("[Error] Failed to create bot instance: ", err)
	}

	// Set bot commands
	var commands []gotgbot.BotCommand
	commands = append(commands,
		gotgbot.BotCommand{Command: "start", Description: "Start the bot"},
		gotgbot.BotCommand{Command: "help", Description: "Show help message"},
		gotgbot.BotCommand{Command: "set_models", Description: "Select AI model for text chat"},
	)
	
	// Add image-related commands if enabled
	if isImageGenerationEnabled() {
		commands = append(commands,
			gotgbot.BotCommand{Command: "set_image_models", Description: "Select AI model for image generation"},
			gotgbot.BotCommand{Command: "my_images", Description: "Show your generated images"},
		)
	}
	if _, err := b.SetMyCommands(commands, nil); err != nil {
		log.Fatal("[Error] Failed to set bot commands: ", err)
	}

	// Set menu button to show commands
	if _, err := b.SetChatMenuButton(&gotgbot.SetChatMenuButtonOpts{
		MenuButton: &gotgbot.MenuButtonCommands{},
	}); err != nil {
		log.Fatal("[Error] Failed to set menu button: ", err)
	}

	// Create dispatcher
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Printf("[Error] Failed to handle update: %v", err.Error())
			return ext.DispatcherActionNoop
		},
	})

	// Add handlers
	dispatcher.AddHandler(handlers.NewCommand("start", handleStart))
	dispatcher.AddHandler(handlers.NewCommand("help", handleHelp))
	dispatcher.AddHandler(handlers.NewCommand("set_models", handleSetModels))
	
	// Add image-related handlers if enabled
	if isImageGenerationEnabled() {
		dispatcher.AddHandler(handlers.NewCommand("set_image_models", handleSetImageModels))
		dispatcher.AddHandler(handlers.NewCommand("my_images", handleMyImages))
	}
	dispatcher.AddHandler(handlers.NewCallback(nil, handleCallback))
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
		log.Fatal("[Error] Failed to start polling: ", err)
	}
	log.Printf("[System] Bot started as @%s", b.User.Username)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Wait for interrupt signal
	<-sigChan
	log.Println("[System] Shutting down...")
	cancel()
}

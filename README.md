# Telegram OpenAI Bot

A simple Telegram bot written in Go that uses OpenRouter API to generate AI responses to user messages.

## Prerequisites

1. Install Go:
   - Windows: Download and install from [Go Downloads](https://golang.org/dl/)
   - Linux: `sudo apt-get install golang`
   - macOS: `brew install go`

2. Install Redis:
   - Windows: Download and install from [Redis for Windows](https://github.com/microsoftarchive/redis/releases)
   - Linux: `sudo apt-get install redis-server`
   - macOS: `brew install redis`

## Setup

1. Clone this repository

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file in the project root with your API keys and Redis configuration:
```
# API Keys
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
OPENROUTER_API_KEY=your_openrouter_api_key_here

# Redis configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_DB=0
REDIS_PASS=your_redis_password_here  # Password for Redis authentication
```

To get the required API keys:
- Telegram Bot Token: Talk to [@BotFather](https://t.me/botfather) on Telegram
- OpenRouter API Key: Sign up at [OpenRouter](https://openrouter.ai/)

4. Configure Redis password:
   - Windows: Edit redis.windows.conf and set `requirepass your_redis_password_here`
   - Linux/macOS: Edit /etc/redis/redis.conf and set `requirepass your_redis_password_here`
   Make sure to use the same password as in your .env file

5. Start Redis server:
   - Windows: Start Redis service
   - Linux/macOS: `redis-server`

6. Build and run the bot:
```bash
# Build the bot
go build -o bot

# Run the bot
./bot  # On Unix-like systems
bot.exe  # On Windows
```

## Usage

1. Start a chat with your bot on Telegram
2. Send `/start` to begin
3. Send any message to get an AI response
4. Use the "🔄 Restart Conversation" button to start a new conversation

## Features

- Maintains conversation history for contextual responses
- Responds to all text messages using AI
- Uses Mistral-7B model through OpenRouter
- Simple error handling
- "Restart Conversation" button to clear chat history
- Secure Redis connection with password authentication

## How It Works

The bot maintains a conversation history for each user using Redis. This allows the AI to understand the context of your messages and provide more relevant responses. When you press the "Restart Conversation" button, your conversation history is cleared, and you can start a fresh conversation.

The Redis connection is secured with password authentication to ensure data safety. Make sure to use a strong password and keep it secure in your .env file.

## Project Structure

- `main.go`: Main bot implementation
- `go.mod`: Go module definition and dependencies
- `.env`: Configuration file for API keys and Redis settings

## Error Handling

The bot includes comprehensive error handling for:
- Redis connection issues
- API communication errors
- Message processing failures
- Invalid configurations

All errors are logged with timestamps and relevant context information.

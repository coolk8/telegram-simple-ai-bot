# Telegram OpenAI Bot

A simple Telegram bot that uses OpenRouter API to generate AI responses to user messages.

## Setup

1. Clone this repository

2. Install Redis:
   - For Windows: Download and install from [Redis for Windows](https://github.com/microsoftarchive/redis/releases)
   - For Linux: `sudo apt-get install redis-server`
   - For macOS: `brew install redis`

3. Install dependencies:
```bash
pip install -r requirements.txt
```

4. Create a `.env` file in the project root with your API keys and Redis configuration:
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

5. Configure Redis password:
   - Windows: Edit redis.windows.conf and set `requirepass your_redis_password_here`
   - Linux/macOS: Edit /etc/redis/redis.conf and set `requirepass your_redis_password_here`
   Make sure to use the same password as in your .env file

6. Start Redis server:
   - Windows: Start Redis service
   - Linux/macOS: `redis-server`

7. Run the bot:
```bash
python main.py
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
- Comprehensive message logging:
  - All user messages
  - AI responses
  - Command usage
  - Errors
  - Timestamps and user information
  - Logs stored in message_logs.txt

## How It Works

The bot maintains a conversation history for each user using Redis. This allows the AI to understand the context of your messages and provide more relevant responses. When you press the "Restart Conversation" button, your conversation history is cleared, and you can start a fresh conversation.

The Redis connection is secured with password authentication to ensure data safety. Make sure to use a strong password and keep it secure in your .env file.

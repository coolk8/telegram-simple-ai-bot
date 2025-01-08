# Telegram OpenAI Bot

A simple Telegram bot that uses OpenRouter API to generate AI responses to user messages.

## Setup

1. Clone this repository
2. Install dependencies:
```bash
pip install -r requirements.txt
```

3. Create a `.env` file in the project root with your API keys:
```
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
OPENROUTER_API_KEY=your_openrouter_api_key_here
```

To get the required API keys:
- Telegram Bot Token: Talk to [@BotFather](https://t.me/botfather) on Telegram
- OpenRouter API Key: Sign up at [OpenRouter](https://openrouter.ai/)

4. Run the bot:
```bash
python bot.py
```

## Usage

1. Start a chat with your bot on Telegram
2. Send `/start` to begin
3. Send any message to get an AI response

## Features

- Responds to all text messages using AI
- Uses Mistral-7B model through OpenRouter
- Simple error handling
- Basic logging

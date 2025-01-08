import os
import logging
from dotenv import load_dotenv
import requests
from telegram import Update
from telegram.ext import Application, CommandHandler, MessageHandler, ContextTypes, filters

# Load environment variables
load_dotenv()
TELEGRAM_TOKEN = os.getenv("TELEGRAM_BOT_TOKEN")
OPENROUTER_API_KEY = os.getenv("OPENROUTER_API_KEY")
OPENROUTER_MODEL = os.getenv("OPENROUTER_MODEL")

# Configure logging
logging.basicConfig(
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    level=logging.INFO
)

async def start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Send a message when the command /start is issued."""
    await update.message.reply_text('Hi! I am your AI assistant. Send me a message and I will respond using AI.')

async def help_command(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Send a message when the command /help is issued."""
    await update.message.reply_text('Send me any message and I will respond using AI.')

async def handle_message(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Handle user messages and get AI response."""
    message = update.message.text
    
    # Call OpenRouter API
    headers = {
        "Authorization": f"Bearer {OPENROUTER_API_KEY}",
        "Content-Type": "application/json"
    }
    
    data = {
        "model": "mistralai/mistral-7b-instruct",  # Using Mistral as default model
        "messages": [{"role": "user", "content": message}]
    }
    
    try:
        response = requests.post(
            "https://openrouter.ai/api/v1/chat/completions",
            headers=headers,
            json=data
        )
        response.raise_for_status()
        
        ai_response = response.json()["choices"][0]["message"]["content"]
        await update.message.reply_text(ai_response)
        
    except Exception as e:
        logging.error(f"Error calling OpenRouter API: {str(e)}")
        await update.message.reply_text("Sorry, I encountered an error processing your request.")

def main():
    """Start the bot."""
    if not TELEGRAM_TOKEN or not OPENROUTER_API_KEY:
        logging.error("Missing required environment variables!")
        return

    # Create application
    application = Application.builder().token(TELEGRAM_TOKEN).build()

    # Add handlers
    application.add_handler(CommandHandler("start", start))
    application.add_handler(CommandHandler("help", help_command))
    application.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_message))

    # Start polling
    application.run_polling(allowed_updates=Update.ALL_TYPES)

if __name__ == "__main__":
    main()

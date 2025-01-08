import os
import logging
import json
from dotenv import load_dotenv
import requests
import aioredis
from telegram import Update, ReplyKeyboardMarkup, KeyboardButton
from telegram.ext import Application, CommandHandler, MessageHandler, ContextTypes, filters
from datetime import datetime

# Load environment variables
load_dotenv()
TELEGRAM_TOKEN = os.getenv("TELEGRAM_BOT_TOKEN")
OPENROUTER_API_KEY = os.getenv("OPENROUTER_API_KEY")
REDIS_HOST = os.getenv("REDIS_HOST", "localhost")
REDIS_PORT = int(os.getenv("REDIS_PORT", 6379))
REDIS_DB = int(os.getenv("REDIS_DB", 0))
REDIS_PASS = os.getenv("REDIS_PASS")

# Configure logging
logging.basicConfig(
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    level=logging.INFO
)

# Configure message logging
MESSAGE_LOG_FILE = "message_logs.txt"
def log_message(user_id: int, username: str, message_type: str, content: str):
    """Log messages to a file with timestamp."""
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    log_entry = f"[{timestamp}] User {user_id} (@{username}) | {message_type}: {content}\n"
    
    with open(MESSAGE_LOG_FILE, "a", encoding="utf-8") as f:
        f.write(log_entry)

# Redis connection
redis = None

async def init_redis():
    """Initialize Redis connection."""
    global redis
    redis = await aioredis.from_url(
        f"redis://{REDIS_HOST}:{REDIS_PORT}/{REDIS_DB}",
        encoding="utf-8",
        decode_responses=True,
        password=REDIS_PASS
    )

async def get_conversation_history(user_id: int) -> list:
    """Get conversation history from Redis."""
    history = await redis.get(f"conversation:{user_id}")
    return json.loads(history) if history else []

async def save_conversation_history(user_id: int, history: list):
    """Save conversation history to Redis."""
    await redis.set(f"conversation:{user_id}", json.dumps(history))

async def clear_conversation_history(user_id: int):
    """Clear conversation history from Redis."""
    await redis.delete(f"conversation:{user_id}")

def get_restart_keyboard():
    """Create keyboard with Restart Conversation button."""
    keyboard = [[KeyboardButton("🔄 Restart Conversation")]]
    return ReplyKeyboardMarkup(keyboard, resize_keyboard=True)

async def start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Send a message when the command /start is issued."""
    response = 'Hi! I am your AI assistant. Send me a message and I will respond using AI.'
    await update.message.reply_text(
        response,
        reply_markup=get_restart_keyboard()
    )
    
    log_message(
        update.message.from_user.id,
        update.message.from_user.username or "unknown",
        "command",
        "/start"
    )

async def help_command(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Send a message when the command /help is issued."""
    response = 'Send me any message and I will respond using AI. Use "🔄 Restart Conversation" to start a new conversation.'
    await update.message.reply_text(
        response,
        reply_markup=get_restart_keyboard()
    )
    
    log_message(
        update.message.from_user.id,
        update.message.from_user.username or "unknown",
        "command",
        "/help"
    )

async def handle_message(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Handle user messages and get AI response."""
    message = update.message.text
    user_id = update.message.from_user.id
    username = update.message.from_user.username or "unknown"
    
    # Handle restart conversation button
    if message == "🔄 Restart Conversation":
        await clear_conversation_history(user_id)
        await update.message.reply_text(
            "Conversation has been reset. Send a new message to start.",
            reply_markup=get_restart_keyboard()
        )
        log_message(user_id, username, "system", "Conversation reset")
        return
    
    # Log user message
    log_message(user_id, username, "user_message", message)
    
    # Get conversation history
    history = await get_conversation_history(user_id)
    
    # Add user message to history
    history.append({"role": "user", "content": message})
    
    # Call OpenRouter API
    headers = {
        "Authorization": f"Bearer {OPENROUTER_API_KEY}",
        "Content-Type": "application/json"
    }
    
    data = {
        "model": "mistralai/mistral-7b-instruct",
        "messages": history  # Send full conversation history
    }
    
    try:
        response = requests.post(
            "https://openrouter.ai/api/v1/chat/completions",
            headers=headers,
            json=data
        )
        response.raise_for_status()
        
        ai_response = response.json()["choices"][0]["message"]["content"]
        
        # Add AI response to history
        history.append({"role": "assistant", "content": ai_response})
        
        # Save updated conversation history
        await save_conversation_history(user_id, history)
        
        await update.message.reply_text(
            ai_response,
            reply_markup=get_restart_keyboard()
        )
        
        # Log AI response
        log_message(user_id, username, "ai_response", ai_response)
        
    except Exception as e:
        error_message = f"Error calling OpenRouter API: {str(e)}"
        logging.error(error_message)
        await update.message.reply_text(
            "Sorry, I encountered an error processing your request.",
            reply_markup=get_restart_keyboard()
        )
        
        # Log error
        log_message(user_id, username, "error", error_message)

async def main():
    """Start the bot."""
    if not all([TELEGRAM_TOKEN, OPENROUTER_API_KEY, REDIS_PASS]):
        logging.error("Missing required environment variables!")
        return

    # Initialize Redis connection
    await init_redis()

    # Create application
    application = Application.builder().token(TELEGRAM_TOKEN).build()

    # Add handlers
    application.add_handler(CommandHandler("start", start))
    application.add_handler(CommandHandler("help", help_command))
    application.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_message))

    # Start polling
    await application.run_polling(allowed_updates=Update.ALL_TYPES)

if __name__ == "__main__":
    import asyncio
    asyncio.run(main())

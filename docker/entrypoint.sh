#!/usr/bin/env bash
# Bot entrypoint: render config.json from environment variables, then run the bot.
set -euo pipefail

: "${BOT_TOKEN:?BOT_TOKEN is required (Telegram bot token from @BotFather)}"
: "${ADMIN_ID:?ADMIN_ID is required (numeric Telegram user id)}"
: "${ARIA2_KEY:?ARIA2_KEY is required (must match ARIA2_SECRET of the aria2 service)}"

CFG=/app/config.json
python3 /app/render_config.py > "$CFG"
chmod 600 "$CFG"

exec /app/DownloadBot -c "$CFG"

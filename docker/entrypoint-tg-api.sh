#!/usr/bin/env bash
# Entrypoint for the local Telegram Bot API server (tdlib/telegram-bot-api).
set -euo pipefail

: "${TG_API_ID:?TG_API_ID is required (my.telegram.org app id)}"
: "${TG_API_HASH:?TG_API_HASH is required (my.telegram.org app hash)}"

DATA_DIR="${TG_API_DIR:-/data/.tg-bot-api}"
PORT="${TG_API_PORT:-8081}"

mkdir -p "$DATA_DIR"

export TELEGRAM_API_ID="$TG_API_ID"
export TELEGRAM_API_HASH="$TG_API_HASH"

exec telegram-bot-api --local --http-port="$PORT" --dir="$DATA_DIR"

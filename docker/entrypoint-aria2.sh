#!/usr/bin/env bash
# aria2 entrypoint. Pass the RPC secret via ARIA2_SECRET.
set -euo pipefail

: "${ARIA2_SECRET:?ARIA2_SECRET is required (must match ARIA2_SECRET of the bot)}"
DOWNLOAD_DIR="${DOWNLOAD_DIR:-/data/downloads}"

mkdir -p "$DOWNLOAD_DIR"

exec aria2c \
  --enable-rpc \
  --rpc-listen-all \
  --rpc-listen-port=6800 \
  --rpc-secret="$ARIA2_SECRET" \
  --dir="$DOWNLOAD_DIR" \
  --continue=true \
  --max-concurrent-downloads=5 \
  --max-connection-per-server=16 \
  --split=16 \
  --file-allocation=none \
  --enable-dht=true \
  --seed-time=0

#!/usr/bin/env python3
"""Render config.json for ariaflow from environment variables.

The Go binary reads config from a JSON file only (no env substitution), so we
generate it here at container start. This mirrors the logic in setup.sh.
"""
import json
import os


def b(key, default=False):
    v = os.environ.get(key, "true" if default else "false").strip().lower()
    return v in ("1", "true", "yes", "on")


def s(key, default=""):
    return os.environ.get(key, "").strip() or default


LIB_ROOT = s("LIBRARY_ROOT", "/data")
DOWNLOAD_FOLDER = s("DOWNLOAD_FOLDER", "/data/downloads")

# Local Bot API server: only enabled when both API id and hash are provided.
api_id = s("API_ID", "0")
api_hash = s("API_HASH")
if api_id and api_id != "0" and api_hash:
    api_base = s("API_BASE", "http://tg-bot-api:8081")
    api_dir = s("API_DIR", "/data/.tg-bot-api")
else:
    api_id, api_hash, api_base, api_dir = "0", "", "", ""

cfg = {
    "input": {
        "aria2": {
            "aria2-server": s("ARIA2_SERVER", "ws://aria2:6800/jsonrpc"),
            "aria2-key": s("ARIA2_KEY"),
        }
    },
    "output": {
        "telegram": {
            "bot-key": s("BOT_TOKEN"),
            "user-id": s("ADMIN_ID"),
            "api-id": int(api_id or 0),
            "api-hash": api_hash,
            "api-base": api_base,
            "api-dir": api_dir,
        }
    },
    "max-index": int(s("MAX_INDEX", "10") or 10),
    "language": s("LANGUAGE", "en"),
    "downloadFolder": DOWNLOAD_FOLDER,
    "organize": {
        "enabled": b("ORGANIZE_ENABLED", True),
        "anilist": b("ANILIST", True),
        "deleteArchive": b("DELETE_ARCHIVE"),
        "movies": s("MOVIES", LIB_ROOT + "/movies"),
        "series": s("SERIES", LIB_ROOT + "/series"),
        "anime": s("ANIME", LIB_ROOT + "/anime"),
        "music": s("MUSIC", LIB_ROOT + "/music"),
        "documents": s("DOCUMENTS", LIB_ROOT + "/documents"),
        "archives": s("ARCHIVES", LIB_ROOT + "/archives"),
        "others": s("OTHERS", LIB_ROOT + "/others"),
        "torrents": s("TORRENTS", LIB_ROOT + "/torrents"),
        "keepTorrent": b("KEEP_TORRENT", True),
        "youtube": s("YOUTUBE", LIB_ROOT + "/YouTube"),
        "services": s("SERVICES", LIB_ROOT + "/Services"),
        "ytdlpPath": s("YTDLP_PATH", "yt-dlp"),
        "ytdlpQuality": s(
            "YTDLP_QUALITY",
            "bestvideo[height<=1080]+bestaudio/best[height<=1080]/best",
        ),
        "ytdlpCookies": s("YTDLP_COOKIES"),
        "ytdlpProxy": s("YTDLP_PROXY"),
        "ytdlpEmbed": b("YTDLP_EMBED", True),
    },
    "log": {
        "logPath": s("LOG_PATH"),
        "errPath": s("ERR_PATH"),
        "level": s("LOG_LEVEL", "info"),
    },
}

print(json.dumps(cfg, indent=2))

# Docker Compose deployment for prox-download-bot

Run the whole stack — the bot, `aria2c`, and (optionally) a self-hosted
Telegram Bot API server — with Docker Compose.

```
compose/
├── docker-compose.yml          # the stack
├── .env.example                # copy → .env
└── docker/
    ├── Dockerfile.bot          # bot image (builds the Go binary)
    ├── Dockerfile.aria2        # aria2c daemon image
    ├── Dockerfile.tg-bot-api   # tdlib/telegram-bot-api image (slow build)
    ├── entrypoint.sh           # bot entrypoint: renders config.json from env
    ├── render_config.py        # env → config.json
    ├── entrypoint-aria2.sh     # aria2 entrypoint
    └── entrypoint-tg-api.sh    # API server entrypoint
```

## 1. Configure

```bash
cp .env.example .env
# edit .env: set ARIA2_SECRET, BOT_TOKEN, ADMIN_ID (required)
# optionally API_ID / API_HASH for the local API server
```

## 2. Start

```bash
# Standard (cloud Bot API, forwarded files ≤ 20 MB)
docker compose up -d --build

# With the local Bot API server (forwarded files up to 2 GB)
docker compose --profile api-server up -d --build
```

> The `tg-bot-api` image compiles tdlib — the first build takes several
> minutes even on a fast machine.

## 3. Verify

```bash
docker compose ps
docker compose logs -f bot      # should show "Aria2 websocket connection established!"
docker compose logs aria2       # should show RPC enabled
```

Then open Telegram, send `/start` to your bot.

## Configuration

The bot reads `config.json`, which the `bot` entrypoint generates at startup
from environment variables (see `docker/render_config.py`). The whole
config/reference is documented in the project's `README.md`; the env mapping is:

| Env | Config key |
|-----|------------|
| `ARIA2_SERVER` | `input.aria2.aria2-server` |
| `ARIA2_KEY` | `input.aria2.aria2-key` |
| `BOT_TOKEN` | `output.telegram.bot-key` |
| `ADMIN_ID` | `output.telegram.user-id` |
| `API_ID` / `API_HASH` | `output.telegram.api-id` / `api-hash` |
| `API_BASE` | `output.telegram.api-base` (empty = cloud, 20 MB cap) |
| `API_DIR` | `output.telegram.api-dir` |
| `MAX_INDEX` | `max-index` |
| `LANGUAGE` | `language` |
| `DOWNLOAD_FOLDER` | `downloadFolder` |
| `LIBRARY_ROOT` | base for all `organize.*` paths |
| `ORGANIZE_ENABLED` / `ANILIST` / `DELETE_ARCHIVE` / `KEEP_TORRENT` | `organize.*` |
| `YTDLP_*` | `organize.ytdlp*` |
| `LOG_LEVEL` / `LOG_PATH` / `ERR_PATH` | `log.*` |

### Storage

All services share one volume (`prox-download-bot-media`) mounted at `/data`:

- aria2 downloads into `/data/downloads`
- the bot organizes into `/data/movies`, `/data/series`, … (from `LIBRARY_ROOT`)
- (optional) the API server stores its temp files in `/data/.tg-bot-api`

Keeping these on the same volume/filesystem is required so forwarded files can
be **hardlinked** into the download folder (instant, no extra disk). If you'd
rather use a NAS, change the `media` volume in `docker-compose.yml` to a bind
mount (example commented in the file).

## Notes / troubleshooting

- **Bot can't reach aria2** → `docker compose logs bot`; make sure
  `ARIA2_SECRET` is identical everywhere and aria2 is healthy.
- **Forwarded file > 20 MB fails** → you're on the cloud API. Use the
  `api-server` profile with `API_ID`/`API_HASH`.
- **"Can't lock file … tqueue.binlog"** → two servers share the same
  `API_DIR`; give each its own.
- **`api-dir` progress stuck on "Fetching…"** → `API_DIR` must point at the
  server's data dir (default `/data/.tg-bot-api` is correct for this stack).
- **Update yt-dlp** → rebuild the bot image: `docker compose build bot`.
- **Secrets** → `.env`, `config.json`, `users.json`, `tasks.json` are
  git-ignored; never commit them. Keep `.env` permission 0600.

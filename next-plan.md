# Next plan — Admin-only "Update yt-dlp" button

Status: **Proposed** (not yet implemented)
Scope: feature planning + integration notes for both deployment styles (systemd
** `opt/prox-download-bot` and Docker Compose).

---

## 1. Goal

Add an **admin-only** control that updates the **yt-dlp** binary to the latest
official release, right from Telegram — no SSH needed.

**Why it's worth it**
- yt-dlp releases extremely often (site breaks → new extractor). Keeping it
  current is the difference between "works" and "video downloads fail".
- Today updating means SSH + `curl`; a button makes it one tap and lets the
  admin recover a broken site instantly.
- Fits the existing admin surface (the 👥 Users admin panel already exists).

**Why it's not strictly required**
- Ops convenience, not a core download feature.
- Adds a small attack/risk surface → must be implemented carefully (below).

---

## 2. Functional spec

### 2.1 UI flow
- New button on the **admin control keyboard**: `⚙️ Update yt-dlp`
  (shown only to the configured admin; never to approved users).
- On tap -> inline confirm row: `✅ Update now` / `❌ Cancel`.
  - Confirmation avoids accidental taps and makes the action deliberate.
- On success -> result message, e.g.:
  `✅ yt-dlp updated 2026.01.01 → 2026.11.03`
- On "already latest" -> `ℹ️ yt-dlp is already up to date (2026.11.03)`.
- On failure -> a clear error (download failed / verify failed / not enough
  disk), with the previous binary left intact.

### 2.2 Admin enforcement
- Gate **server-side** on the existing admin check (config `user-id`), not on
  hiding the button. Non-admins never receive the callback/action.
- Add a unit test that a non-admin cannot trigger the update.

### 2.3 Update routine (safe)
1. Read target path from config `organize.ytdlpPath` (default `yt-dlp`).
2. Get current (`yt-dlp --version`) and latest version (GitHub latest release).
   - If already equal → reply "already up to date", stop.
3. Download the official release asset to a **temp file** in the same
   directory (same filesystem, so replace is atomic).
4. **Verify** before replacing:
   - ELF/executable check (`file`-equivalent or byte header);
   - run the downloaded binary `--version` and require it succeeds and parses
     as a version.
5. **Atomic swap**: back up current binary to `<path>.prev`, then
   `os.Rename` temp → target.
   - On any error before/at this step, remove temp and **leave the current
     binary untouched**.
6. Reply old → new version.
7. If step 5 succeeds but the *new* binary is somehow broken (defensive),
   provide a documented rollback: restore `<path>.prev`.

### 2.4 Safety & concurrency
- **Never** build the command from user input. The URL is pinned to the
  official GitHub release; the target path comes from config; the filename is
  fixed. No shell interpolation of user-provided strings.
- A yt-dlp download already in flight keeps its old inode — replacing the
  binary mid-run is safe. Still, block starting an update while a yt-dlp
  task is active (or require explicit admin confirmation) to avoid a broken
  mid-download if the new binary has issues.
- Use `exec.Command` (no `sh -c`) for the whole routine.

---

## 3. Integration points (code map)

- **UI/control keyboard** — `tool/output/telegram/telegram.go`
  - control keyboard builder (`createKeyBoardRow` / admin-only row),
  - inline callback dispatch (mirror the existing `approve~…`/`deny~…`
    pattern: new data like `ytupdate~confirm` / `ytupdate~cancel`),
  - admin-only gate using `adminIDs` / `users.Store`.
- **Updater package** — new `internal/ytdlp/update.go` (or extend
  `internal/ytdlp`):
  - `LatestVersion(ctx)`, `CurrentVersion(bin)`, `Update(ctx, bin)`.
  - Keep it a pure function over the binary path for testability.
- **Config** — already exposes `organize.ytdlpPath` via
  `config.GetOrganizeConfig().YtdlpPath`. Use it as the target path.
- **i18n** — add the new strings to `i18n/` (English).

---

## 4. Docker compatibility (must work in both deploys)

The bot image ships yt-dlp at `/usr/local/bin/yt-dlp` (from `Dockerfile.bot`,
`YTDLP_PATH` default `yt-dlp`). A container filesystem is **ephemeral**: an
update written to `/usr/local/bin` would be lost on container recreation
(rebuild/`docker compose up`). The feature must therefore work durably under
Docker.

### 4.1 Design for Docker
- **Persist the binary on a volume.** Point `ytdlpPath` (env `YTDLP_PATH`)
  at a path on the shared/persistent volume, e.g. `YTDLP_PATH=/data/bin/yt-dlp`
  (the existing `media` volume, or a small dedicated ```` volume mounted in
  the bot and, if needed, the entrypoint).
- **Entrypoint bootstrap.** In `entrypoint.sh`, before starting the bot:
  if the target path doesn't exist yet, copy/symlink the image's default
  `yt-dlp` to it (`cp /usr/local/bin/yt-dlp /data/bin/yt-dlp`). This gives a
  sane starting binary and lets the updater replace it in place.
- **Updater writes to the volume path**, so the update survives restarts and
  `docker compose up -d --build`.
- **Network egress**: the bot container must reach
  `github.com/yt-dlp/yt-dlp/...` (already needed for normal operation; no
  extra firewall change for this feature).
- **Verification identical** in both deploys: same update routine, same
  atomic backup/replace, same result messages. The only difference is *where*
  the binary lives (volume path vs `/usr/local/bin` on systemd hosts).

### 4.2 Systemd (prod `10.10.10.177`) notes
- Binary path is the real `yt-dlp` on the host (or the configured
  `ytdlpPath`). Works with the same routine; `restart` of the service not
  required since the bot shells out per-download.

---

## 5. Testing / verification

- Unit tests: version parse (current/latest), "already latest" short-circuit,
  non-admin rejection, update failure leaves previous binary intact.
- Manual, systemd: click button on real admin account → confirm → see version
  bump message; run a YouTube download to prove the new binary works.
- Manual, Docker: run stack with `YTDLP_PATH=/data/bin/yt-dlp`, update via
  button, **`docker compose up -d --build`** (recreate), then confirm the
  version persists and downloads still work.

---

## 6. Rollback

- Automatic: previous binary kept at `<path>.prev` before every swap.
- Manual (documented in the reply/README): stop bot, restore `.prev`, start.

---

## 7. Out of scope / future

- Scheduled/auto-update on startup or via a timer (could be a follow-up
  option but not part of this button).
- Updating other components (aria2, bot itself) — different topic.
- Multi-arch asset handling beyond the host's architecture.

---

## 8. Deliverables when implemented

- `internal/ytdlp/update.go` (+ tests)
- Telegram control-keyboard button + inline confirm + callback handling + i18n
- `entrypoint.sh` bootstrap (Docker)
- `docker/README.md` + main `README.md` docs of the new admin control
- CHANGELOG entry
- Re-test on systemd prod **and** a Docker stack

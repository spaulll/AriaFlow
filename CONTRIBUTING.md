# Contributing to AriaFlow

Thanks for wanting to help make AriaFlow better! This guide explains how to set
up a dev environment, what the project looks like, and how to get a change
merged.

## Table of contents

- [Code of conduct](#code-of-conduct)
- [Getting started](#getting-started)
- [Project layout](#project-layout)
- [Development workflow](#development-workflow)
- [Building and testing](#building-and-testing)
- [Commit conventions](#commit-conventions)
- [Pull requests](#pull-requests)
- [Do not commit secrets](#do-not-commit-secrets)

## Code of conduct

Be respectful and constructive. This project is open to everyone regardless of
experience level. Aim for kind, specific, and actionable feedback.

## Getting started

AriaFlow is written in **Go 1.24+** with vendored dependencies.

```bash
git clone https://github.com/spaulll/AriaFlow.git
cd AriaFlow
go build -mod=vendor -trimpath -ldflags "-s -w" -o ariaflow ./cmd/DownloadBot
```

To actually run the bot you need `aria2c` with RPC enabled, a Telegram bot
token, and an admin user id — see the [README](README.md#quick-start).

## Project layout

| Path | Contents |
|------|----------|
| `cmd/DownloadBot` | entrypoint (`main.go`) — config loading, wiring, startup |
| `internal/config` | configuration parsing and validation |
| `internal/organize` | media library organizer (movies / series / anime / music / …) |
| `internal/extract` | archive extraction (zip / rar / 7z / tar / …) |
| `internal/users` | multi-user admin-approval model |
| `internal/ytdlp` | yt-dlp integration |
| `tool/controller` | Telegram keyboard / flow controller |
| `tool/input` | Telegram input handling, inline queries, message parsing |
| `tool/output` | Telegram outbound messages and progress rendering |
| `tool/monitor` | download progress / monitoring plumbing |
| `tool/typeTrans` | shared types & conversions |
| `i18n` | translation strings (`active.en.json`) |
| `docker/` | Dockerfile(s) for the compose stack |
| `.github/workflows` | CI build + release automation |

## Development workflow

1. **Open an issue first** for non-trivial changes (features, refactors,
   behavior changes) so we can agree on the approach before you write code.
   Small bug fixes and typo fixes don't need an issue.
2. **Fork the repo** and create a branch off `main`:

   ```bash
   git checkout -b feat/your-feature
   ```

3. Send a **pull request** against `main`. Reference the issue you're fixing
   in the PR description (e.g. `Closes #12`).

## Building and testing

```bash
# full build
go build -mod=vendor -trimpath -o ariaflow ./cmd/DownloadBot

# run the test suite
go test ./internal/... ./tool/output/...
```

Keep tests green before opening a PR. If you add a new behavior, add a test for
it where practical.

## Commit conventions

Use conventional-commit style prefixes. This keeps `CHANGELOG.md` updates easy
and the history tidy.

| Prefix | Use for |
|--------|---------|
| `feat:` | new functionality |
| `fix:` | bug fixes |
| `refactor:` | code changes that don't change behavior |
| `docs:` | documentation only |
| `test:` | tests only |
| `chore:` | tooling, CI, dependencies |
| `perf:` | performance improvements |
| `style:` | formatting, whitespace |
| `revert:` | revert a previous commit |

Breaking changes: append `!` to the type (e.g. `refactor!:`).

Example:

```
feat: add batch pause/resume commands

Adds a command to pause and resume all of a user's downloads at once.
Closes #42.
```

## Pull requests

- Keep PRs **small and focused**. Split unrelated changes into separate PRs.
- Run `gofmt` (or `go fmt ./...`) before committing.
- Update the README if you change setup steps, configuration keys, or behavior.
- If you change user-visible strings, update `i18n/active.en.json`.
- If you add a changelog-worthy change, add a line to `CHANGELOG.md`.
- Squash fixup commits before merging if asked.

## Do not commit secrets

`config.json`, `users.json`, `tasks.json`, and the built binaries are
git-ignored — **never force-add or commit them**. They contain bot tokens and
API credentials. If a secret ever leaks, revoke it via @BotFather immediately.

## License

By contributing, you agree that your contributions are licensed under the same
[Apache-2.0](LICENSE) license as the project.

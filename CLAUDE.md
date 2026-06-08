# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

WhatWhen is a minimalist self-hosted web app for tracking when you last did a recurring task
("changed the beds", "flea-treated the cat"). Each task is a button with a live timer counting
up since it was last reset. No auth, no database — designed to run locally or behind a reverse
proxy and survive reboots.

The guiding principle is **minimalism**: Go standard library only (no third-party Go modules),
a vanilla HTML/CSS/JS frontend (no build step, no framework), and a flat JSON file for storage.
Preserve this when extending the project — reach for a dependency only when the stdlib genuinely
can't do the job.

## Commands

```bash
# Run locally (writes data to ./whatwhen.json instead of /data)
DATA_FILE=./whatwhen.json go run .

# Static binary (CGO disabled — required for the scratch image)
CGO_ENABLED=0 go build -ldflags "-s -w" -o whatwhen .

go vet ./...          # the lint/check step used in this repo

# Docker (compose pulls the published ghcr.io image by default)
docker compose up -d
```

There is **no test suite** yet (`go test ./...` finds nothing). Verify changes by running the
binary and exercising the HTTP API with `curl`, plus a restart to confirm the JSON file persisted.

## Architecture

Three small Go files in `package main`, plus an embedded frontend:

- **`main.go`** — entry point. Reads `PORT` (default `8080`) and `DATA_FILE`
  (default `/data/whatwhen.json`) from the environment, constructs the `Store`, wires routes, serves.
- **`store.go`** — the `Store` type: an in-memory `[]*Item` guarded by a `sync.Mutex`, mirrored to
  disk. Every mutating method (`Add`/`Update`/`Reset`/`Delete`) takes the lock and calls `persist()`
  before returning. `persist()` writes to `DATA_FILE + ".tmp"` then `os.Rename`s it into place
  (atomic write). The on-disk shape is `{"items": [...]}`.
- **`handlers.go`** — HTTP layer using Go 1.22 `ServeMux` method+path patterns
  (e.g. `"PATCH /api/items/{id}"`, read via `r.PathValue("id")`). **No third-party router.** The
  frontend is embedded with `//go:embed web` and served from `/`.

### Key design decision: timers are timestamps, not clocks

The server never "runs" timers. It only stores `createdAt` and `lastReset` (UTC RFC3339). The
**browser** computes elapsed time from `lastReset` and ticks it once per second via a single
`setInterval` (`web/app.js`). This is why a timer stays correct across server downtime and page
closes — and why any new "time-based" feature should follow the same pattern (store a timestamp,
compute on the client) rather than introducing server-side timers.

### HTTP API

| Method | Path                      | Body / Notes                                            |
|--------|---------------------------|---------------------------------------------------------|
| GET    | `/api/items`              | List all items                                          |
| POST   | `/api/items`              | `{ "label" }` — creates with `lastReset = now`          |
| PATCH  | `/api/items/{id}`         | `{ "label"?, "lastReset"? }` — both optional, ≥1 required |
| POST   | `/api/items/{id}/reset`   | Sets `lastReset = now`                                  |
| DELETE | `/api/items/{id}`         | 204 on success                                          |

`PATCH` uses pointer fields (`*string`, `*time.Time` via `ItemUpdate`) so that omitted fields are
left unchanged vs. explicitly set. `lastReset` must be RFC3339. Validation lives in `handlers.go`
(trim label, reject empty, truncate to `maxLabelLen` = 100); `ErrNotFound` from the store maps to
404 via `respondMaybeNotFound`.

### Frontend (`web/`, no build step)

`index.html`, `style.css`, `app.js` are embedded into the binary, so **changes require a rebuild**
to be served (or use `go run .`). `app.js` keeps an in-memory mirror of items and rebuilds the DOM
from it. Two UI state flags live in `localStorage`, not the server:

- **`whatwhen-unlocked`** — a padlock toggle. Locked (default) hides edit/delete and timestamp
  editing for a clean everyday view; this is UI convenience, **not a security boundary**.
- **`whatwhen-theme`** — `"light"`/`"dark"` override of the OS preference. An inline `<head>` script
  applies a saved theme before first paint to avoid a flash; CSS variables drive both palettes via
  `prefers-color-scheme` and a `data-theme` attribute.

## Conventions

- **No standalone database and no auth** are intentional product constraints — don't add either
  without explicit direction.
- Persistence is the source of truth: anything that must survive a reboot has to go through the
  `Store` and land in `DATA_FILE` (mounted at the `/data` volume in Docker).
- Docker image references are **lowercase**: the published image is
  `ghcr.io/markmork/whatwhen:latest`.
- Development happens on a feature branch (currently `claude/zen-keller-C9bGn`), not on the default
  branch.

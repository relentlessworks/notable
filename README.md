# Notable

> Agentic-first notes & knowledge base. The agent IS the interface.

Notable is a notes and knowledge base service designed for AI agents to drive over plain HTTP. No UI, no SDK. The API is the product.

## Quick Start

```bash
make build
./notable
```

The server starts on `:8080` with zero config. Data is persisted to a JSON file automatically.

## How It Works

1. **Get a token**: `POST /auth/request` → `POST /auth/verify` → bearer token
2. **Create a note**: `POST /api/notes` with `title=My Note&body=Content&tags=work,idea`
3. **List notes**: `GET /api/notes` (filter by tag with `?tag=work`)
4. **Get a note**: `GET /api/notes/note_a1b2c` (full note with body)
5. **Update a note**: `PATCH /api/notes/note_a1b2c` with any of `title`, `body`, `tags`
6. **Delete a note**: `DELETE /api/notes/note_a1b2c`
7. **Search notes**: `GET /api/notes/search?q=keyword`

## Principles

- **Plain text by default** — one labeled, grepable line per record. JSON on demand via `Accept: application/json` or `?format=json`.
- **Instructive errors** — every 4xx includes a hint telling the agent what to do next.
- **Self-documenting** — `GET /help` returns a one-page operating manual.
- **Simple auth** — OTP via email → long-lived bearer token.
- **Single static binary** — Go, zero external dependencies, deploys as one file.
- **Zero config defaults** — runs out of the box. Config: defaults < env < flags.
- **Multi-tenant** — workspaces isolate notes per tenant.
- **Short stable handles** — every note gets a workspace-scoped handle like `note_k7m2q`.

## Configuration

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `-addr` | `NOTABLE_ADDR` | `:8080` | Listen address |
| `-db` | `NOTABLE_DB` | `notable.json` | Data file path |
| `-secret` | `NOTABLE_SECRET` | random | Token signing secret |

## Build

```bash
make build    # CGO_ENABLED=0, single static binary
make test     # go test ./...
make vet      # go vet ./...
```

## API Reference

### Authentication

```
POST /auth/request   email=<email>&workspace=<handle>  → OTP code
POST /auth/verify     email=<email>&code=<code>          → Bearer token
```

### Notes (requires Bearer token)

```
POST   /api/notes              title=<title>&body=<content>&tags=<comma-separated>  → handle=note_xxx
GET    /api/notes              → list all notes (optional ?tag=<tag> to filter)
GET    /api/notes/<handle>      → full note with body
PATCH  /api/notes/<handle>      title=<new>&body=<new>&tags=<new>  → updated note
DELETE /api/notes/<handle>      → delete note
GET    /api/notes/search?q=<q>  → search notes by title or body content
GET    /api/workspace           → workspace info
```

### Formats

- **Plain text** (default): `handle=note_a1b2c title=My Note tags=work,idea updated=2024-01-01T00:00:00Z`
- **JSON**: add `Accept: application/json` or `?format=json`

## License

MIT

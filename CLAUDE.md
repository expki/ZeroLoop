# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is ZeroLoop

ZeroLoop is a self-hosted AI assistant platform designed to run entirely on your own hardware. It pairs a Go web server with a local llama.cpp instance so users can interact with an autonomous AI agent through a browser — no cloud AI APIs required.

The core idea is a **project-oriented agentic workspace**: users create projects (each with its own file tree), then chat with an AI agent that can reason, execute code, search the web, read/write project files, delegate to sub-agents, and persist knowledge across conversations. The agent operates in a think-execute-observe loop, using tools to accomplish tasks autonomously rather than just answering questions.

Key goals:
- **Fully self-hosted** — LLM inference runs on a local llama.cpp server; no data leaves your network
- **Agentic, not just conversational** — The agent loop iterates up to 25 times per turn, calling tools, verifying results, and retrying on failure before delivering a final answer
- **Project-scoped** — Each project has isolated files, chats, and context. The agent's file operations are sandboxed to the project directory
- **Extensible** — 16 built-in tools, an extension hook system (13 hook points), swappable agent profiles, MCP protocol support for external tool servers, and a template-based prompt system
- **Single binary deployment** — The React frontend is embedded into the Go binary at compile time with pre-compressed static assets (zstd/brotli/gzip), so the entire app is one file plus a database

## Build & Run

```bash
# Build (requires Go 1.26+)
go build -o zeroloop .

# Run in development
ENVIRONMENT=development PORT=9368 go run .

# Dev with tmux (backend + frontend hot-reload in split windows)
./dev.sh

# Frontend dev server (separate terminal)
cd ui && npm run dev

# Build frontend (output embedded at compile time via dist/)
cd ui && npm run build
```

## Testing

```bash
# All Go tests
go test ./...

# Frontend tests
cd ui && npx vitest run

# Frontend lint
cd ui && npm run lint
```

## Architecture

Single Go binary (`github.com/expki/ZeroLoop.git`) with an embedded React SPA. The server hosts an agentic AI loop that talks to a llama.cpp server for LLM inference.

### Data flow

User (browser) -> WebSocket (`/ws`) -> Hub -> Agent loop (think->execute->observe, max 25 iterations) -> LLM (llama.cpp `/v1/chat/completions` SSE streaming) -> Tool execution -> response broadcast back via WebSocket.

### Key packages

- **`main`** — HTTP server, routing, SPA serving with pre-compressed static assets (zstd/brotli/gzip built at init from `//go:embed dist/*`), TLS cert manager
- **`config`** — Singleton from env vars. Access via `config.Get()` or `config.Load()`
- **`database`** — GORM (SQLite/Postgres). Access via `database.Get()`. Models registered in `database.AutoMigrate()`
- **`models`** — GORM models: `Project`, `ProjectFile`, `Chat`, `Message` (11 message types)
- **`search`** — Bleve full-text search index. Package-level functions: `search.Init()`, `search.Index()`, `search.Search()`, `search.Delete()`
- **`filemanager`** — Per-project file operations with path traversal protection and read/write locking
- **`llm`** — HTTP client for llama.cpp (streaming SSE chat completions, infill/FIM, tokenize, slot save/restore for KV cache)
- **`agent`** — Agent loop, tool registry, prompt templates, extension system, profile loading
- **`api`** — REST handlers (`handler.go`), WebSocket hub/client architecture (`ws.go`), project file API (`files.go`, `projects.go`)
- **`tools`** — 16 agent tools implementing the `agent.Tool` interface
- **`mcp`** — JSON-RPC 2.0 client for external MCP tool servers (protocol 2024-11-05)
- **`middleware`** — Rate limiting (semaphore, 20 concurrent/45s timeout), security headers, cache headers, compression, JWT auth

### Startup sequence (main.go)

godotenv -> config -> logger -> database -> migrations -> projects dir -> search index -> file manager -> LLM client -> WebSocket hub -> routes -> middleware chain -> listen

### Agent system

The agent loop lives in `agent/agent.go:runLoop()`. Each chat gets a persistent `Agent` instance (stored in `Hub.agents` map). The agent is stateful — its `History` persists across messages within a chat.

**Tool interface** (`agent/tool.go`): `Name()`, `Description()`, `Parameters()` (JSON Schema), `Execute(ctx, agent, args)`. Tools return `ToolResult{Message, BreakLoop}`. The `response` tool sets `BreakLoop: true` to end the loop.

**Tool registration** happens in `api/ws.go:NewHub()`. To add a tool: create file in `tools/`, implement `agent.Tool`, register in `NewHub()`.

**Prompt templates** (`prompts/`): Uses `{{include file}}` and `{{variable}}` syntax. Profile-specific overrides in `agents/{profile}/prompts/`. Profiles (default, developer, researcher, hacker) are JSON in `agents/{profile}/profile.json`.

**Extension system** (`agent/extensions.go`): 13 hook points with priority-ordered execution. Register via `agent.RegisterExtension(point, ext)`.

**History management**: Compression via LLM summarization when >50 messages, fallback pruning at 100. Tool repeat detection (max 3 identical calls).

### WebSocket protocol

- **Client->Server**: `subscribe`, `send_message`, `cancel`, `pause`, `resume`, `clear`, `intervene`
- **Server->Client**: `message`, `stream`, `chat_update`, `clear`, `file_event`

Messages queued when agent is already running (processed sequentially). Pause saves/restores llama.cpp KV cache slots for fast resume.

### Frontend (`ui/`)

React 19, TypeScript 5.7, Vite 6.1, Zustand 5 (stores: `chatStore`, `projectStore`, `uiStore`). CodeMirror for file editing with ghost text completions via `/api/completions` (llama.cpp infill). Testing: Vitest + @testing-library/react + jsdom.

### REST API

Routes registered in `api/handler.go:RegisterRoutes()`. Uses Go 1.22+ `"METHOD /path"` routing with `{param}` and `{path...}` wildcards.

- `/api/projects` — CRUD for projects
- `/api/projects/{id}/files` and `/api/projects/{id}/files/{path...}` — File operations
- `/api/chats` — CRUD, export, branch
- `/api/chats/{id}/messages` — Message history
- `/api/health/llm` — LLM health check
- `/api/completions` — Code completion (FIM/infill)

## Environment Variables

All config via env vars (or `.env`). Key ones:

| Variable | Default | Notes |
|----------|---------|-------|
| `PORT` | `9368` | Server port |
| `ENVIRONMENT` | `development` | `development` or `production` |
| `DB_DRIVER` | `sqlite` | `sqlite` or `postgres` |
| `DATABASE_URL` | `<base_dir>/zeroloop.db` | SQLite path or Postgres DSN |
| `LLM_BASE_URL` | `http://192.168.10.15:8081` | llama.cpp server URL |
| `SEARXNG_URL` | empty | SearXNG instance for web search tool |
| `PROJECTS_DIR` | `<base_dir>/projects` | Project files root directory |

## Patterns

- **Logger**: `logger.Log.Infow("msg", "key", val)` — Zap sugared logger, always use `w`-suffix methods
- **Adding GORM models**: Define in `models/`, add to `database.AutoMigrate()` slice
- **Adding routes**: Register in `api/handler.go:RegisterRoutes()`. Use `writeJSON()` and `writeError()` helpers
- **Adding tools**: Create file in `tools/`, implement `agent.Tool` interface, register in `api/ws.go:NewHub()`
- **Adding extensions**: `agent.RegisterExtension(hookPoint, agent.Extension{Name, Priority, Fn})`
- **Database access**: `database.Get()` returns `*gorm.DB`
- **Config access**: `config.Get()` — singleton, safe after init
- **Frontend stores**: Zustand stores in `ui/src/stores/`. WebSocket service in `ui/src/services/websocket.ts`, REST API in `ui/src/services/api.ts`

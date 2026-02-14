# Forge — Project Plan

## Vision

Forge is a local-first agentic AI platform for learning and building with AI agents. The goal is a modular, extensible system where LLMs can use tools, execute code safely, and interact with external services — all running on your own hardware with your choice of model.

## Completed

### Phase 1 — Foundation
- [x] Project scaffold with Go modules
- [x] Cobra CLI with `chat` command
- [x] OpenAI-compatible LLM client (works with Ollama, Claude, Gemini)
- [x] ReAct agent loop (Think → Act → Observe)
- [x] Interactive readline-based chat with history
- [x] Basic configuration loading via Viper

### Phase 2 — Tools & Streaming
- [x] MCP tool registry and client (stdio transport)
- [x] 5 MCP tool servers: shell-exec, file-ops, web-search, github-ops, code-runner
- [x] Docker sandbox for code execution with resource limits
- [x] Agent profiles (swappable system prompts, tools, providers)
- [x] Streaming token output with callbacks
- [x] Rate limit handling with retry/backoff
- [x] Ollama interactive model selection
- [x] Signal handling (Ctrl+C cancellation, graceful shutdown)

## Roadmap

### Phase 3 — Persistence & Context
- [x] Conversation persistence (save/load chat sessions)
- [x] Session management (list, resume, delete past conversations)
- [x] Chat export (markdown, JSON)
- [ ] RAG pipeline — chunk, embed, and retrieve local documents
- [ ] Vector storage backend (SQLite with vector extensions or similar)
- [ ] Context window management (summarization, sliding window)

### Phase 4 — Server Mode & API
- [ ] HTTP/WebSocket server (`forge serve`)
- [ ] REST API for chat, sessions, and tool management
- [ ] Multi-session support (concurrent conversations)
- [ ] API key authentication
- [ ] Web UI frontend (lightweight, single-page)

### Phase 5 — Advanced Agents
- [ ] Multi-agent orchestration (agents delegating to sub-agents)
- [ ] Planning and task decomposition
- [ ] Agent memory (long-term facts, preferences, learned patterns)
- [ ] Custom tool authoring guide and scaffolding
- [ ] Tool approval policies (ask before executing, auto-approve safe tools)

### Phase 6 — Ecosystem
- [ ] Plugin system for community tool servers
- [ ] Pre-built agent templates (researcher, sysadmin, data analyst)
- [ ] Ollama model management (pull, delete, show info)
- [ ] Provider health checks and automatic fallback
- [ ] Telemetry and usage dashboards (local only)

## Design Principles

1. **Local-first** — Runs on your machine, your models, your data. Cloud providers are optional.
2. **Modular** — Every tool is a separate process speaking MCP. Swap, disable, or add tools without touching core code.
3. **Provider-agnostic** — Any OpenAI-compatible API works. Ollama, Claude, Gemini, or your own endpoint.
4. **Safe by default** — Code runs in Docker sandboxes. Tools can be restricted per profile. No silent destructive actions.
5. **Simple to extend** — Adding a new tool means writing a small Go binary that speaks MCP over stdio.

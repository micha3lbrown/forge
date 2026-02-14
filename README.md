# Forge

A local agentic AI platform built in Go. Forge connects to multiple LLM providers and gives them tools via the Model Context Protocol (MCP), creating an interactive agent that can execute shell commands, read/write files, search the web, run code in Docker sandboxes, and more.

## Features

- **Multi-provider LLM support** — Ollama (local), Claude, Gemini via OpenAI-compatible API
- **ReAct agent loop** — Think → Act → Observe cycle with configurable iteration limits
- **MCP tool system** — Modular tool servers communicating over stdio
- **Streaming output** — Real-time token streaming with tool call/result callbacks
- **Agent profiles** — Swappable personalities with different system prompts, tools, and providers
- **Docker sandbox** — Secure code execution with resource limits and network isolation
- **Interactive CLI** — Readline-based chat with history, slash commands, and Ctrl+C cancellation

## Quick Start

### Prerequisites

- Go 1.23+
- Docker (for code execution sandbox)
- An LLM provider: [Ollama](https://ollama.com) running locally, or API keys for Claude/Gemini

### Setup

```bash
# Clone the repo
git clone https://github.com/michaelbrown/forge.git
cd forge

# Copy and configure environment variables
cp .env.example .env
# Edit .env with your API keys (ANTHROPIC_API_KEY, GEMINI_API_KEY, etc.)

# Build everything (CLI + tool servers)
make all

# Start chatting
make chat
```

### Usage

```bash
# Default provider (Ollama)
./bin/forge chat

# Specify a provider
./bin/forge chat --provider claude
./bin/forge chat --provider gemini

# Use a specific model
./bin/forge chat --provider ollama --model qwen3:14b

# Use an agent profile
./bin/forge chat --profile coder
```

### Slash Commands

| Command    | Description                  |
|------------|------------------------------|
| `/help`    | Show available commands      |
| `/quit`    | Exit the chat                |
| `/reset`   | Clear conversation history   |
| `/history` | Show conversation history    |

## Architecture

```
User ──► CLI (cmd/forge) ──► Agent (ReAct Loop) ──► LLM Client
                                    │                     │
                                    ▼                     ▼
                             MCP Tool Registry     Provider API
                                    │              (Ollama/Claude/Gemini)
                                    ▼
                             Tool Servers (stdio)
                             ├── shell-exec
                             ├── file-ops
                             ├── web-search
                             ├── github-ops
                             └── code-runner
```

### Project Structure

```
cmd/
  forge/              CLI entry point and chat loop
  tools/              MCP tool server binaries
    shell-exec/       Shell command execution
    file-ops/         File read/write/patch/list
    web-search/       Web search (Tavily) and fetch
    github-ops/       GitHub PR/issue operations
    code-runner/      Docker-based code execution
internal/
  agent/              ReAct agent loop and profiles
  llm/                LLM client (OpenAI-compatible)
  tools/              MCP registry and client
  config/             Configuration loading (Viper)
  sandbox/            Docker sandbox with security policies
  server/             Server mode (planned)
  rag/                RAG pipeline (planned)
configs/
  agents/             Agent profile definitions (YAML)
```

## Configuration

Forge is configured via `forge.yaml` in the project root:

```yaml
providers:
  ollama:
    base_url: "http://localhost:11434/v1/"
    api_key: "ollama"
    models:
      default: "qwen3:14b"
  claude:
    base_url: "https://api.anthropic.com/v1/"
    api_key: "${ANTHROPIC_API_KEY}"
    models:
      default: "claude-sonnet-4-5-20250929"
  gemini:
    base_url: "https://generativelanguage.googleapis.com/v1beta/openai/"
    api_key: "${GEMINI_API_KEY}"
    models:
      default: "gemini-2.0-flash"

default_provider: ollama
```

Environment variables are expanded at load time. Set them in your `.env` file or export them in your shell.

### Agent Profiles

Profiles live in `configs/agents/` as YAML files. Each profile can override the system prompt, available tools, provider, and iteration limits.

```yaml
name: coder
provider: gemini
system_prompt: |
  You are Forge Coder, a specialized AI coding assistant.
tools:
  - shell_exec
  - file_read
  - file_write
  - file_patch
  - code_run
max_iterations: 15
```

## MCP Tool Servers

Each tool server is a standalone binary that speaks the [Model Context Protocol](https://modelcontextprotocol.io) over stdio. Tools are registered in `forge.yaml` and launched on demand by the agent.

| Server       | Tools                                          | Description                         |
|--------------|-------------------------------------------------|-------------------------------------|
| shell-exec   | `shell_exec`                                   | Run shell commands                  |
| file-ops     | `file_read`, `file_write`, `file_patch`, `file_list` | File system operations        |
| web-search   | `web_search`, `web_fetch`                      | Search (Tavily API) and fetch URLs  |
| github-ops   | `github_list_prs`, `github_list_issues`, `github_view_pr`, `github_repo_info` | GitHub integration via `gh` CLI |
| code-runner  | `code_run`                                     | Execute code in Docker containers   |

## Development

```bash
make build        # Build CLI only
make build-tools  # Build all tool servers
make all          # Build everything
make test         # Run tests
make clean        # Remove build artifacts
```

## License

MIT

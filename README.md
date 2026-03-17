# tinybot

tinybot is a lightweight personal AI assistant written in Go. It combines LLM chat, tool execution, scheduled tasks, persistent memory, multi-channel messaging, and a desktop client with a Cherry Studio-inspired interface.

This project is a Go rewrite of [nanobot](https://github.com/HKUDS/nanobot).

## Highlights

- LLM chat with session persistence and conversation summaries
- Built-in tools for shell execution, file editing, web search, and web fetch
- Memory workspace with long-term notes and daily logs
- Cron-based scheduled tasks
- Console and Telegram channels
- Desktop client built with Wails
- Thinking/reasoning stream support in the desktop chat UI
- Client-side model settings, including custom provider/model entries

## Desktop Client

The desktop app lives in `frontend/` and is packaged with Wails.

Current desktop features include:

- Assistant lane, conversation lane, and chat workspace
- Streaming chat UI with message metadata, reasoning blocks, and tool result blocks
- Settings drawer for runtime tuning
- Custom rename dialog for conversations
- Provider/model settings UI where users can:
  - switch the active provider
  - edit built-in provider model settings
  - add custom provider/model entries from the client
  - configure `name`, `kind`, `model`, `api base`, and `api key`

Build the desktop binary with:

```bash
wails build -tags desktop -m
```

The output binary is written to `build/bin/`.

## Core Features

- `chat`: direct prompt/response workflows
- `tools`: local execution and web helpers
- `memory`: long-term memory and daily note storage
- `cron`: recurring and one-shot scheduled jobs
- `gateway`: long-running runtime for connected channels
- `desktop`: Wails-based local client

## Built-in Tools

| Tool | Purpose |
| --- | --- |
| `exec` | Run shell commands |
| `read_file` | Read file contents |
| `write_file` | Create or overwrite files |
| `edit_file` | Search/replace file edits |
| `list_dir` | List directory contents |
| `web_search` | Search the web |
| `web_fetch` | Fetch and extract webpage content |
| `message` | Send a message to a channel |

## Requirements

- Go 1.21+
- Node.js and npm for the desktop frontend
- Wails CLI for desktop builds
- At least one configured model provider

## Quick Start

Clone and build:

```bash
git clone https://github.com/yourusername/tinybot.git
cd tinybot
go build ./cmd/tinybot
```

Initialize the workspace:

```bash
./tinybot.exe onboard
```

Run a direct chat:

```bash
./tinybot.exe "What can you do?"
```

Run the gateway:

```bash
./tinybot.exe gateway
```

Check status:

```bash
./tinybot.exe status
```

## CLI Commands

```bash
tinybot <message>              # send a direct prompt
tinybot onboard                # initialize workspace files
tinybot status                 # show workspace and runtime status
tinybot gateway                # run the long-lived gateway

tinybot cron list
tinybot cron add <name> <seconds> <prompt>
tinybot cron add-cron <name> <cron_expr> <prompt>
tinybot cron add-at <name> <rfc3339_time> <prompt>
tinybot cron remove <task_id>
tinybot cron run-once
```

## Configuration

Configuration is stored in `config.json` by default.

You can override the path with:

```bash
set TINYBOT_CONFIG=D:\path\to\config.json
```

Example:

```json
{
  "agents": {
    "workspace": "./workspace",
    "max_tokens": 8192,
    "temperature": 0.7,
    "max_tool_iterations": 20,
    "enable_thinking": true
  },
  "providers": {
    "active": "qwen",
    "list": {
      "qwen": {
        "kind": "qwen",
        "model": "qwen3-max",
        "api_key": "",
        "api_base": "https://dashscope.aliyuncs.com/compatible-mode/v1"
      },
      "openai": {
        "kind": "openai",
        "model": "gpt-4o-mini",
        "api_key": "",
        "api_base": "https://api.openai.com/v1"
      }
    }
  },
  "channels": {
    "console": {
      "enabled": true
    },
    "telegram": {
      "enabled": false,
      "token": ""
    }
  }
}
```

### Environment Variables

| Variable | Purpose |
| --- | --- |
| `TINYBOT_CONFIG` | Custom config file path |
| `QWEN_API_KEY` | Qwen API key |
| `QWEN_API_BASE` | Qwen API base |
| `QWEN_MODEL` | Qwen default model |
| `OPENAI_API_KEY` | OpenAI API key |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token |

## Custom Models From the Client

You can now add custom provider/model entries directly in the desktop settings UI.

Open the client settings and use the model settings section to:

1. add a new custom entry
2. set a provider `name`
3. set its `kind`
4. set the `model`
5. set the `api base`
6. optionally set the `api key`
7. save and switch the active provider

These settings are written back through the desktop config save flow and persisted to `config.json`.

## Project Layout

```text
cmd/tinybot/                CLI entrypoint
internal/
  app/                      config, bootstrap, and shared setup
  desktop/                  desktop service and Wails bindings
  repository/               persistence for sessions and cron jobs
  service/                  chat, cron, and orchestration flows
  transport/                channels and gateway runtime
  adapters/                 providers, tools, and workspace helpers
frontend/
  src/app/                  desktop app state and bindings
  src/features/             UI features such as assistants, topics, chat, settings
build/bin/                  built desktop binaries
workspace/                  memory, session, and runtime files
```

## Memory Workspace

tinybot stores long-term memory and daily notes under `workspace/memory/`.

Typical layout:

```text
workspace/
  memory/
    MEMORY.md
    2026-03-16.md
    2026-03-17.md
```

## Development

Run Go tests:

```bash
go test ./...
```

Run a narrower set:

```bash
go test ./internal/desktop ./internal/repository/sessionrepo
```

Build the frontend only:

```bash
cd frontend
npm install
npm run build
```

Build the desktop app:

```bash
wails build -tags desktop -m
```

## License

MIT

## Credits

- [nanobot](https://github.com/HKUDS/nanobot)
- [openai-go](https://github.com/openai/openai-go)
- [robfig/cron](https://github.com/robfig/cron)
- [Wails](https://wails.io/)

# Perplexity CLI

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A fast and simple command-line client for the Perplexity API with interactive chat mode, built with Go.

## Features

- **Interactive Chat Mode** - Conversational interface with history persistence and context
- **Real-time Streaming** - SSE-powered live response streaming
- **Formatted Output** - Rendered markdown with syntax highlighting
- **Multiple API Keys** - Automatic rotation on failure or rate limits
- **Token Statistics** - Optional usage tracking and citations
- **Pipe Input Support** - Use with shell pipelines
- **Export Conversations** - Save chats to markdown files
- **Cross-platform** - macOS, Linux, and Windows support

## Installation

### From Source

```bash
git clone https://github.com/quocvuong92/perplexity-cli.git
cd perplexity-cli
make install
```

### Configuration

```bash
# Single key
export PERPLEXITY_API_KEY="your-api-key"

# Multiple keys (automatic rotation)
export PERPLEXITY_API_KEYS="key1,key2,key3"

# Optional: custom timeout (default: 120 seconds)
export PERPLEXITY_TIMEOUT=180
```

Add to `~/.bashrc` or `~/.zshrc` for persistence.

## Usage

### Quick Start

```bash
# Simple query
perplexity "What is quantum computing?"

# Streaming with rendered output
perplexity -sr "Explain relativity"

# Interactive mode
perplexity -i

# Save response to file
perplexity -o response.md "Explain Docker"

# Pipe input
echo "What is Go?" | perplexity
cat question.txt | perplexity -sr
```

### Options

| Flag | Description |
|------|-------------|
| `-i, --interactive` | Interactive chat mode with conversation history |
| `-s, --stream` | Stream output in real-time |
| `-r, --render` | Render markdown with colors and formatting |
| `-c, --citations` | Display citations |
| `-u, --usage` | Show token usage statistics |
| `-m, --model` | Choose model (default: sonar-pro) |
| `-o, --output` | Save response to file |
| `-a, --api-key` | Override API key |
| `-v, --verbose` | Enable verbose logging |
| `--list-models` | List available models |

### Interactive Mode

Launch an interactive session with persistent conversation context:

```bash
perplexity -isr
```

**Commands:**

| Command | Description |
|---------|-------------|
| `/model [name]`, `/m` | Switch or show current model |
| `/citations [on\|off]` | Toggle citations display |
| `/history` | Show recent conversations |
| `/search <keyword>` | Search conversation history |
| `/resume [n]` | Resume conversation (n=index from /history) |
| `/delete <n>` | Delete conversation (n=index from /history) |
| `/retry`, `/r` | Retry last message |
| `/copy` | Copy last response to clipboard |
| `/export [filename]` | Export conversation to markdown |
| `/system [prompt\|reset]` | Show/set/reset system prompt |
| `/clear`, `/c` | Reset conversation |
| `/help`, `/h` | Display commands |
| `/exit`, `/quit`, `/q` | Exit session |

**Tips:**
- Press `Ctrl+C` during a response to cancel without exiting
- Use `\` at end of line for multiline input
- Tab completion available for commands

### Available Models

| Model | Description |
|-------|-------------|
| `sonar-pro` | Professional search (default) |
| `sonar` | Base search model |
| `sonar-reasoning-pro` | Advanced reasoning capabilities |
| `sonar-reasoning` | Standard reasoning model |
| `sonar-deep-research` | Deep research analysis |

## Building

```bash
make build          # Current platform
make build-darwin   # macOS (Universal)
make build-all      # All platforms
make test           # Run tests
```

## Requirements

- Go 1.24+
- Perplexity API key ([Get one here](https://www.perplexity.ai/settings/api))

## License

MIT License - see [LICENSE](LICENSE) for details.

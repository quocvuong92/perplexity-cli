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
| `-a, --api-key` | Override API key |
| `-v, --verbose` | Enable verbose logging |

### Interactive Mode

Launch an interactive session with persistent conversation context:

```bash
perplexity -isr
```

**Commands:**

| Command | Description |
|---------|-------------|
| `/model [name]`, `/m` | Switch or show model |
| `/citations [on\|off]` | Toggle citations display |
| `/history` | Show recent conversations |
| `/resume` | Resume last conversation |
| `/clear`, `/c` | Reset conversation |
| `/help`, `/h` | Display commands |
| `/exit`, `/q` | Exit session |

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
```

## Requirements

- Go 1.24+
- Perplexity API key ([Get one here](https://www.perplexity.ai/settings/api))

## License

MIT License - see [LICENSE](LICENSE) for details.

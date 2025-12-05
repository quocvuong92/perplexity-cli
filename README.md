## Perplexity CLI

Perplexity CLI is a simple and convenient command-line client for the Perplexity API, allowing users to quickly ask questions and receive answers directly from the terminal.

![screen](docs/screen.png)

## Features

- Easy querying of the Perplexity API
- Support for various language models
- Real-time streaming output (SSE)
- Optional display of token usage statistics
- Optional display of citations
- Markdown output format for easy copying
- Rendered markdown output with colors and formatting
- API key handling from environment variable or command-line argument
- Multiple API keys support with automatic rotation
- Cross-platform support (macOS, Linux, Windows)

## Requirements

- Go 1.21+ (for building from source)
- Perplexity API key

## Installation

### From Source

```bash
git clone https://github.com/quocvuong92/perplexity-cli.git
cd perplexity-cli
make install
```

### Manual Installation

```bash
# Build the binary
make build

# Copy to your PATH
cp perplexity /usr/local/bin/
```

### Set API Key

```bash
# Single API key
export PERPLEXITY_API_KEY="your-api-key"

# Multiple API keys (comma-separated) - automatically rotates on failure
export PERPLEXITY_API_KEYS="key1,key2,key3"
```

Add to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.) for persistence.

> **Note:** When using multiple keys via `PERPLEXITY_API_KEYS`, the CLI will automatically switch to the next key if the current one fails (e.g., due to rate limits or exhausted credits).

## Usage

```bash
perplexity "What is the meaning of life?"
```

### With Streaming (Real-time Output)

```bash
perplexity -s "Explain quantum computing"
```

### With Additional Options

```bash
perplexity -scu "Explain Einstein's theory of relativity"
```

## Options

| Flag | Description |
|------|-------------|
| `-s, --stream` | Stream output in real-time |
| `-r, --render` | Render markdown with colors and formatting |
| `-u, --usage` | Show token usage statistics |
| `-c, --citations` | Show citations |
| `-m, --model` | Choose the language model (default: sonar-pro) |
| `-a, --api-key` | Set the API key (defaults to `PERPLEXITY_API_KEYS` or `PERPLEXITY_API_KEY` env var) |
| `-v, --verbose` | Enable debug mode |

## Available Models

- sonar-reasoning-pro
- sonar-reasoning
- sonar-pro
- sonar

## Building

```bash
# Build for current platform
make build

# Build for macOS (arm64 + amd64)
make build-darwin

# Build for all platforms
make build-all

# Clean build artifacts
make clean
```

## License

This project is released under the MIT License.

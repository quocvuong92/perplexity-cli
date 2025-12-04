## Perplexity CLI

Perplexity CLI is a simple and convenient command-line client for the Perplexity API, allowing users to quickly ask questions and receive answers directly from the terminal.

![screen](docs/screen.png)

## Features

- Easy querying of the Perplexity API
- Support for various language models
- Optional display of token usage statistics
- Optional display of citations
- Colorful output formatting (with glow support)
- API key handling from environment variable or command-line argument
- Cross-platform support (macOS, Linux, Windows)

## Requirements

- Go 1.21+ (for building from source)
- Perplexity API key

## Installation

### From Source

```bash
git clone https://github.com/quocvuong92/perplexity-api.git
cd perplexity-api
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
export PERPLEXITY_API_KEY="your-api-key"
```

Add to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.) for persistence.

## Usage

```bash
perplexity "What is the meaning of life?"
```

### With Additional Options

```bash
perplexity -uc -m sonar-pro "Explain Einstein's theory of relativity"
```

## Options

| Flag | Description |
|------|-------------|
| `-v, --verbose` | Enable debug mode |
| `-u, --usage` | Show token usage statistics |
| `-c, --citations` | Show citations |
| `-g, --glow` | Use Glow-compatible formatting |
| `-a, --api-key` | Set the API key (optional, defaults to `PERPLEXITY_API_KEY` env var) |
| `-m, --model` | Choose the language model (default: sonar-pro) |

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

## Author

Dawid Szewc (original Python version)

# Codebase Information

## Project Overview

**Name:** Axe  
**Repository:** github.com/jrswab/axe  
**Language:** Go 1.25.0  
**License:** Apache-2.0  
**Total Files:** 223  
**Lines of Code:** 28,818  
**Size Category:** Large (L)

## Description

Axe is a CLI tool for managing and running LLM-powered agents. It treats LLM agents like Unix programs - small, focused, and composable. Each agent is defined in a TOML configuration file with its own system prompt, model selection, skill files, context files, working directory, persistent memory, and the ability to delegate to sub-agents.

## Technology Stack

### Core Dependencies
- **cobra** (v1.10.2) - CLI framework
- **toml** (v1.6.0) - Configuration parsing
- **go-sdk** (v1.4.0) - Model Context Protocol support
- **golang.org/x/net** (v0.51.0) - HTTP/2 support

### Indirect Dependencies
- google/jsonschema-go - JSON schema validation
- segmentio/encoding - High-performance JSON encoding
- golang.org/x/oauth2 - OAuth2 authentication

## Supported Providers

1. **Anthropic** - Claude models via Anthropic API
2. **OpenAI** - GPT models via OpenAI API
3. **Ollama** - Local models via Ollama
4. **OpenCode** - Multi-model provider supporting Claude, GPT, and Gemini routes
5. **AWS Bedrock** - AWS-hosted models via Bedrock Converse API

## Directory Structure

```
axe/
├── cmd/                    # CLI commands and entry points
├── internal/
│   ├── agent/             # Agent configuration and management
│   ├── config/            # Global configuration handling
│   ├── envinterp/         # Environment variable interpolation
│   ├── mcpclient/         # Model Context Protocol client
│   ├── memory/            # Persistent memory system
│   ├── provider/          # LLM provider implementations
│   ├── refusal/           # Refusal detection logic
│   ├── resolve/           # Path and context resolution
│   ├── testutil/          # Testing utilities and mock servers
│   ├── tool/              # Built-in tool implementations
│   ├── toolname/          # Tool name constants
│   └── xdg/               # XDG Base Directory support
├── docs/
│   ├── design/            # Architecture and design documents
│   ├── plans/             # Implementation plans and specs
│   └── skills/            # Skill documentation
├── examples/              # Example agent configurations
├── skills/                # Embedded skill files
└── main.go                # Application entry point
```

## Key Features

- Multi-provider LLM support (Anthropic, OpenAI, Ollama, OpenCode, AWS Bedrock)
- TOML-based agent configuration
- Sub-agent delegation with depth limiting
- Persistent memory with garbage collection
- Skill system for reusable instruction sets
- Built-in tools (file operations, shell commands)
- Model Context Protocol (MCP) integration
- Stdin piping support
- Dry-run mode
- JSON output format
- Docker containerization

## Build and Test

- **Build:** `go build .`
- **Test:** `go test ./...`
- **Lint:** Uses `.golangci.yml` configuration
- **Release:** GoReleaser with `.goreleaser.yml`

## CI/CD

GitHub Actions workflows:
- `go.yml` - Go testing and linting
- `release.yml` - Automated releases

## Configuration

- **Config Directory:** `$XDG_CONFIG_HOME/axe/` (default: `~/.config/axe/`)
- **Data Directory:** `$XDG_DATA_HOME/axe/` (default: `~/.local/share/axe/`)
- **Agent Files:** `$XDG_CONFIG_HOME/axe/agents/*.toml`
- **Skills:** `$XDG_CONFIG_HOME/axe/skills/`

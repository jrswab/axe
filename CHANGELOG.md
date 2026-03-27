# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.6.0] - 2026-03-24

### Added
- Artifact management system (#47)
- Inline prompt flag for `axe run` (#46)
- mdBook documentation site and deploy workflow (#50)

### Changed
- Expanded release skill with version bump, README, and GHCR steps

## [1.5.0] - 2026-03-20

### Added
- Output allowlist for outgoing connections (#45)
- Local agent directory support (#41)
- Token budget limits per agent run (#37)

### Changed
- Consolidate Docker release workflow (#44)
- Bump github.com/modelcontextprotocol/go-sdk from 1.4.0 to 1.4.1 (#39)

## [1.4.0] - 2026-03-17

### Added
- AWS Bedrock provider support (#18)
- Retry logic for intermittent LLM provider failures (#35)

### Fixed
- Add path sandbox enforcement to `run_command` tool (#31)
- Update Dockerfile to Go 1.25 and bump CI actions to Node 24

## [1.3.0] - 2026-03-15

### Added
- Google Gemini provider support
- MiniMax provider support (#27)
- GHCR docker publish workflow on version tags (#33)

### Fixed
- Use max_completion_tokens for OpenAI provider (#34)

## [1.2.0] - 2026-03-10

### Added
- OpenCode provider support
- HTTP request timeout (15s per-request) for `url_fetch` tool
- HTML tag stripping for `url_fetch` responses

## [1.1.1] - 2026-03-06

### Fixed
- Remove hardcoded `/v1` from OpenAI provider URL path

## [1.1.0] - 2026-03-06

### Added
- MCP tool support
- `web_search` built-in tool using Tavily Search API
- `url_fetch` built-in tool
- JSON `tool_call_details` in run output
- Refusal detection in JSON output
- Executable examples

### Changed
- Docker docs for running a single agent with selective mounts

## [1.0.0] - 2026-03-02

### Added
- Core CLI with `run`, `agents list`, `agents show`, `agents init`, `agents edit`, `gc`, and `config init` commands
- TOML-based agent configuration with SKILL.md context support
- Multi-provider LLM support: Anthropic, OpenAI, and Ollama
- Tool-calling system with `read_file`, `write_file`, `edit_file`, `list_directory`, and `run_command` tools
- Sub-agent orchestration with `call_agent` tool and depth-limited execution
- Agent memory system with XDG-compliant data storage and garbage collection
- Path security: `~` and `$VAR` expansion for user-supplied paths
- Stdin piping support for composing with Unix tools
- Dry-run mode and per-tool verbose logging
- Docker containerization support
- CI workflows: lint, test, build, and GoReleaser

### Fixed
- Arg validation UX and skill path resolution
- staticcheck/errcheck lint issues
- Duplicate error silencing, glob validation, and nil ExitError guard

[unreleased]: https://github.com/jrswab/axe/compare/v1.6.0...HEAD
[1.6.0]: https://github.com/jrswab/axe/compare/v1.5.0...v1.6.0
[1.5.0]: https://github.com/jrswab/axe/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/jrswab/axe/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/jrswab/axe/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/jrswab/axe/compare/v1.1.1...v1.2.0
[1.1.1]: https://github.com/jrswab/axe/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/jrswab/axe/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/jrswab/axe/releases/tag/v1.0.0

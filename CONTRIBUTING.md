# Contributing to Axe

Thank you for your interest in contributing to Axe! This document provides guidelines and information for contributors.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Code Style and Standards](#code-style-and-standards)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Project Structure](#project-structure)
- [Design Principles](#design-principles)

## Getting Started

### Prerequisites

- Go 1.25.0 or later
- Git
- A text editor or IDE with Go support
- (Optional) Docker for container testing
- (Optional) golangci-lint for linting

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/axe.git
   cd axe
   ```
3. Add the upstream repository:
   ```bash
   git remote add upstream https://github.com/jrswab/axe.git
   ```

## Development Setup

### Build from Source

```bash
go build .
```

This creates an `axe` binary in the current directory.

### Run Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests verbosely
go test -v ./...

# Run specific package tests
go test ./internal/agent/
```

### Update Golden Files

Golden file tests capture expected CLI output. To update them after intentional changes:

```bash
UPDATE_GOLDEN=1 go test ./cmd/
```

### Linting

```bash
golangci-lint run
```

Configuration is in `.golangci.yml`.

## Code Style and Standards

### General Guidelines

- **Follow Go conventions** - Use `gofmt`, follow effective Go practices
- **Write clear, self-documenting code** - Prefer clarity over cleverness
- **Keep functions small and focused** - Each function should do one thing well
- **Avoid global state** - Pass dependencies explicitly
- **Error messages should help users** - Explain what went wrong and how to fix it

### Specific Patterns

**Resolution Order:**
Flags override TOML overrides environment variables override defaults. This pattern is used throughout the codebase.

**Output:**
- Clean output to stdout (safe to pipe)
- Debug information to stderr
- Use `--verbose` flag for detailed logging
- Use `--json` flag for structured output

**Exit Codes:**
- `0` - Success
- `1` - Runtime error
- `2` - Configuration error

**Path Handling:**
- Support tilde expansion (`~/path`)
- Support environment variable expansion (`$HOME/path`, `${VAR}/path`)
- Validate paths to prevent directory traversal
- Sandbox file operations to working directory

## Testing

### Test Organization

- **Unit tests** - `*_test.go` files alongside implementation
- **Integration tests** - `cmd/*_integration_test.go` for end-to-end testing
- **Smoke tests** - `cmd/smoke_test.go` for real binary execution
- **Golden file tests** - `cmd/golden_test.go` for CLI output validation

### Writing Tests

**Table-Driven Tests:**

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:  "valid input",
            input: "test",
            want:  "result",
        },
        {
            name:    "invalid input",
            input:   "",
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("MyFunction() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("MyFunction() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

**Using Test Utilities:**

```go
// Setup temporary XDG directories
cleanup := testutil.SetupXDGDirs(t)
defer cleanup()

// Seed test agents
testutil.SeedFixtureAgents(t, "testdata/agents", xdg.GetConfigDir())

// Create mock LLM server
server := testutil.NewMockLLMServer(t, testutil.AnthropicResponse("test response"))
defer server.Close()
```

**Avoid Mocking When Possible:**
Prefer using real implementations or test utilities over mocks. Use `internal/testutil/mockserver.go` for provider testing.

## Submitting Changes

### Before Submitting

1. **Run tests:** `go test ./...`
2. **Run linter:** `golangci-lint run`
3. **Update documentation** if you changed behavior
4. **Update CHANGELOG.md** with your changes
5. **Ensure commits are clean** and have descriptive messages

### Commit Messages

Follow conventional commit format:

```text
type(scope): brief description

Longer explanation if needed.

Fixes #123
```

**Types:**
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation changes
- `test` - Test additions or changes
- `refactor` - Code refactoring
- `chore` - Maintenance tasks

**Examples:**
```
feat(tool): add url_fetch tool with HTML stripping

fix(memory): prevent race condition in concurrent writes

docs(readme): update installation instructions

test(provider): add OpenAI error handling tests
```

### Pull Request Process

1. **Create a feature branch:**
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make your changes** and commit them

3. **Push to your fork:**
   ```bash
   git push origin feature/my-feature
   ```

4. **Open a Pull Request** on GitHub

5. **Respond to review feedback** - maintainers may request changes

6. **Once approved**, your PR will be merged

### Pull Request Guidelines

- **One feature per PR** - Keep changes focused
- **Include tests** for new functionality
- **Update documentation** as needed
- **Ensure CI passes** - all tests and linting must pass
- **Write a clear description** - explain what and why

## Project Structure

```
axe/
├── cmd/                    # CLI commands
│   ├── run.go             # Agent execution
│   ├── agents.go          # Agent management
│   ├── config.go          # Configuration
│   ├── gc.go              # Garbage collection
│   └── testdata/          # Test fixtures
├── internal/
│   ├── agent/             # Agent configuration
│   ├── config/            # Global configuration
│   ├── envinterp/         # Environment variable expansion
│   ├── mcpclient/         # MCP client
│   ├── memory/            # Persistent memory
│   ├── provider/          # LLM providers
│   ├── refusal/           # Refusal detection
│   ├── resolve/           # Context resolution
│   ├── testutil/          # Test utilities
│   ├── tool/              # Built-in tools
│   ├── toolname/          # Tool constants
│   └── xdg/               # XDG directories
├── docs/
│   ├── design/            # Design documents
│   └── plans/             # Implementation plans
├── examples/              # Example agents
└── skills/                # Embedded skills
```

### Key Files

- `main.go` - Application entry point
- `cmd/root.go` - Root command and error handling
- `internal/provider/provider.go` - Provider interface
- `internal/tool/registry.go` - Tool registry
- `internal/agent/agent.go` - Agent configuration

## Design Principles

### Unix Philosophy

- **Do one thing well** - Each agent is single-purpose
- **Compose with standard tools** - Pipes, cron, git hooks
- **Clean stdout** - Output is safe to pipe
- **Meaningful exit codes** - 0 for success, 1 for runtime error, 2 for config error

### Context Minimization

- **Small context windows** - Each agent gets only what it needs
- **Opaque sub-agents** - Parents only see final results, not internals
- **Focused skills** - SKILL.md files provide targeted instructions

### Configuration Over Code

- **TOML-based** - Agent definitions are declarative
- **No source changes** - Users never touch Go code to create agents
- **Version controllable** - Configurations can be committed to git

### Testability

- **Table-driven tests** - Consistent test structure
- **Minimal mocking** - Prefer real implementations
- **Integration tests** - Test end-to-end behavior
- **Golden files** - Capture expected CLI output

### Security

- **Path sandboxing** - File operations restricted to working directory
- **Symlink validation** - Prevent escaping working directory
- **No arbitrary execution** - Except explicit `run_command` tool

## Adding New Features

### Adding a New Tool

1. Create `internal/tool/my_tool.go`
2. Define tool entry function returning `Definition` and `Executor`
3. Implement executor with `ExecuteContext` and arguments
4. Add constant to `internal/toolname/toolname.go`
5. Register in `internal/tool/registry.go` `RegisterAll()`
6. Add to `ValidNames()` in `internal/toolname/toolname.go`
7. Write tests in `internal/tool/my_tool_test.go`
8. Add integration test in `cmd/run_integration_test.go`
9. Update documentation

### Adding a New Provider

1. Create `internal/provider/myprovider.go`
2. Implement `Provider` interface with `Send()` method
3. Implement request/response conversion
4. Implement error handling and categorization
5. Add constructor (e.g., `NewMyProvider()`)
6. Register in `internal/provider/registry.go` `New()`
7. Add to `Supported()` function
8. Write tests in `internal/provider/myprovider_test.go`
9. Add integration test in `cmd/run_integration_test.go`
10. Update documentation

### Adding a New Command

1. Create `cmd/mycommand.go`
2. Define Cobra command with flags
3. Implement command logic
4. Add to root command in `init()` function
5. Write tests in `cmd/mycommand_test.go`
6. Add smoke test in `cmd/smoke_test.go`
7. Create golden files if applicable
8. Update documentation

## Getting Help

- **Issues** - Open an issue on GitHub for bugs or feature requests
- **Discussions** - Use GitHub Discussions for questions
- **Documentation** - Check `docs/` directory for design documents

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- Assume good intentions

## License

By contributing to Axe, you agree that your contributions will be licensed under the Apache-2.0 License.

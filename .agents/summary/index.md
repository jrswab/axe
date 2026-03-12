# Axe Codebase Knowledge Base Index

## Purpose

This index serves as the primary entry point for AI assistants working with the Axe codebase. It provides metadata about each documentation file to help determine which files contain relevant information for specific questions.

## How to Use This Documentation

**For AI Assistants:**
1. Start by reading this index file to understand the documentation structure
2. Identify which documentation files are relevant to your task based on the summaries below
3. Load only the specific files you need rather than all documentation
4. Use the cross-references to navigate between related topics

**For Humans:**
- This documentation was auto-generated from the codebase
- Each file focuses on a specific aspect of the system
- Start with `codebase_info.md` for a high-level overview
- Refer to `architecture.md` for system design understanding

## Documentation Files

### codebase_info.md
**Purpose:** High-level project overview and basic information  
**Contains:**
- Project name, repository, language, and license
- Technology stack and dependencies
- Directory structure overview
- Key features list
- Build and test commands
- Configuration locations

**When to consult:** Getting started, understanding project basics, finding where things are located

### architecture.md
**Purpose:** System architecture and design patterns  
**Contains:**
- Architectural diagrams (Mermaid)
- Core layer descriptions (CLI, Agent, Provider, Tool, Memory, MCP)
- Data flow diagrams
- Design principles (Unix philosophy, context minimization, configuration over code)
- Extension points for adding new features

**When to consult:** Understanding system design, adding new components, architectural decisions, understanding how pieces fit together

### components.md
**Purpose:** Detailed component and package documentation  
**Contains:**
- CLI command descriptions and responsibilities
- Internal package documentation with key functions
- Tool implementations and features
- Test organization and fixtures
- Component relationships

**When to consult:** Finding specific functions, understanding what a package does, locating implementation details, writing tests

### interfaces.md
**Purpose:** API contracts and interfaces  
**Contains:**
- Provider interface and data structures
- Tool interface and registry
- MCP client interface
- Agent configuration schema (TOML and Go)
- CLI interface and flags
- JSON output format
- Environment variables
- File system conventions

**When to consult:** Understanding contracts between components, implementing new providers/tools, configuration format, CLI usage, integration points

### data_models.md
**Purpose:** Data structures and type definitions  
**Contains:**
- Agent configuration models
- Provider request/response models
- Tool models and execution context
- Memory models and operations
- Configuration models
- MCP models
- CLI output models
- Validation rules
- Constants and limits
- Type conversions

**When to consult:** Understanding data structures, validation logic, type conversions, constants and limits, model relationships

### workflows.md
**Purpose:** Process flows and operational procedures  
**Contains:**
- Agent execution workflow (sequence diagrams)
- Sub-agent delegation workflow
- Memory lifecycle workflow
- Garbage collection workflow
- Tool execution workflow
- Configuration resolution workflow
- MCP server connection workflow
- Error handling workflow
- Development workflows (adding tools, providers, commands)
- Release workflow

**When to consult:** Understanding how processes work end-to-end, debugging execution flow, adding new features, release procedures

### dependencies.md
**Purpose:** External dependencies and their usage  
**Contains:**
- Direct dependencies (cobra, toml, go-sdk, golang.org/x/net)
- Indirect dependencies
- Standard library usage
- External service dependencies (LLM APIs, web search)
- Build dependencies
- Docker dependencies
- Dependency management strategy
- Dependency graph
- License compatibility

**When to consult:** Understanding what libraries are used, why dependencies were chosen, updating dependencies, license compliance

## Quick Reference by Task

### Understanding the Codebase
1. Start with `codebase_info.md` for overview
2. Read `architecture.md` for design understanding
3. Consult `components.md` for specific packages

### Implementing New Features
1. Check `architecture.md` for extension points
2. Review `interfaces.md` for contracts to implement
3. Follow `workflows.md` for development procedures
4. Reference `data_models.md` for data structures

### Debugging Issues
1. Check `workflows.md` for execution flow
2. Review `components.md` for implementation details
3. Consult `interfaces.md` for expected behavior
4. Check `data_models.md` for validation rules

### Configuration and Setup
1. Read `codebase_info.md` for basic setup
2. Check `interfaces.md` for configuration schema
3. Review `dependencies.md` for external requirements

### Adding Dependencies
1. Review `dependencies.md` for current dependencies
2. Check dependency management strategy
3. Ensure license compatibility

## Cross-References

- **Agent Configuration:** interfaces.md (schema) → data_models.md (structures) → components.md (loading/validation)
- **Tool System:** architecture.md (design) → interfaces.md (contracts) → components.md (implementations) → workflows.md (execution)
- **Provider Integration:** architecture.md (layer) → interfaces.md (interface) → components.md (implementations) → dependencies.md (external APIs)
- **Memory System:** architecture.md (design) → components.md (implementation) → workflows.md (lifecycle) → data_models.md (format)
- **MCP Integration:** components.md (client) → interfaces.md (protocol) → workflows.md (connection) → dependencies.md (SDK)

## Metadata

**Generated:** 2026-03-12T09:32:56-05:00  
**Codebase Path:** /Users/cmorton/repos/axe  
**Total Files Analyzed:** 223  
**Lines of Code:** 28,818  
**Documentation Files:** 7

## Notes for AI Assistants

- This documentation is comprehensive but may not cover every edge case
- When in doubt, examine the actual source code
- Test files (*_test.go) often provide excellent usage examples
- The codebase follows Go conventions and Unix philosophy
- Design docs in `docs/design/` and plans in `docs/plans/` provide additional context
- The existing AGENTS.md file contains project-specific conventions and constraints

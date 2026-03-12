# Workflows

## Agent Execution Workflow

```mermaid
sequenceDiagram
    participant User
    participant CLI
    participant Agent
    participant Resolve
    participant Memory
    participant Provider
    participant Tool
    
    User->>CLI: axe run my-agent [flags]
    CLI->>Agent: Load agent TOML
    Agent->>Agent: Validate configuration
    CLI->>Resolve: Resolve workdir
    CLI->>Resolve: Resolve skill file
    CLI->>Resolve: Resolve context files
    CLI->>Resolve: Read stdin (if piped)
    CLI->>Resolve: Build system prompt
    
    alt Memory enabled
        CLI->>Memory: Load last N entries
        Memory-->>CLI: Historical context
    end
    
    CLI->>Provider: Send initial request
    Provider-->>CLI: Response with tool calls
    
    loop Conversation (max 50 turns)
        alt Has tool calls
            CLI->>Tool: Execute tools (parallel/sequential)
            Tool-->>CLI: Tool results
            CLI->>Provider: Send tool results
            Provider-->>CLI: Next response
        else Text response
            Note over CLI: Exit loop
        end
    end
    
    alt Memory enabled and success
        CLI->>Memory: Append entry
    end
    
    CLI-->>User: Output (text or JSON)
```

## Sub-Agent Delegation Workflow

```mermaid
sequenceDiagram
    participant Parent
    participant Tool
    participant SubAgent
    participant Provider
    
    Parent->>Tool: call_agent(agent_name, task)
    Tool->>Tool: Check depth limit
    Tool->>Tool: Verify agent allowed
    Tool->>SubAgent: Load configuration
    SubAgent->>SubAgent: Increment depth
    SubAgent->>SubAgent: Remove call_agent tool
    SubAgent->>Provider: Execute task
    
    loop Sub-agent conversation
        Provider-->>SubAgent: Response
        alt Has tool calls
            SubAgent->>SubAgent: Execute tools
        end
    end
    
    Provider-->>SubAgent: Final response
    SubAgent-->>Tool: Text result only
    Tool-->>Parent: Tool result
    
    Note over Parent,SubAgent: Sub-agent internals hidden from parent
```

## Memory Lifecycle Workflow

```mermaid
flowchart TD
    Start[Agent Run Starts]
    LoadMem{Memory Enabled?}
    LoadEntries[Load Last N Entries]
    BuildPrompt[Build System Prompt]
    Execute[Execute Agent]
    Success{Execution Success?}
    AppendEntry[Append Task/Result Entry]
    CheckCount[Count Total Entries]
    ExceedsMax{Exceeds max_entries?}
    WarnUser[Warn: Consider GC]
    End[Agent Run Ends]
    
    Start --> LoadMem
    LoadMem -->|Yes| LoadEntries
    LoadMem -->|No| BuildPrompt
    LoadEntries --> BuildPrompt
    BuildPrompt --> Execute
    Execute --> Success
    Success -->|Yes| AppendEntry
    Success -->|No| End
    AppendEntry --> CheckCount
    CheckCount --> ExceedsMax
    ExceedsMax -->|Yes| WarnUser
    ExceedsMax -->|No| End
    WarnUser --> End
```

## Garbage Collection Workflow

```mermaid
flowchart TD
    Start[axe gc agent]
    CheckMem{Memory Enabled?}
    NoMem[Error: Memory disabled]
    FileExists{Memory File Exists?}
    NoFile[Info: No memory file]
    CountEntries[Count Entries]
    WithinLimit{Within max_entries?}
    NoTrim[Info: Nothing to trim]
    
    AnalyzePrompt[Build Analysis Prompt]
    CallLLM[Call LLM for Pattern Detection]
    LLMSuccess{LLM Success?}
    ParseResponse[Parse Keep Count]
    ValidCount{Valid Count?}
    FallbackMax[Fallback: Use max_entries]
    
    DryRun{--dry-run?}
    ShowPlan[Show Trim Plan]
    TrimEntries[Trim to Keep Count]
    Success[Success: Trimmed N entries]
    
    Start --> CheckMem
    CheckMem -->|No| NoMem
    CheckMem -->|Yes| FileExists
    FileExists -->|No| NoFile
    FileExists -->|Yes| CountEntries
    CountEntries --> WithinLimit
    WithinLimit -->|Yes| NoTrim
    WithinLimit -->|No| AnalyzePrompt
    
    AnalyzePrompt --> CallLLM
    CallLLM --> LLMSuccess
    LLMSuccess -->|No| FallbackMax
    LLMSuccess -->|Yes| ParseResponse
    ParseResponse --> ValidCount
    ValidCount -->|No| FallbackMax
    ValidCount -->|Yes| DryRun
    FallbackMax --> DryRun
    
    DryRun -->|Yes| ShowPlan
    DryRun -->|No| TrimEntries
    TrimEntries --> Success
```

## Tool Execution Workflow

```mermaid
flowchart TD
    Start[Receive Tool Calls]
    Parallel{Parallel Mode?}
    
    SeqLoop[For Each Tool Call]
    SeqDispatch[Dispatch Tool]
    SeqBuiltin{Built-in Tool?}
    SeqRegistry[Execute via Registry]
    SeqMCP[Execute via MCP Router]
    SeqResult[Collect Result]
    SeqNext{More Calls?}
    
    ParLoop[For Each Tool Call - Goroutine]
    ParDispatch[Dispatch Tool]
    ParBuiltin{Built-in Tool?}
    ParRegistry[Execute via Registry]
    ParMCP[Execute via MCP Router]
    ParResult[Collect Result]
    ParWait[Wait All Goroutines]
    
    Combine[Combine Results]
    Return[Return to LLM]
    
    Start --> Parallel
    
    Parallel -->|No| SeqLoop
    SeqLoop --> SeqDispatch
    SeqDispatch --> SeqBuiltin
    SeqBuiltin -->|Yes| SeqRegistry
    SeqBuiltin -->|No| SeqMCP
    SeqRegistry --> SeqResult
    SeqMCP --> SeqResult
    SeqResult --> SeqNext
    SeqNext -->|Yes| SeqLoop
    SeqNext -->|No| Combine
    
    Parallel -->|Yes| ParLoop
    ParLoop --> ParDispatch
    ParDispatch --> ParBuiltin
    ParBuiltin -->|Yes| ParRegistry
    ParBuiltin -->|No| ParMCP
    ParRegistry --> ParResult
    ParMCP --> ParResult
    ParResult --> ParWait
    ParWait --> Combine
    
    Combine --> Return
```

## Configuration Resolution Workflow

```mermaid
flowchart TD
    Start[Load Agent TOML]
    
    Model[Resolve Model]
    ModelFlag{--model flag?}
    ModelTOML[Use TOML model]
    ModelFinal[Final: model]
    
    Workdir[Resolve Workdir]
    WorkdirFlag{--workdir flag?}
    WorkdirTOML{TOML workdir?}
    WorkdirCWD[Use CWD]
    WorkdirExpand[Expand ~ and $VAR]
    WorkdirFinal[Final: workdir]
    
    Skill[Resolve Skill]
    SkillFlag{--skill flag?}
    SkillTOML[Use TOML skill]
    SkillResolve[Resolve Path]
    SkillFinal[Final: skill content]
    
    Files[Resolve Files]
    FilesGlob[Match Glob Patterns]
    FilesFilter[Filter Binary Files]
    FilesSort[Sort & Deduplicate]
    FilesFinal[Final: file contents]
    
    Stdin[Resolve Stdin]
    StdinPiped{Stdin Piped?}
    StdinRead[Read Stdin]
    StdinSkip[Skip]
    StdinFinal[Final: stdin content]
    
    Combine[Build System Prompt]
    Done[Context Ready]
    
    Start --> Model
    Model --> ModelFlag
    ModelFlag -->|Yes| ModelFinal
    ModelFlag -->|No| ModelTOML
    ModelTOML --> ModelFinal
    
    ModelFinal --> Workdir
    Workdir --> WorkdirFlag
    WorkdirFlag -->|Yes| WorkdirExpand
    WorkdirFlag -->|No| WorkdirTOML
    WorkdirTOML -->|Yes| WorkdirExpand
    WorkdirTOML -->|No| WorkdirCWD
    WorkdirCWD --> WorkdirExpand
    WorkdirExpand --> WorkdirFinal
    
    WorkdirFinal --> Skill
    Skill --> SkillFlag
    SkillFlag -->|Yes| SkillResolve
    SkillFlag -->|No| SkillTOML
    SkillTOML --> SkillResolve
    SkillResolve --> SkillFinal
    
    SkillFinal --> Files
    Files --> FilesGlob
    FilesGlob --> FilesFilter
    FilesFilter --> FilesSort
    FilesSort --> FilesFinal
    
    FilesFinal --> Stdin
    Stdin --> StdinPiped
    StdinPiped -->|Yes| StdinRead
    StdinPiped -->|No| StdinSkip
    StdinRead --> StdinFinal
    StdinSkip --> StdinFinal
    
    StdinFinal --> Combine
    Combine --> Done
```

## MCP Server Connection Workflow

```mermaid
flowchart TD
    Start[Load Agent Config]
    HasMCP{Has MCP Servers?}
    Skip[Skip MCP Setup]
    
    Loop[For Each MCP Server]
    CheckTransport{Transport Type?}
    
    Stdio[Stdio Transport]
    StdioCmd[Start Command Process]
    StdioConnect[Connect via Stdin/Stdout]
    
    HTTP[HTTP/HTTPS Transport]
    HTTPExpand[Expand ${VAR} in Headers]
    HTTPConnect[Create HTTP Client]
    
    ListTools[List Available Tools]
    RegisterTools[Register in Router]
    
    Next{More Servers?}
    Ready[MCP Ready]
    
    Start --> HasMCP
    HasMCP -->|No| Skip
    HasMCP -->|Yes| Loop
    
    Loop --> CheckTransport
    CheckTransport -->|stdio| Stdio
    CheckTransport -->|http/https| HTTP
    
    Stdio --> StdioCmd
    StdioCmd --> StdioConnect
    StdioConnect --> ListTools
    
    HTTP --> HTTPExpand
    HTTPExpand --> HTTPConnect
    HTTPConnect --> ListTools
    
    ListTools --> RegisterTools
    RegisterTools --> Next
    Next -->|Yes| Loop
    Next -->|No| Ready
```

## Error Handling Workflow

```mermaid
flowchart TD
    Start[Error Occurs]
    Type{Error Type?}
    
    Provider[Provider Error]
    Category{Error Category?}
    Auth[Auth Error]
    Rate[Rate Limit]
    Server[Server Error]
    Input[Input Error]
    Network[Network Error]
    Unknown[Unknown Error]
    
    Tool[Tool Error]
    ToolResult[Return as Tool Result]
    ToolContinue[LLM Continues]
    
    Config[Config Error]
    ConfigExit[Exit Code 2]
    
    Runtime[Runtime Error]
    RuntimeExit[Exit Code 1]
    
    Start --> Type
    
    Type -->|Provider| Provider
    Provider --> Category
    Category -->|Auth| Auth
    Category -->|Rate Limit| Rate
    Category -->|Server| Server
    Category -->|Input| Input
    Category -->|Network| Network
    Category -->|Unknown| Unknown
    
    Auth --> RuntimeExit
    Rate --> RuntimeExit
    Server --> RuntimeExit
    Input --> RuntimeExit
    Network --> RuntimeExit
    Unknown --> RuntimeExit
    
    Type -->|Tool| Tool
    Tool --> ToolResult
    ToolResult --> ToolContinue
    
    Type -->|Config| Config
    Config --> ConfigExit
    
    Type -->|Runtime| Runtime
    Runtime --> RuntimeExit
```

## Development Workflow

### Adding a New Tool

1. Create tool file in `internal/tool/`
2. Define tool entry function returning `Definition` and `Executor`
3. Implement executor function with `ExecuteContext` and arguments
4. Add constant to `internal/toolname/toolname.go`
5. Register in `internal/tool/registry.go` `RegisterAll()`
6. Add validation to `internal/toolname/ValidNames()`
7. Write unit tests in `*_test.go`
8. Add integration test in `cmd/run_integration_test.go`
9. Update documentation

### Adding a New Provider

1. Create provider file in `internal/provider/`
2. Implement `Provider` interface with `Send()` method
3. Implement request/response conversion functions
4. Implement error handling and categorization
5. Add constructor function (e.g., `NewProvider()`)
6. Register in `internal/provider/registry.go` `New()`
7. Add to `Supported()` function
8. Write unit tests with mock HTTP server
9. Add integration test in `cmd/run_integration_test.go`
10. Update documentation

### Adding a New Command

1. Create command file in `cmd/`
2. Define Cobra command with flags
3. Implement command logic
4. Add to root command in `init()` function
5. Write unit tests
6. Add smoke test in `cmd/smoke_test.go`
7. Create golden files if applicable
8. Update CLI documentation

### Release Workflow

1. Update `CHANGELOG.md` with changes
2. Update version in `cmd/version.go`
3. Commit changes
4. Create git tag: `git tag v1.x.x`
5. Push tag: `git push origin v1.x.x`
6. GitHub Actions runs GoReleaser
7. Binaries published to GitHub Releases
8. Docker images built and pushed (if configured)

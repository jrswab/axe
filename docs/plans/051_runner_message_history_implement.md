# Implement Runner Pre-Built Message History (#82)

## Section 1: Context Summary

Issue #81 extracted the agent execution engine into `pkg/runner` as a public API. Today, `runner.Options` only accepts `Prompt` (string) or `Stdin` (reader), which `Run()` converts into a single `provider.Message`. Callers who manage their own conversation state cannot seed `Run()` with prior turns, nor can they retrieve the updated history after execution. This implementation adds an opt-in `Messages []Message` field to `Options` and returns the full history in `Result.Messages`. The change is purely additive — existing `Prompt`/`Stdin` behavior is preserved when `Messages` is empty. Because `internal/provider` types cannot be exposed in `pkg/runner`'s public API, new self-contained `Message`, `ToolCall`, and `ToolResult` structs are defined in `pkg/runner` with conversion helpers to `provider` types.

---

## Section 2: Implementation Checklist

### Phase 1: Public Types (parallelizable)

- [x] **`pkg/runner/messages.go` — Define public types**
  Define `Message`, `ToolCall`, `ToolResult` structs isomorphic to `provider.Message`, `provider.ToolCall`, `provider.ToolResult`:
  ```go
  type Message struct {
      Role        string
      Content     string
      ToolCalls   []ToolCall
      ToolResults []ToolResult
  }
  type ToolCall struct { ID string; Name string; Arguments map[string]string }
  type ToolResult struct { CallID string; Content string; IsError bool }
  ```

- [x] **`pkg/runner/messages.go` — Add conversion functions**
  - `toProviderMessages([]Message) []provider.Message`
  - `fromProviderMessage(provider.Message) Message`
  These are the only places that bridge `pkg/runner` ↔ `internal/provider`.

- [x] **`pkg/runner/messages.go` — Add validation function**
  - `validateMessages([]Message) error` — returns `*ConfigError` on:
    - invalid `Role` (not `"user"`, `"assistant"`, `"tool"`)
    - `assistant` role with non-empty `ToolResults`
    - `tool` role with non-empty `Content` or non-empty `ToolCalls`
    - `user` role with non-empty `ToolCalls` or non-empty `ToolResults`
    - `ToolResult.CallID` not matching any `ToolCall.ID` from the most recent preceding `assistant` message
  Include the offending index in every error message.

- [x] **`pkg/runner/messages_test.go` — Test validation and conversion**
  Test `validateMessages` with valid histories, invalid roles, wrong field combinations, and dangling CallIDs. Test `toProviderMessages`/`fromProviderMessage` round-trip equality.

### Phase 2: Struct Updates (parallelizable with Phase 1)

- [x] **`pkg/runner/options.go` — Add Messages field**
  Add `Messages []Message` to `Options`. Zero-value behavior remains unchanged.

- [x] **`pkg/runner/result.go` — Add Messages field and JSON serialization**
  - Add `Messages []Message` to `Result` struct.
  - Update `Result.MarshalJSON()`: include `"messages"` key as an array of message objects. Empty `tool_calls`/`tool_results` must serialize as `[]`, not `null` or omitted.

- [x] **`pkg/runner/result_test.go` — Test JSON with Messages**
  Verify `MarshalJSON` produces `"messages"` array with correct shape when `Result.Messages` is populated. Verify existing JSON output is unchanged when `Messages` is empty.

### Phase 3: Core Integration (depends on Phase 1 + 2)

- [x] **`pkg/runner/run.go` — Skip prompt/stdin when Messages is set**
  Around line ~220 (after stdin read), change the user message resolution logic:
  ```go
  if len(opts.Messages) > 0 {
      // skip Prompt/Stdin/default resolution
  } else {
      // existing resolution: Prompt → Stdin → defaultUserMessage
  }
  ```

- [x] **`pkg/runner/run.go` — Validate pre-built messages before provider call**
  After agent loading and overrides (around line ~340, before `req` construction), add:
  ```go
  if len(opts.Messages) > 0 {
      if err := validateMessages(opts.Messages); err != nil {
          return nil, err // *ConfigError
      }
  }
  ```

- [x] **`pkg/runner/run.go` — Initialize req.Messages from opts.Messages`**
  Around line ~344, change `req.Messages` initialization:
  ```go
  var messages []provider.Message
  if len(opts.Messages) > 0 {
      messages = toProviderMessages(opts.Messages)
  } else {
      messages = []provider.Message{{Role: "user", Content: userMessage}}
  }
  req := &provider.Request{ ... Messages: messages ... }
  ```

- [x] **`pkg/runner/run.go` — Update verbose prompt source`**
  Around line ~435, update the `promptSource` logic:
  ```go
  promptSource := "default"
  if len(opts.Messages) > 0 {
      promptSource = fmt.Sprintf("%d pre-built messages", len(opts.Messages))
  } else if strings.TrimSpace(opts.Prompt) != "" {
      promptSource = "flag"
  } else if strings.TrimSpace(stdinContent) != "" {
      promptSource = "stdin"
  }
  ```

- [x] **`pkg/runner/run.go` — Populate Result.Messages at end of Run()`**
  Around line ~660 (after `result := &Result{...}`), add:
  ```go
  result.Messages = make([]Message, len(req.Messages))
  for i, m := range req.Messages {
      result.Messages[i] = fromProviderMessage(m)
  }
  ```
  This must run for both the single-shot and conversation-loop paths.

- [x] **`pkg/runner/run.go` — Update memory append logic`**
  Around line ~707, change the `memory.AppendEntry` call to use the first user message from history:
  ```go
  taskText := userMessage // from existing resolution
  if len(opts.Messages) > 0 {
      taskText = firstUserMessageContent(opts.Messages)
      if taskText == "" {
          taskText = "(pre-built message history with no user message)"
      }
  }
  // then: memory.AppendEntry(appendPath, taskText, resp.Content)
  ```
  Extract `firstUserMessageContent` either as a helper in `messages.go` or inline.

### Phase 4: Dry-Run + CLI (parallelizable with Phase 3)

- [x] **`pkg/runner/result.go` — Add MessageCount to DryRunInfo`**
  Add `MessageCount int` to `DryRunInfo`.

- [x] **`pkg/runner/run.go` — Populate MessageCount in dry-run path`**
  Around line ~290 (inside the `opts.DryRun` block), set `DryRunInfo.MessageCount = len(opts.Messages)` and `DryRunInfo.UserMessage = "(pre-built message history)"` when `len(opts.Messages) > 0`.

- [x] **`cmd/run.go` — Update printDryRun for pre-built messages`**
  In `printDryRun()`, when `info.MessageCount > 0`, print:
  ```
  --- User Message ---
  (pre-built message history, N messages)
  ```
  instead of the raw `UserMessage` string.

### Phase 5: Integration Tests

- [x] **`pkg/runner/run_test.go` — Test single-shot with pre-built messages`**
  Create a test that passes `Options.Messages` with a single user message, runs against the mock server, and asserts `Result.Messages` contains both the user message and the assistant response.

- [x] **`pkg/runner/run_test.go` — Test tool loop with pre-built messages`**
  Pass a history containing a prior assistant message with a tool call and a tool result message. Run an agent with tools. Assert the conversation loop appends new assistant/tool messages to the existing history and that `Result.Messages` contains the complete history.

- [x] **`pkg/runner/run_test.go` — Test validation errors propagate as ConfigError`**
  Pass invalid `Options.Messages` (invalid role, dangling CallID). Assert `Run()` returns a `ConfigError` before any provider call.

- [x] **`pkg/runner/run_test.go` — Test empty (non-nil) Messages falls back to Prompt`**
  Verify `Messages: []Message{}` behaves identically to `Messages: nil`, i.e. uses `Prompt`/`Stdin`.

- [x] **Run existing test suites**
  `go test ./pkg/runner/... ./cmd/...` must pass with zero modifications to existing tests.

---

## Parallelization Guide

| Phase | Can Parallelize With |
|-------|---------------------|
| Phase 1 (types + validation) | Phase 2 (struct updates) |
| Phase 2 (struct updates) | Phase 1 (types + validation) |
| Phase 3 (run.go integration) | Blocked on Phase 1 + 2 |
| Phase 4 (dry-run + CLI) | Blocked on Phase 2, can parallelize with Phase 3 |
| Phase 5 (tests) | Blocked on Phase 3 + 4 |

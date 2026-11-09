package runner

import "context"

// ToolAction specifies how the runner handles a tool call.
type ToolAction string

const (
	// ToolAllow dispatches the tool call normally.
	ToolAllow ToolAction = "allow"
	// ToolDeny returns an error result without running the tool.
	ToolDeny ToolAction = "deny"
	// ToolStub substitutes a normal result without running the tool.
	ToolStub ToolAction = "stub"
)

// ToolDecision is the host's decision for a tool call.
// ToolDeny uses Content when non-empty, otherwise the fixed text
// "tool call denied by host policy". ToolStub uses Content as-is, including
// an empty string. ToolAllow ignores Content. An unknown Action (including
// "") is denied with the fixed text, and Content is ignored.
type ToolDecision struct {
	Action  ToolAction
	Content string
}

// ToolHook decides whether a tool call is allowed, denied, or stubbed.
// Calls are sequential on the run goroutine, in model order, before any
// dispatch of that turn. The call's Arguments map is a fresh copy, so hook
// mutations do not change the dispatched arguments.
//
// Concurrent Run calls sharing one hook may call it concurrently; the host
// must make the hook safe for concurrent use. A deny differs from an ordinary
// tool error because the tool never runs.
//
// A hook error or panic denies the call with the fixed text
// "tool call denied by host policy"; raw error or panic text is not returned.
type ToolHook func(ctx context.Context, call ToolCall) (ToolDecision, error)

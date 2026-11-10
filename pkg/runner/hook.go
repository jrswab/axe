package runner

import (
	"context"
	"errors"
	"time"
)

const toolDenyText = "tool call denied by host policy"

var errHookPanic = errors.New("tool hook panicked")

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

// ToolEvent contains metadata about a completed tool call, never result text.
// Action is the effective action the runner applied, not the raw hook output.
// ContentLength is the untruncated byte length of the result before truncation.
// Duration measures dispatch only and is zero for calls that were not dispatched.
// Turn is zero-based. Call.Arguments is a fresh copy; mutations do not change history.
type ToolEvent struct {
	Call          ToolCall
	Action        ToolAction
	CallID        string
	IsError       bool
	ContentLength int
	Duration      time.Duration
	Turn          int
}

// ToolObserver receives metadata only, never result text, and returns nothing.
// Calls are sequential on the run goroutine, in model order, after the turn's
// tool calls finish. Concurrent Run calls sharing one observer may call it
// concurrently; the host must make the observer safe for concurrent use.
// No cross-run ordering is guaranteed. A panic is recovered and reported to
// stderr as the fixed line "Warning: tool observer panicked"; delivery continues.
type ToolObserver func(ctx context.Context, ev ToolEvent)

func callToolHook(ctx context.Context, hook ToolHook, call ToolCall) (decision ToolDecision, err error) {
	args := make(map[string]string, len(call.Arguments))
	for key, value := range call.Arguments {
		args[key] = value
	}
	call.Arguments = args

	defer func() {
		if r := recover(); r != nil {
			err = errHookPanic
		}
	}()
	return hook(ctx, call)
}

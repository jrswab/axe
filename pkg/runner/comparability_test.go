package runner

// These compile-time assertions preserve the v1 comparability contract when
// fields are added. They do not guarantee stable equality results.
var (
	_ comparableGuard[ConfigError]
	_ comparableGuard[RuntimeError]
	_ comparableGuard[BudgetExceededError]
	_ comparableGuard[ArtifactInfo]
	_ comparableGuard[BudgetState]
	_ comparableGuard[ToolResult]
	_ comparableGuard[ToolDecision]
)

type comparableGuard[T comparable] map[T]struct{}

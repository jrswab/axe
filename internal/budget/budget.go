package budget

import "sync"

// BudgetTracker tracks cumulative token usage against an optional maximum.
// It is safe for concurrent use.
type BudgetTracker struct {
	mu         sync.Mutex
	maxTokens  int
	usedTokens int
}

// New creates a BudgetTracker with the given maximum token limit.
// A maxTokens of 0 means unlimited (no budget enforcement).
func New(maxTokens int) *BudgetTracker {
	return &BudgetTracker{maxTokens: maxTokens}
}

// Add records token usage. Always accumulates tokens even when maxTokens is 0.
func (b *BudgetTracker) Add(input, output int) {
	b.mu.Lock()
	b.usedTokens += input + output
	b.mu.Unlock()
}

// Exceeded reports whether the budget has been met or exceeded.
// Returns false when maxTokens is 0 (unlimited).
func (b *BudgetTracker) Exceeded() bool {
	if b.maxTokens == 0 {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.usedTokens >= b.maxTokens
}

// Used returns the total tokens consumed so far.
func (b *BudgetTracker) Used() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.usedTokens
}

// Max returns the maximum token budget. 0 means unlimited.
func (b *BudgetTracker) Max() int {
	return b.maxTokens
}

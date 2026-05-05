package runner

import (
	"fmt"

	"github.com/jrswab/axe/internal/provider"
)

// ConfigError indicates a configuration problem (missing agent, invalid TOML,
// missing API key, etc.). CLI maps this to exit code 2.
type ConfigError struct {
	Msg string
	Err error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// RuntimeError indicates a runtime failure (provider call failure, tool
// execution failure, JSON marshal failure, etc.). CLI maps this to exit code 1.
type RuntimeError struct {
	Msg string
	Err error
}

func (e *RuntimeError) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

func (e *RuntimeError) Unwrap() error {
	return e.Err
}

// BudgetExceededError indicates the token budget was exceeded.
// CLI maps this to exit code 4.
type BudgetExceededError struct {
	Used int
	Max  int
}

func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("budget exceeded: used %d of %d tokens; increase --max-tokens or reduce prompt/tool output", e.Used, e.Max)
}

// IsConfigError returns true if err is a *ConfigError.
func IsConfigError(err error) bool {
	_, ok := err.(*ConfigError)
	return ok
}

// IsRuntimeError returns true if err is a *RuntimeError.
func IsRuntimeError(err error) bool {
	_, ok := err.(*RuntimeError)
	return ok
}

// IsBudgetExceededError returns true if err is a *BudgetExceededError.
func IsBudgetExceededError(err error) bool {
	_, ok := err.(*BudgetExceededError)
	return ok
}

// ProviderCategory returns the provider error category if err wraps a
// *provider.ProviderError, and a boolean indicating whether one was found.
// This lets the CLI map to exit code 3 for auth/rate-limit/timeout/server errors.
func ProviderCategory(err error) (provider.ErrorCategory, bool) {
	var provErr *provider.ProviderError
	if AsProviderError(err, &provErr) {
		return provErr.Category, true
	}
	return "", false
}

// AsProviderError unwraps errors to find a *provider.ProviderError.
func AsProviderError(err error, target **provider.ProviderError) bool {
	if target == nil || err == nil {
		return false
	}
	// Walk the chain
	for err != nil {
		if pe, ok := err.(*provider.ProviderError); ok {
			*target = pe
			return true
		}
		// Try unwrapping
		type unwrapper interface {
			Unwrap() error
		}
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
		} else {
			break
		}
	}
	return false
}

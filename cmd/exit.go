package cmd

import "fmt"

// ExitError wraps an error with a specific process exit code.
type ExitError struct {
	Code int
	Err  error
}

// Error delegates to the wrapped error's Error method.
// If Err is nil, it returns a string containing only the exit code.
func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit code %d", e.Code)
	}
	return e.Err.Error()
}

// Unwrap returns the wrapped error, supporting errors.Is and errors.As.
func (e *ExitError) Unwrap() error {
	return e.Err
}

package cmd

// ExitError wraps an error with a specific process exit code.
type ExitError struct {
	Code int
	Err  error
}

// Error delegates to the wrapped error's Error method.
func (e *ExitError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the wrapped error, supporting errors.Is and errors.As.
func (e *ExitError) Unwrap() error {
	return e.Err
}

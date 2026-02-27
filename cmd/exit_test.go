package cmd

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitError_ErrorInterface(t *testing.T) {
	inner := errors.New("something went wrong")
	exitErr := &ExitError{Code: 2, Err: inner}

	// ExitError must implement the error interface
	var _ error = exitErr

	// Error() must delegate to Err.Error()
	got := exitErr.Error()
	want := "something went wrong"
	if got != want {
		t.Errorf("ExitError.Error() = %q, want %q", got, want)
	}
}

func TestExitError_Unwrap(t *testing.T) {
	inner := errors.New("original error")
	exitErr := &ExitError{Code: 3, Err: inner}

	// Wrap it further
	wrapped := fmt.Errorf("wrapper: %w", exitErr)

	// errors.As must be able to extract ExitError
	var extracted *ExitError
	if !errors.As(wrapped, &extracted) {
		t.Fatal("errors.As failed to extract ExitError from wrapped error")
	}
	if extracted.Code != 3 {
		t.Errorf("extracted ExitError.Code = %d, want 3", extracted.Code)
	}

	// errors.Is must work through Unwrap chain
	if !errors.Is(exitErr, inner) {
		t.Error("errors.Is(exitErr, inner) = false, want true")
	}
}

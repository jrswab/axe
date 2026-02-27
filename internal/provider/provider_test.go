package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestProviderError_ErrorInterface(t *testing.T) {
	pe := &ProviderError{
		Category: ErrCategoryAuth,
		Status:   401,
		Message:  "invalid api key",
		Err:      nil,
	}

	// Must implement error interface
	var _ error = pe

	// Error() must return "<category>: <message>"
	got := pe.Error()
	want := "auth: invalid api key"
	if got != want {
		t.Errorf("ProviderError.Error() = %q, want %q", got, want)
	}
}

func TestProviderError_Unwrap(t *testing.T) {
	inner := errors.New("original cause")
	pe := &ProviderError{
		Category: ErrCategoryServer,
		Status:   500,
		Message:  "server error",
		Err:      inner,
	}

	// Wrap it further
	wrapped := fmt.Errorf("wrapper: %w", pe)

	// errors.As must extract ProviderError
	var extracted *ProviderError
	if !errors.As(wrapped, &extracted) {
		t.Fatal("errors.As failed to extract ProviderError from wrapped error")
	}
	if extracted.Category != ErrCategoryServer {
		t.Errorf("extracted Category = %q, want %q", extracted.Category, ErrCategoryServer)
	}

	// errors.Is must work through Unwrap chain
	if !errors.Is(pe, inner) {
		t.Error("errors.Is(pe, inner) = false, want true")
	}
}

// TestProviderInterface_Compile verifies the Provider interface compiles correctly.
// This is a compile-time check, not a runtime test.
func TestProviderInterface_Compile(t *testing.T) {
	// Verify the interface has the expected method signature
	var _ Provider = (*mockProvider)(nil)
}

// mockProvider is a minimal implementation for compile-time interface check.
type mockProvider struct{}

func (m *mockProvider) Send(ctx context.Context, req *Request) (*Response, error) {
	return nil, nil
}

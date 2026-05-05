package runner

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jrswab/axe/internal/provider"
)

// --- ConfigError tests ---

func TestConfigError_Error_WithWrapped(t *testing.T) {
	inner := errors.New("inner cause")
	e := &ConfigError{Msg: "config problem", Err: inner}
	want := "config problem: inner cause"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestConfigError_Error_WithoutWrapped(t *testing.T) {
	e := &ConfigError{Msg: "config problem"}
	want := "config problem"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestConfigError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	e := &ConfigError{Msg: "msg", Err: inner}
	if !errors.Is(e, inner) {
		t.Error("expected errors.Is to find inner via Unwrap")
	}
}

// --- RuntimeError tests ---

func TestRuntimeError_Error_WithWrapped(t *testing.T) {
	inner := errors.New("runtime inner")
	e := &RuntimeError{Msg: "runtime problem", Err: inner}
	want := "runtime problem: runtime inner"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestRuntimeError_Error_WithoutWrapped(t *testing.T) {
	e := &RuntimeError{Msg: "runtime problem"}
	want := "runtime problem"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestRuntimeError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	e := &RuntimeError{Msg: "msg", Err: inner}
	if !errors.Is(e, inner) {
		t.Error("expected errors.Is to find inner via Unwrap")
	}
}

// --- BudgetExceededError tests ---

func TestBudgetExceededError_Error(t *testing.T) {
	e := &BudgetExceededError{Used: 150, Max: 100}
	want := "budget exceeded: used 150 of 100 tokens; increase --max-tokens or reduce prompt/tool output"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// --- IsConfigError tests ---

func TestIsConfigError_True(t *testing.T) {
	if !IsConfigError(&ConfigError{Msg: "x"}) {
		t.Error("expected true for *ConfigError")
	}
}

func TestIsConfigError_False(t *testing.T) {
	if IsConfigError(errors.New("plain")) {
		t.Error("expected false for plain error")
	}
	if IsConfigError(&RuntimeError{Msg: "x"}) {
		t.Error("expected false for *RuntimeError")
	}
	if IsConfigError(nil) {
		t.Error("expected false for nil")
	}
}

// --- IsRuntimeError tests ---

func TestIsRuntimeError_True(t *testing.T) {
	if !IsRuntimeError(&RuntimeError{Msg: "x"}) {
		t.Error("expected true for *RuntimeError")
	}
}

func TestIsRuntimeError_False(t *testing.T) {
	if IsRuntimeError(errors.New("plain")) {
		t.Error("expected false for plain error")
	}
	if IsRuntimeError(nil) {
		t.Error("expected false for nil")
	}
}

// --- ProviderCategory tests ---

func TestProviderCategory_Found(t *testing.T) {
	inner := &provider.ProviderError{Category: provider.ErrCategoryAuth, Message: "bad key"}
	wrapped := fmt.Errorf("wrapper: %w", inner)

	cat, ok := ProviderCategory(wrapped)
	if !ok {
		t.Fatal("expected ProviderCategory to find error")
	}
	if cat != provider.ErrCategoryAuth {
		t.Errorf("cat = %q, want auth", cat)
	}
}

func TestProviderCategory_NotFound(t *testing.T) {
	_, ok := ProviderCategory(errors.New("plain"))
	if ok {
		t.Error("expected not found for plain error")
	}
}

func TestProviderCategory_Nil(t *testing.T) {
	_, ok := ProviderCategory(nil)
	if ok {
		t.Error("expected not found for nil")
	}
}

// --- AsProviderError tests ---

func TestAsProviderError_Success(t *testing.T) {
	inner := &provider.ProviderError{Category: provider.ErrCategoryRateLimit, Message: "slow down"}
	wrapped := fmt.Errorf("wrapped: %w", inner)

	var target *provider.ProviderError
	if !AsProviderError(wrapped, &target) {
		t.Fatal("expected AsProviderError to succeed")
	}
	if target.Message != "slow down" {
		t.Errorf("message = %q, want 'slow down'", target.Message)
	}
}

func TestAsProviderError_DirectError(t *testing.T) {
	inner := &provider.ProviderError{Category: provider.ErrCategoryServer, Message: "boom"}
	var target *provider.ProviderError
	if !AsProviderError(inner, &target) {
		t.Fatal("expected AsProviderError to succeed on direct error")
	}
	if target.Message != "boom" {
		t.Errorf("message = %q, want 'boom'", target.Message)
	}
}

func TestAsProviderError_NilError(t *testing.T) {
	var target *provider.ProviderError
	if AsProviderError(nil, &target) {
		t.Error("expected false for nil error")
	}
}

func TestAsProviderError_NilTarget(t *testing.T) {
	e := &provider.ProviderError{Category: provider.ErrCategoryAuth}
	if AsProviderError(e, nil) {
		t.Error("expected false for nil target")
	}
}

func TestAsProviderError_NotFound(t *testing.T) {
	var target *provider.ProviderError
	if AsProviderError(errors.New("plain"), &target) {
		t.Error("expected false when no ProviderError in chain")
	}
}

func TestAsProviderError_DeeplyNested(t *testing.T) {
	inner := &provider.ProviderError{Category: provider.ErrCategoryTimeout}
	lvl1 := fmt.Errorf("l1: %w", inner)
	lvl2 := fmt.Errorf("l2: %w", lvl1)
	lvl3 := &RuntimeError{Msg: "l3", Err: lvl2}

	var target *provider.ProviderError
	if !AsProviderError(lvl3, &target) {
		t.Fatal("expected AsProviderError to find deeply nested ProviderError")
	}
	if target.Category != provider.ErrCategoryTimeout {
		t.Errorf("category = %q, want timeout", target.Category)
	}
}

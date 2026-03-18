package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"time"
)

// RetryConfig holds configuration for the retry decorator.
type RetryConfig struct {
	MaxRetries     int
	Backoff        string // "exponential", "linear", "fixed"
	InitialDelayMs int
	MaxDelayMs     int
	Verbose        bool
	Stderr         io.Writer
}

// RetryProvider wraps a Provider with configurable retry logic.
type RetryProvider struct {
	inner    Provider
	cfg      RetryConfig
	attempts int // cumulative retry count across all Send() calls
}

// NewRetry creates a RetryProvider wrapping the given provider.
// Runtime defaults are applied for zero-valued config fields.
func NewRetry(p Provider, cfg RetryConfig) *RetryProvider {
	if cfg.Backoff == "" {
		cfg.Backoff = "exponential"
	}
	if cfg.InitialDelayMs == 0 {
		cfg.InitialDelayMs = 500
	}
	if cfg.MaxDelayMs == 0 {
		cfg.MaxDelayMs = 30000
	}
	return &RetryProvider{inner: p, cfg: cfg}
}

// Attempts returns the cumulative number of retry attempts across all Send() calls.
func (r *RetryProvider) Attempts() int {
	return r.attempts
}

// Send delegates to the wrapped provider with retry logic.
// When MaxRetries is 0, it passes through directly with no overhead.
func (r *RetryProvider) Send(ctx context.Context, req *Request) (*Response, error) {
	if r.cfg.MaxRetries <= 0 {
		return r.inner.Send(ctx, req)
	}

	resp, err := r.inner.Send(ctx, req)
	if err == nil {
		return resp, nil
	}

	for attempt := 0; attempt < r.cfg.MaxRetries; attempt++ {
		if !isRetriable(err) {
			return nil, err
		}

		// Check context before sleeping
		if ctx.Err() != nil {
			return nil, ctxCancelErr(ctx)
		}

		delay := computeDelay(attempt, r.cfg)

		// Log retry attempt if verbose
		if r.cfg.Verbose && r.cfg.Stderr != nil {
			var provErr *ProviderError
			category := "unknown"
			if errors.As(err, &provErr) {
				category = string(provErr.Category)
			}
			_, _ = fmt.Fprintf(r.cfg.Stderr, "[retry] Attempt %d/%d after %s, waiting %dms\n",
				attempt+1, r.cfg.MaxRetries, category, delay.Milliseconds())
		}

		// Sleep with context cancellation support
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctxCancelErr(ctx)
		}

		r.attempts++
		resp, err = r.inner.Send(ctx, req)
		if err == nil {
			return resp, nil
		}
	}

	return nil, err
}

// ctxCancelErr returns an appropriate error when the context is cancelled.
// For DeadlineExceeded, it returns a ProviderError with ErrCategoryTimeout.
// For plain Canceled, it returns ctx.Err() as-is.
func ctxCancelErr(ctx context.Context) error {
	if ctx.Err() == context.DeadlineExceeded {
		return &ProviderError{
			Category: ErrCategoryTimeout,
			Message:  ctx.Err().Error(),
			Err:      ctx.Err(),
		}
	}
	return ctx.Err()
}

// isRetriable returns true if the error is a ProviderError with a retriable category.
func isRetriable(err error) bool {
	if err == nil {
		return false
	}
	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		return false
	}
	switch provErr.Category {
	case ErrCategoryRateLimit, ErrCategoryServer, ErrCategoryOverloaded, ErrCategoryTimeout:
		return true
	default:
		return false
	}
}

// computeDelay calculates the backoff delay for the given attempt number.
func computeDelay(attempt int, cfg RetryConfig) time.Duration {
	var delayMs int

	switch cfg.Backoff {
	case "fixed":
		delayMs = cfg.InitialDelayMs
	case "linear":
		delayMs = cfg.InitialDelayMs * (attempt + 1)
	case "exponential":
		// Guard against overflow: if attempt >= 62 or shift would overflow, use max directly
		if attempt >= 62 {
			return time.Duration(cfg.MaxDelayMs) * time.Millisecond
		}
		shift := int64(1) << uint(attempt)
		base := int64(cfg.InitialDelayMs) * shift
		// Check for overflow
		if base/shift != int64(cfg.InitialDelayMs) || base < 0 {
			return time.Duration(cfg.MaxDelayMs) * time.Millisecond
		}
		jitter := int64(0)
		if cfg.InitialDelayMs > 0 {
			jitter = int64(rand.IntN(cfg.InitialDelayMs))
		}
		total := base + jitter
		if total < 0 { // overflow
			return time.Duration(cfg.MaxDelayMs) * time.Millisecond
		}
		delayMs = int(total)
	default:
		delayMs = cfg.InitialDelayMs
	}

	if delayMs > cfg.MaxDelayMs {
		delayMs = cfg.MaxDelayMs
	}

	return time.Duration(delayMs) * time.Millisecond
}

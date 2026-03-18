package provider

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

// retryMockProvider is a test helper that returns pre-configured responses/errors.
type retryMockProvider struct {
	responses []*Response
	errors    []error
	calls     int
}

func (m *retryMockProvider) Send(ctx context.Context, req *Request) (*Response, error) {
	i := m.calls
	m.calls++
	if i < len(m.errors) && m.errors[i] != nil {
		return nil, m.errors[i]
	}
	if i < len(m.responses) && m.responses[i] != nil {
		return m.responses[i], nil
	}
	return &Response{Content: "ok"}, nil
}

// --- 2f: isRetriable tests ---

func TestIsRetriable_RateLimit(t *testing.T) {
	err := &ProviderError{Category: ErrCategoryRateLimit}
	if !isRetriable(err) {
		t.Error("expected rate_limit to be retriable")
	}
}

func TestIsRetriable_Server(t *testing.T) {
	err := &ProviderError{Category: ErrCategoryServer}
	if !isRetriable(err) {
		t.Error("expected server to be retriable")
	}
}

func TestIsRetriable_Overloaded(t *testing.T) {
	err := &ProviderError{Category: ErrCategoryOverloaded}
	if !isRetriable(err) {
		t.Error("expected overloaded to be retriable")
	}
}

func TestIsRetriable_Timeout(t *testing.T) {
	err := &ProviderError{Category: ErrCategoryTimeout}
	if !isRetriable(err) {
		t.Error("expected timeout to be retriable")
	}
}

func TestIsRetriable_Auth(t *testing.T) {
	err := &ProviderError{Category: ErrCategoryAuth}
	if isRetriable(err) {
		t.Error("expected auth to NOT be retriable")
	}
}

func TestIsRetriable_BadRequest(t *testing.T) {
	err := &ProviderError{Category: ErrCategoryBadRequest}
	if isRetriable(err) {
		t.Error("expected bad_request to NOT be retriable")
	}
}

func TestIsRetriable_PlainError(t *testing.T) {
	err := errors.New("something went wrong")
	if isRetriable(err) {
		t.Error("expected plain error to NOT be retriable")
	}
}

func TestIsRetriable_Nil(t *testing.T) {
	if isRetriable(nil) {
		t.Error("expected nil to NOT be retriable")
	}
}

// --- 2g: computeDelay tests ---

func TestComputeDelay_Fixed(t *testing.T) {
	cfg := RetryConfig{Backoff: "fixed", InitialDelayMs: 100, MaxDelayMs: 5000}
	for _, attempt := range []int{0, 1, 2, 5, 10} {
		d := computeDelay(attempt, cfg)
		want := 100 * time.Millisecond
		if d != want {
			t.Errorf("attempt %d: got %v, want %v", attempt, d, want)
		}
	}
}

func TestComputeDelay_Fixed_Capped(t *testing.T) {
	cfg := RetryConfig{Backoff: "fixed", InitialDelayMs: 500, MaxDelayMs: 100}
	d := computeDelay(0, cfg)
	want := 100 * time.Millisecond
	if d != want {
		t.Errorf("got %v, want %v", d, want)
	}
}

func TestComputeDelay_Linear(t *testing.T) {
	cfg := RetryConfig{Backoff: "linear", InitialDelayMs: 100, MaxDelayMs: 5000}
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 300 * time.Millisecond},
	}
	for _, tt := range tests {
		d := computeDelay(tt.attempt, cfg)
		if d != tt.want {
			t.Errorf("attempt %d: got %v, want %v", tt.attempt, d, tt.want)
		}
	}
}

func TestComputeDelay_Linear_Capped(t *testing.T) {
	cfg := RetryConfig{Backoff: "linear", InitialDelayMs: 100, MaxDelayMs: 250}
	d := computeDelay(5, cfg)
	want := 250 * time.Millisecond
	if d != want {
		t.Errorf("got %v, want %v", d, want)
	}
}

func TestComputeDelay_Exponential_Range(t *testing.T) {
	cfg := RetryConfig{Backoff: "exponential", InitialDelayMs: 100, MaxDelayMs: 50000}
	// attempt 0: base = 100*1 = 100, jitter in [0,100), so delay in [100, 200)
	d := computeDelay(0, cfg)
	if d < 100*time.Millisecond || d >= 200*time.Millisecond {
		t.Errorf("attempt 0: got %v, want in [100ms, 200ms)", d)
	}
	// attempt 1: base = 100*2 = 200, jitter in [0,100), so delay in [200, 300)
	d = computeDelay(1, cfg)
	if d < 200*time.Millisecond || d >= 300*time.Millisecond {
		t.Errorf("attempt 1: got %v, want in [200ms, 300ms)", d)
	}
	// attempt 2: base = 100*4 = 400, jitter in [0,100), so delay in [400, 500)
	d = computeDelay(2, cfg)
	if d < 400*time.Millisecond || d >= 500*time.Millisecond {
		t.Errorf("attempt 2: got %v, want in [400ms, 500ms)", d)
	}
}

func TestComputeDelay_Exponential_Capped(t *testing.T) {
	cfg := RetryConfig{Backoff: "exponential", InitialDelayMs: 100, MaxDelayMs: 250}
	// attempt 2: base = 400 + jitter, but capped at 250
	d := computeDelay(2, cfg)
	if d > 250*time.Millisecond {
		t.Errorf("got %v, want <= 250ms", d)
	}
}

func TestComputeDelay_Exponential_Overflow(t *testing.T) {
	cfg := RetryConfig{Backoff: "exponential", InitialDelayMs: 500, MaxDelayMs: 30000}
	// attempt 100 would overflow int — must not panic, must return max_delay_ms
	d := computeDelay(100, cfg)
	want := 30000 * time.Millisecond
	if d != want {
		t.Errorf("got %v, want %v", d, want)
	}
}

func TestComputeDelay_MaxDelayLessThanInitial(t *testing.T) {
	cfg := RetryConfig{Backoff: "exponential", InitialDelayMs: 500, MaxDelayMs: 100}
	d := computeDelay(0, cfg)
	want := 100 * time.Millisecond
	if d != want {
		t.Errorf("got %v, want %v (capped at max_delay_ms)", d, want)
	}
}

// --- 2h: Send success on first try ---

func TestRetrySend_SuccessFirstTry(t *testing.T) {
	mock := &retryMockProvider{
		responses: []*Response{{Content: "hello"}},
	}
	rp := NewRetry(mock, RetryConfig{MaxRetries: 3, InitialDelayMs: 1, MaxDelayMs: 10})
	resp, err := rp.Send(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello" {
		t.Errorf("got content %q, want %q", resp.Content, "hello")
	}
	if rp.Attempts() != 0 {
		t.Errorf("got %d attempts, want 0", rp.Attempts())
	}
	if mock.calls != 1 {
		t.Errorf("got %d calls, want 1", mock.calls)
	}
}

// --- 2i: Send success after retries ---

func TestRetrySend_SuccessAfterRetries(t *testing.T) {
	mock := &retryMockProvider{
		errors: []error{
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			nil, // success on 3rd call
		},
		responses: []*Response{nil, nil, {Content: "ok"}},
	}
	rp := NewRetry(mock, RetryConfig{MaxRetries: 3, InitialDelayMs: 1, MaxDelayMs: 10})
	resp, err := rp.Send(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("got content %q, want %q", resp.Content, "ok")
	}
	if rp.Attempts() != 2 {
		t.Errorf("got %d attempts, want 2", rp.Attempts())
	}
	if mock.calls != 3 {
		t.Errorf("got %d calls, want 3", mock.calls)
	}
}

// --- 2j: Send exhaustion ---

func TestRetrySend_Exhaustion(t *testing.T) {
	serverErr := &ProviderError{Category: ErrCategoryServer, Message: "internal error"}
	mock := &retryMockProvider{
		errors: []error{serverErr, serverErr, serverErr},
	}
	rp := NewRetry(mock, RetryConfig{MaxRetries: 2, InitialDelayMs: 1, MaxDelayMs: 10})
	_, err := rp.Send(context.Background(), &Request{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("got category %s, want server", provErr.Category)
	}
	if rp.Attempts() != 2 {
		t.Errorf("got %d attempts, want 2", rp.Attempts())
	}
	if mock.calls != 3 {
		t.Errorf("got %d calls, want 3", mock.calls)
	}
}

// --- 2k: Send non-retriable error ---

func TestRetrySend_NonRetriableAuth(t *testing.T) {
	mock := &retryMockProvider{
		errors: []error{&ProviderError{Category: ErrCategoryAuth, Message: "bad key"}},
	}
	rp := NewRetry(mock, RetryConfig{MaxRetries: 3, InitialDelayMs: 1, MaxDelayMs: 10})
	_, err := rp.Send(context.Background(), &Request{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if rp.Attempts() != 0 {
		t.Errorf("got %d attempts, want 0", rp.Attempts())
	}
	if mock.calls != 1 {
		t.Errorf("got %d calls, want 1", mock.calls)
	}
}

// --- 2l: Send non-ProviderError ---

func TestRetrySend_NonProviderError(t *testing.T) {
	mock := &retryMockProvider{
		errors: []error{errors.New("unexpected failure")},
	}
	rp := NewRetry(mock, RetryConfig{MaxRetries: 3, InitialDelayMs: 1, MaxDelayMs: 10})
	_, err := rp.Send(context.Background(), &Request{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if rp.Attempts() != 0 {
		t.Errorf("got %d attempts, want 0", rp.Attempts())
	}
	if mock.calls != 1 {
		t.Errorf("got %d calls, want 1", mock.calls)
	}
}

// --- 2m: Send retriable then non-retriable ---

func TestRetrySend_RetriableThenNonRetriable(t *testing.T) {
	mock := &retryMockProvider{
		errors: []error{
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			&ProviderError{Category: ErrCategoryAuth, Message: "bad key"},
		},
	}
	rp := NewRetry(mock, RetryConfig{MaxRetries: 3, InitialDelayMs: 1, MaxDelayMs: 10})
	_, err := rp.Send(context.Background(), &Request{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryAuth {
		t.Errorf("got category %s, want auth", provErr.Category)
	}
	if rp.Attempts() != 1 {
		t.Errorf("got %d attempts, want 1", rp.Attempts())
	}
	if mock.calls != 2 {
		t.Errorf("got %d calls, want 2", mock.calls)
	}
}

// --- 2n: Send context cancellation during backoff ---

func TestRetrySend_ContextCancellation(t *testing.T) {
	mock := &retryMockProvider{
		errors: []error{
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
		},
	}
	rp := NewRetry(mock, RetryConfig{MaxRetries: 5, InitialDelayMs: 5000, MaxDelayMs: 10000})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := rp.Send(ctx, &Request{})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should return quickly, not wait 5 seconds
	if elapsed > 500*time.Millisecond {
		t.Errorf("took %v, expected < 500ms (context should cancel backoff)", elapsed)
	}
	// Mock should be called at most twice (initial + maybe one retry before cancel)
	if mock.calls > 2 {
		t.Errorf("got %d calls, expected at most 2", mock.calls)
	}
}

// --- 2o: Send zero max_retries passthrough ---

func TestRetrySend_ZeroMaxRetries(t *testing.T) {
	mock := &retryMockProvider{
		errors: []error{&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"}},
	}
	rp := NewRetry(mock, RetryConfig{MaxRetries: 0, InitialDelayMs: 1, MaxDelayMs: 10})
	_, err := rp.Send(context.Background(), &Request{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if rp.Attempts() != 0 {
		t.Errorf("got %d attempts, want 0", rp.Attempts())
	}
	if mock.calls != 1 {
		t.Errorf("got %d calls, want 1", mock.calls)
	}
}

// --- 2p: Send verbose logging ---

func TestRetrySend_VerboseLogging(t *testing.T) {
	mock := &retryMockProvider{
		errors: []error{
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			nil,
		},
		responses: []*Response{nil, {Content: "ok"}},
	}
	var buf bytes.Buffer
	rp := NewRetry(mock, RetryConfig{
		MaxRetries:     3,
		InitialDelayMs: 1,
		MaxDelayMs:     10,
		Verbose:        true,
		Stderr:         &buf,
	})
	_, err := rp.Send(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("[retry] Attempt 1/")) {
		t.Errorf("expected verbose output containing '[retry] Attempt 1/', got %q", output)
	}
	if !bytes.Contains([]byte(output), []byte("rate_limit")) {
		t.Errorf("expected verbose output containing 'rate_limit', got %q", output)
	}
}

// --- 2q: Send no output when not verbose ---

func TestRetrySend_SilentWhenNotVerbose(t *testing.T) {
	mock := &retryMockProvider{
		errors: []error{
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			nil,
		},
		responses: []*Response{nil, {Content: "ok"}},
	}
	var buf bytes.Buffer
	rp := NewRetry(mock, RetryConfig{
		MaxRetries:     3,
		InitialDelayMs: 1,
		MaxDelayMs:     10,
		Verbose:        false,
		Stderr:         &buf,
	})
	_, err := rp.Send(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output when not verbose, got %q", buf.String())
	}
}

// --- Context cancellation error classification tests ---

func TestRetrySend_ContextDeadlineExceeded(t *testing.T) {
	// Create a mock provider that returns a retriable error
	mock := &retryMockProvider{
		errors: []error{
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
		},
	}
	// Very short deadline (10ms) with long backoff (5000ms) ensures deadline fires during backoff
	rp := NewRetry(mock, RetryConfig{MaxRetries: 5, InitialDelayMs: 5000, MaxDelayMs: 10000})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := rp.Send(ctx, &Request{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should be able to unwrap to ProviderError
	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected error to be a *ProviderError (via errors.As), got %T: %v", err, err)
	}

	if provErr.Category != ErrCategoryTimeout {
		t.Errorf("expected Category = ErrCategoryTimeout, got %s", provErr.Category)
	}

	// The underlying error should be context.DeadlineExceeded
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected error to wrap context.DeadlineExceeded (via errors.Is)")
	}
}

func TestRetrySend_ContextCanceled_NotProviderError(t *testing.T) {
	// Create a mock provider that returns a retriable error
	mock := &retryMockProvider{
		errors: []error{
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
			&ProviderError{Category: ErrCategoryRateLimit, Message: "rate limited"},
		},
	}
	rp := NewRetry(mock, RetryConfig{MaxRetries: 5, InitialDelayMs: 5000, MaxDelayMs: 10000})
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	_, err := rp.Send(ctx, &Request{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should NOT be a ProviderError - plain cancellation is not a provider error
	var provErr *ProviderError
	if errors.As(err, &provErr) {
		t.Errorf("expected plain context.Canceled error, but got ProviderError: %v", err)
	}

	// The error should be context.Canceled
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected error to be context.Canceled, got %T: %v", err, err)
	}
}

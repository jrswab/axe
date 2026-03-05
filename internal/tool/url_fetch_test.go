package tool

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrswab/axe/internal/provider"
)

func TestURLFetch_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello world"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-success", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "hello world" {
		t.Errorf("Content = %q, want %q", result.Content, "hello world")
	}
	if result.CallID != call.ID {
		t.Errorf("CallID = %q, want %q", result.CallID, call.ID)
	}
}

func TestURLFetch_EmptyURL(t *testing.T) {
	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-empty", Arguments: map[string]string{}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "url is required") {
		t.Errorf("Content = %q, want contains %q", result.Content, "url is required")
	}
}

func TestURLFetch_MissingURLArgument(t *testing.T) {
	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-missing", Arguments: map[string]string{"url": ""}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "url is required") {
		t.Errorf("Content = %q, want contains %q", result.Content, "url is required")
	}
}

func TestURLFetch_UnsupportedScheme_File(t *testing.T) {
	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-file", Arguments: map[string]string{"url": "file:///etc/passwd"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "unsupported scheme") {
		t.Errorf("Content = %q, want contains %q", result.Content, "unsupported scheme")
	}
	if !strings.Contains(result.Content, "file") {
		t.Errorf("Content = %q, want contains %q", result.Content, "file")
	}
}

func TestURLFetch_UnsupportedScheme_FTP(t *testing.T) {
	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-ftp", Arguments: map[string]string{"url": "ftp://example.com/file"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "unsupported scheme") {
		t.Errorf("Content = %q, want contains %q", result.Content, "unsupported scheme")
	}
	if !strings.Contains(result.Content, "ftp") {
		t.Errorf("Content = %q, want contains %q", result.Content, "ftp")
	}
}

func TestURLFetch_NoScheme(t *testing.T) {
	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-noscheme", Arguments: map[string]string{"url": "example.com"}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "unsupported scheme") {
		t.Errorf("Content = %q, want contains %q", result.Content, "unsupported scheme")
	}
}

func TestURLFetch_Non2xxStatus_404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-404", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "HTTP 404") {
		t.Errorf("Content = %q, want contains %q", result.Content, "HTTP 404")
	}
	if !strings.Contains(result.Content, "not found") {
		t.Errorf("Content = %q, want contains %q", result.Content, "not found")
	}
}

func TestURLFetch_Non2xxStatus_500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-500", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "HTTP 500") {
		t.Errorf("Content = %q, want contains %q", result.Content, "HTTP 500")
	}
	if !strings.Contains(result.Content, "internal error") {
		t.Errorf("Content = %q, want contains %q", result.Content, "internal error")
	}
}

func TestURLFetch_LargeResponseTruncation(t *testing.T) {
	largeBody := strings.Repeat("A", 20000)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(largeBody))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-large", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if !strings.Contains(result.Content, "[response truncated, exceeded 10000 characters]") {
		t.Errorf("Content missing truncation notice: %q", result.Content)
	}
	if len(result.Content) <= 10000 {
		t.Errorf("len(Content) = %d, want > 10000", len(result.Content))
	}
	if len(result.Content) >= 20000 {
		t.Errorf("len(Content) = %d, want < 20000", len(result.Content))
	}
}

func TestURLFetch_ExactLimitNotTruncated(t *testing.T) {
	exactBody := strings.Repeat("B", 10000)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(exactBody))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-exact", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if strings.Contains(result.Content, "truncated") {
		t.Errorf("Content = %q, should not contain truncation notice", result.Content)
	}
	if len(result.Content) != 10000 {
		t.Errorf("len(Content) = %d, want 10000", len(result.Content))
	}
}

func TestURLFetch_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()

		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
			_, _ = w.Write([]byte("too late"))
		}
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	entry := urlFetchEntry()
	result := entry.Execute(ctx, provider.ToolCall{ID: "uf-timeout", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
}

func TestURLFetch_ConnectionRefused(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("listener.Close error: %v", err)
	}

	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-refused", Arguments: map[string]string{"url": "http://" + addr}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if result.Content == "" {
		t.Fatal("expected non-empty error content")
	}
}

func TestURLFetch_CallIDPassthrough(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-unique-42", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.CallID != "uf-unique-42" {
		t.Errorf("CallID = %q, want %q", result.CallID, "uf-unique-42")
	}
}

func TestURLFetch_EmptyResponseBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-empty-body", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "" {
		t.Errorf("Content = %q, want empty string", result.Content)
	}
}

func TestURLFetch_Non2xxWithLargeBody(t *testing.T) {
	largeBody := strings.Repeat("C", 20000)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(largeBody))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{ID: "uf-err-large", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "HTTP 500") {
		t.Errorf("Content = %q, want contains %q", result.Content, "HTTP 500")
	}
	if !strings.Contains(result.Content, "[response truncated, exceeded 10000 characters]") {
		t.Errorf("Content missing truncation notice: %q", result.Content)
	}
}

func TestURLFetch_VerboseLog_SanitizesURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	targetURL := strings.Replace(ts.URL, "http://", "http://user:pass@", 1) + "/secret/path?token=abc#frag"

	var stderr bytes.Buffer
	entry := urlFetchEntry()
	_ = entry.Execute(context.Background(), provider.ToolCall{ID: "uf-verbose", Arguments: map[string]string{"url": targetURL}}, ExecContext{Verbose: true, Stderr: &stderr})

	logOutput := stderr.String()
	if strings.Contains(logOutput, "user:pass") {
		t.Errorf("verbose log leaked credentials: %q", logOutput)
	}
	if strings.Contains(logOutput, "token=abc") {
		t.Errorf("verbose log leaked query string: %q", logOutput)
	}
	if strings.Contains(logOutput, "#frag") {
		t.Errorf("verbose log leaked fragment: %q", logOutput)
	}
	if !strings.Contains(logOutput, "/secret/path") {
		t.Errorf("verbose log missing sanitized path: %q", logOutput)
	}
}

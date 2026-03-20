package tool

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jrswab/axe/internal/hostcheck"
	"github.com/jrswab/axe/internal/provider"
)

// fakeURLFetchResolver is a test-only Resolver that returns preset addresses.
type fakeURLFetchResolver struct {
	addrs []net.IPAddr
	err   error
}

func (f *fakeURLFetchResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f.addrs, f.err
}

// skipHostCheck bypasses the host check for tests that use httptest.NewServer
// (which binds to 127.0.0.1, a private IP that would otherwise be rejected).
func skipHostCheck(t *testing.T) {
	t.Helper()
	orig := urlFetchCheckHost
	urlFetchCheckHost = func(ctx context.Context, hostname string, allowlist []string, resolver hostcheck.Resolver) (net.IP, error) {
		return net.ParseIP("127.0.0.1"), nil
	}
	t.Cleanup(func() { urlFetchCheckHost = orig })
}

func TestURLFetch_Success(t *testing.T) {
	skipHostCheck(t)
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
	skipHostCheck(t)
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
	skipHostCheck(t)
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
	skipHostCheck(t)
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
	skipHostCheck(t)
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
	skipHostCheck(t)
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
	skipHostCheck(t)
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
	skipHostCheck(t)
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
	skipHostCheck(t)
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
	skipHostCheck(t)
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
	skipHostCheck(t)
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

func TestURLFetch_PerRequestTimeout(t *testing.T) {
	skipHostCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block longer than the per-request timeout
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
			_, _ = w.Write([]byte("too late"))
		}
	}))
	defer ts.Close()

	// Override the per-request timeout to keep the test fast
	orig := urlFetchTimeout
	urlFetchTimeout = 100 * time.Millisecond
	defer func() { urlFetchTimeout = orig }()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-per-req-timeout", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true when per-request timeout fires")
	}
	if result.CallID != call.ID {
		t.Errorf("CallID = %q, want %q", result.CallID, call.ID)
	}
	if result.Content == "" {
		t.Error("expected non-empty error content")
	}
}

func TestURLFetch_ParentContextWinsOverPerRequestTimeout(t *testing.T) {
	skipHostCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
			_, _ = w.Write([]byte("too late"))
		}
	}))
	defer ts.Close()

	// Set per-request timeout to something long so the parent wins
	orig := urlFetchTimeout
	urlFetchTimeout = 10 * time.Second
	defer func() { urlFetchTimeout = orig }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-parent-wins", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(ctx, call, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true when parent context fires first")
	}
}

func TestURLFetch_FastResponseUnaffectedByTimeout(t *testing.T) {
	skipHostCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fast response"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-fast", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "fast response" {
		t.Errorf("Content = %q, want %q", result.Content, "fast response")
	}
	if result.CallID != call.ID {
		t.Errorf("CallID = %q, want %q", result.CallID, call.ID)
	}
}

// Phase 1A: stripHTML unit tests

func TestStripHTML_BasicExtraction(t *testing.T) {
	got := stripHTML("<html><body><p>Hello</p><p>World</p></body></html>")
	want := "Hello World"
	if got != want {
		t.Errorf("stripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTML_RemovesScriptAndStyle(t *testing.T) {
	got := stripHTML("<html><body><script>var x=1;</script><p>Keep this</p><style>.foo{color:red}</style></body></html>")
	want := "Keep this"
	if got != want {
		t.Errorf("stripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTML_PreservesNoscript(t *testing.T) {
	got := stripHTML("<html><body><noscript>Enable JS</noscript><p>Main content</p></body></html>")
	want := "Enable JS Main content"
	if got != want {
		t.Errorf("stripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTML_NestedScriptInDiv(t *testing.T) {
	got := stripHTML("<div>Before<script>alert('x')</script>After</div>")
	want := "Before After"
	if got != want {
		t.Errorf("stripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTML_WhitespaceCollapsing(t *testing.T) {
	got := stripHTML("<p>  Hello   \n\n  World  </p>")
	want := "Hello World"
	if got != want {
		t.Errorf("stripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTML_EmptyInput(t *testing.T) {
	got := stripHTML("")
	want := ""
	if got != want {
		t.Errorf("stripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTML_OnlyScriptsAndStyles(t *testing.T) {
	got := stripHTML("<html><body><script>code</script><style>css</style></body></html>")
	want := ""
	if got != want {
		t.Errorf("stripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTML_HTMLEntities(t *testing.T) {
	got := stripHTML("<p>Tom &amp; Jerry &#x27;s</p>")
	want := "Tom & Jerry 's"
	if got != want {
		t.Errorf("stripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTML_NoHTMLTags(t *testing.T) {
	got := stripHTML("Just plain text")
	want := "Just plain text"
	if got != want {
		t.Errorf("stripHTML() = %q, want %q", got, want)
	}
}

// Phase 1B: urlFetchExecute integration tests

func TestURLFetch_HTMLContentTypeStripsHTML(t *testing.T) {
	skipHostCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><script>x</script><p>Visible</p></body></html>"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-html-strip", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "Visible" {
		t.Errorf("Content = %q, want %q", result.Content, "Visible")
	}
}

func TestURLFetch_HTMLContentTypeWithCharset(t *testing.T) {
	skipHostCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<p>Hello charset</p>"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-html-charset", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "Hello charset" {
		t.Errorf("Content = %q, want %q", result.Content, "Hello charset")
	}
}

func TestURLFetch_HTMLContentTypeCaseInsensitive(t *testing.T) {
	skipHostCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "TEXT/HTML")
		_, _ = w.Write([]byte("<p>Case test</p>"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-html-case", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "Case test" {
		t.Errorf("Content = %q, want %q", result.Content, "Case test")
	}
}

func TestURLFetch_NonHTMLContentTypeNotStripped(t *testing.T) {
	skipHostCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key": "<b>value</b>"}`))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-json-no-strip", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	want := `{"key": "<b>value</b>"}`
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

func TestURLFetch_PlainTextNotStripped(t *testing.T) {
	skipHostCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("<p>Not HTML</p>"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-plain-no-strip", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "<p>Not HTML</p>" {
		t.Errorf("Content = %q, want %q", result.Content, "<p>Not HTML</p>")
	}
}

func TestURLFetch_MissingContentTypeNotStripped(t *testing.T) {
	skipHostCheck(t)
	// Note: Go's http.DetectContentType may auto-set Content-Type.
	// We explicitly set it to empty to test the missing header case.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "")
		_, _ = w.Write([]byte("<p>No header</p>"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-no-ct", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	// With empty Content-Type, body should be returned verbatim (no stripping)
	if result.Content != "<p>No header</p>" {
		t.Errorf("Content = %q, want %q", result.Content, "<p>No header</p>")
	}
}

func TestURLFetch_Non2xxHTMLStripped(t *testing.T) {
	skipHostCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("<html><body><script>x</script><p>Not Found</p></body></html>"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-404-html", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "HTTP 404") {
		t.Errorf("Content = %q, want contains %q", result.Content, "HTTP 404")
	}
	if !strings.Contains(result.Content, "Not Found") {
		t.Errorf("Content = %q, want contains %q", result.Content, "Not Found")
	}
	if strings.Contains(result.Content, "<script>") {
		t.Errorf("Content = %q, should not contain <script>", result.Content)
	}
}

func TestURLFetch_Non2xxNonHTMLNotStripped(t *testing.T) {
	skipHostCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "bad"}`))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-500-json", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "HTTP 500") {
		t.Errorf("Content = %q, want contains %q", result.Content, "HTTP 500")
	}
	if !strings.Contains(result.Content, "bad") {
		t.Errorf("Content = %q, want contains %q", result.Content, "bad")
	}
}

func TestURLFetch_HTMLStrippedBeforeTruncation(t *testing.T) {
	skipHostCheck(t)
	// Build HTML where raw size exceeds maxReadBytes but stripped text is small.
	// 9950 bytes of CSS in a <style> tag + small visible text.
	bigCSS := strings.Repeat("x", 9950)
	htmlBody := "<html><body><style>" + bigCSS + "</style><p>Short text</p></body></html>"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(htmlBody))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	call := provider.ToolCall{ID: "uf-strip-before-trunc", Arguments: map[string]string{"url": ts.URL}}
	result := entry.Execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if strings.Contains(result.Content, "truncated") {
		t.Errorf("Content should not contain truncation notice after stripping: %q", result.Content)
	}
	if !strings.Contains(result.Content, "Short text") {
		t.Errorf("Content = %q, want contains %q", result.Content, "Short text")
	}
}

// Phase 4: Allowlist and private IP tests

func TestURLFetch_AllowlistPermitsMatchingHost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	// Extract hostname from test server URL (will be 127.0.0.1)
	u, _ := url.Parse(ts.URL)
	hostname := u.Hostname()

	// Override host check: verify allowlist logic passes, bypass private IP check
	// so the test server (127.0.0.1) is reachable.
	orig := urlFetchCheckHost
	urlFetchCheckHost = func(ctx context.Context, h string, allowlist []string, resolver hostcheck.Resolver) (net.IP, error) {
		if !hostcheck.IsAllowed(h, allowlist) {
			return nil, &net.AddrError{Err: "not in allowed_hosts", Addr: h}
		}
		return net.ParseIP("127.0.0.1"), nil
	}
	defer func() { urlFetchCheckHost = orig }()

	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-1",
		Name:      "url_fetch",
		Arguments: map[string]string{"url": ts.URL},
	}, ExecContext{AllowedHosts: []string{hostname}})

	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Content)
	}
}

func TestURLFetch_AllowlistBlocksNonMatchingHost(t *testing.T) {
	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-2",
		Name:      "url_fetch",
		Arguments: map[string]string{"url": "https://blocked.example.com/page"},
	}, ExecContext{AllowedHosts: []string{"allowed.example.com"}})

	if !result.IsError {
		t.Error("expected error for non-matching host")
	}
	if !strings.Contains(result.Content, "not in allowed_hosts") {
		t.Errorf("expected 'not in allowed_hosts' in error, got: %s", result.Content)
	}
}

func TestURLFetch_EmptyAllowlistPermitsAll(t *testing.T) {
	skipHostCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-3",
		Name:      "url_fetch",
		Arguments: map[string]string{"url": ts.URL},
	}, ExecContext{})

	if result.IsError {
		t.Errorf("expected success with empty allowlist, got error: %s", result.Content)
	}
}

func TestURLFetch_BlocksPrivateIP(t *testing.T) {
	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-4",
		Name:      "url_fetch",
		Arguments: map[string]string{"url": "http://192.168.1.1/data"},
	}, ExecContext{})

	if !result.IsError {
		t.Error("expected error for private IP")
	}
	if !strings.Contains(result.Content, "private") {
		t.Errorf("expected 'private' in error, got: %s", result.Content)
	}
}

func TestURLFetch_BlocksLoopbackIP(t *testing.T) {
	entry := urlFetchEntry()
	// Override resolver to avoid actual DNS and return a loopback address
	orig := urlFetchResolver
	urlFetchResolver = &fakeURLFetchResolver{
		addrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}},
	}
	defer func() { urlFetchResolver = orig }()

	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-5",
		Name:      "url_fetch",
		Arguments: map[string]string{"url": "http://evil.example.com/steal"},
	}, ExecContext{})

	if !result.IsError {
		t.Error("expected error for loopback IP")
	}
	if !strings.Contains(result.Content, "private") {
		t.Errorf("expected 'private' in error, got: %s", result.Content)
	}
}

func TestURLFetch_RedirectToDisallowedHostBlocked(t *testing.T) {
	// Target server listens on 127.0.0.1; we'll reach it via "localhost" in the
	// redirect URL so that source ("127.0.0.1") and target ("localhost") have
	// distinct hostnames, letting the allowlist distinguish between them.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("should not reach here"))
	}))
	defer target.Close()

	targetURL, _ := url.Parse(target.URL)
	// Build redirect destination using "localhost" instead of "127.0.0.1".
	redirectDest := "http://localhost:" + targetURL.Port() + "/secret"
	targetHost := "localhost"

	// Source server redirects to the localhost-addressed target.
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectDest, http.StatusMovedPermanently)
	}))
	defer source.Close()

	sourceURL, _ := url.Parse(source.URL)
	sourceHost := sourceURL.Hostname() // "127.0.0.1"

	// Override urlFetchCheckHost to simulate allowlist behavior:
	// - source host ("127.0.0.1") is allowed → returns loopback IP
	// - target host ("localhost") is not in allowlist → blocked
	orig := urlFetchCheckHost
	urlFetchCheckHost = func(ctx context.Context, hostname string, allowlist []string, resolver hostcheck.Resolver) (net.IP, error) {
		if len(allowlist) > 0 && !hostcheck.IsAllowed(hostname, allowlist) {
			return nil, fmt.Errorf("host %q is not in allowed_hosts", hostname)
		}
		return net.ParseIP("127.0.0.1"), nil
	}
	defer func() { urlFetchCheckHost = orig }()

	entry := urlFetchEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-redirect",
		Name:      "url_fetch",
		Arguments: map[string]string{"url": source.URL + "/start"},
	}, ExecContext{AllowedHosts: []string{sourceHost}})

	// The redirect to "localhost" (not in allowlist) should be blocked.
	if !result.IsError {
		t.Errorf("expected error for redirect to disallowed host %s, got success: %s", targetHost, result.Content)
	}
	if !strings.Contains(result.Content, "not in allowed_hosts") {
		t.Errorf("expected 'not in allowed_hosts' in error, got: %s", result.Content)
	}
}

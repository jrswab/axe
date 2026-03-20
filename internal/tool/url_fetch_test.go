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

// newTestURLFetcher returns a urlFetcher that bypasses host validation,
// allowing tests to reach httptest.NewServer on 127.0.0.1.
func newTestURLFetcher() *urlFetcher {
	return &urlFetcher{
		resolver: net.DefaultResolver,
		checkHost: func(_ context.Context, _ string, _ []string, _ hostcheck.Resolver) (net.IP, error) {
			return net.ParseIP("127.0.0.1"), nil
		},
		timeout: 15 * time.Second,
	}
}

func TestURLFetch_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello world"))
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-success", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

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
	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-empty", Arguments: map[string]string{}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "url is required") {
		t.Errorf("Content = %q, want contains %q", result.Content, "url is required")
	}
}

func TestURLFetch_MissingURLArgument(t *testing.T) {
	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-missing", Arguments: map[string]string{"url": ""}}, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true")
	}
	if !strings.Contains(result.Content, "url is required") {
		t.Errorf("Content = %q, want contains %q", result.Content, "url is required")
	}
}

func TestURLFetch_UnsupportedScheme_File(t *testing.T) {
	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-file", Arguments: map[string]string{"url": "file:///etc/passwd"}}, ExecContext{})

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
	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-ftp", Arguments: map[string]string{"url": "ftp://example.com/file"}}, ExecContext{})

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
	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-noscheme", Arguments: map[string]string{"url": "example.com"}}, ExecContext{})

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

	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-404", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

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

	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-500", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

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

	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-large", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

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

	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-exact", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

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

	f := newTestURLFetcher()
	result := f.execute(ctx, provider.ToolCall{ID: "uf-timeout", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

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

	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-refused", Arguments: map[string]string{"url": "http://" + addr}}, ExecContext{})

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

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-unique-42", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

	if result.CallID != "uf-unique-42" {
		t.Errorf("CallID = %q, want %q", result.CallID, "uf-unique-42")
	}
}

func TestURLFetch_EmptyResponseBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-empty-body", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

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

	f := newTestURLFetcher()
	result := f.execute(context.Background(), provider.ToolCall{ID: "uf-err-large", Arguments: map[string]string{"url": ts.URL}}, ExecContext{})

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
	f := newTestURLFetcher()
	_ = f.execute(context.Background(), provider.ToolCall{ID: "uf-verbose", Arguments: map[string]string{"url": targetURL}}, ExecContext{Verbose: true, Stderr: &stderr})

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

	f := newTestURLFetcher()
	f.timeout = 100 * time.Millisecond
	call := provider.ToolCall{ID: "uf-per-req-timeout", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

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

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-parent-wins", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(ctx, call, ExecContext{})

	if !result.IsError {
		t.Fatal("expected IsError true when parent context fires first")
	}
}

func TestURLFetch_FastResponseUnaffectedByTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fast response"))
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-fast", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><script>x</script><p>Visible</p></body></html>"))
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-html-strip", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "Visible" {
		t.Errorf("Content = %q, want %q", result.Content, "Visible")
	}
}

func TestURLFetch_HTMLContentTypeWithCharset(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<p>Hello charset</p>"))
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-html-charset", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "Hello charset" {
		t.Errorf("Content = %q, want %q", result.Content, "Hello charset")
	}
}

func TestURLFetch_HTMLContentTypeCaseInsensitive(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "TEXT/HTML")
		_, _ = w.Write([]byte("<p>Case test</p>"))
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-html-case", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "Case test" {
		t.Errorf("Content = %q, want %q", result.Content, "Case test")
	}
}

func TestURLFetch_NonHTMLContentTypeNotStripped(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key": "<b>value</b>"}`))
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-json-no-strip", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	want := `{"key": "<b>value</b>"}`
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

func TestURLFetch_PlainTextNotStripped(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("<p>Not HTML</p>"))
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-plain-no-strip", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	if result.Content != "<p>Not HTML</p>" {
		t.Errorf("Content = %q, want %q", result.Content, "<p>Not HTML</p>")
	}
}

func TestURLFetch_MissingContentTypeNotStripped(t *testing.T) {
	// Note: Go's http.DetectContentType may auto-set Content-Type.
	// We explicitly set it to empty to test the missing header case.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "")
		_, _ = w.Write([]byte("<p>No header</p>"))
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-no-ct", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

	if result.IsError {
		t.Fatalf("expected IsError false, got true with content %q", result.Content)
	}
	// With empty Content-Type, body should be returned verbatim (no stripping)
	if result.Content != "<p>No header</p>" {
		t.Errorf("Content = %q, want %q", result.Content, "<p>No header</p>")
	}
}

func TestURLFetch_Non2xxHTMLStripped(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("<html><body><script>x</script><p>Not Found</p></body></html>"))
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-404-html", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "bad"}`))
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-500-json", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

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
	// Build HTML where raw size exceeds maxReadBytes but stripped text is small.
	// 9950 bytes of CSS in a <style> tag + small visible text.
	bigCSS := strings.Repeat("x", 9950)
	htmlBody := "<html><body><style>" + bigCSS + "</style><p>Short text</p></body></html>"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(htmlBody))
	}))
	defer ts.Close()

	f := newTestURLFetcher()
	call := provider.ToolCall{ID: "uf-strip-before-trunc", Arguments: map[string]string{"url": ts.URL}}
	result := f.execute(context.Background(), call, ExecContext{})

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

func TestURLFetch_AllowlistAndIPChecks(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) (f *urlFetcher, call provider.ToolCall, ec ExecContext, cleanup func())
		wantError    bool
		wantContains string
	}{
		{
			name: "allowlist permits matching host",
			setup: func(t *testing.T) (*urlFetcher, provider.ToolCall, ExecContext, func()) {
				t.Helper()
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					_, _ = w.Write([]byte("ok"))
				}))
				u, _ := url.Parse(ts.URL)
				hostname := u.Hostname()

				f := &urlFetcher{
					resolver: net.DefaultResolver,
					checkHost: func(_ context.Context, h string, allowlist []string, _ hostcheck.Resolver) (net.IP, error) {
						if !hostcheck.IsAllowed(h, allowlist) {
							return nil, &net.AddrError{Err: "not in allowed_hosts", Addr: h}
						}
						return net.ParseIP("127.0.0.1"), nil
					},
					timeout: 15 * time.Second,
				}
				call := provider.ToolCall{
					ID:        "test-allow-match",
					Name:      "url_fetch",
					Arguments: map[string]string{"url": ts.URL},
				}
				ec := ExecContext{AllowedHosts: []string{hostname}}
				return f, call, ec, ts.Close
			},
			wantError: false,
		},
		{
			name: "allowlist blocks non-matching host",
			setup: func(t *testing.T) (*urlFetcher, provider.ToolCall, ExecContext, func()) {
				t.Helper()
				f := newURLFetcher()
				call := provider.ToolCall{
					ID:        "test-allow-block",
					Name:      "url_fetch",
					Arguments: map[string]string{"url": "https://blocked.example.com/page"},
				}
				ec := ExecContext{AllowedHosts: []string{"allowed.example.com"}}
				return f, call, ec, nil
			},
			wantError:    true,
			wantContains: "not in allowed_hosts",
		},
		{
			name: "empty allowlist permits all",
			setup: func(t *testing.T) (*urlFetcher, provider.ToolCall, ExecContext, func()) {
				t.Helper()
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					_, _ = w.Write([]byte("ok"))
				}))
				f := newTestURLFetcher()
				call := provider.ToolCall{
					ID:        "test-empty-allow",
					Name:      "url_fetch",
					Arguments: map[string]string{"url": ts.URL},
				}
				return f, call, ExecContext{}, ts.Close
			},
			wantError: false,
		},
		{
			name: "blocks private IP",
			setup: func(t *testing.T) (*urlFetcher, provider.ToolCall, ExecContext, func()) {
				t.Helper()
				f := newURLFetcher()
				call := provider.ToolCall{
					ID:        "test-private-ip",
					Name:      "url_fetch",
					Arguments: map[string]string{"url": "http://192.168.1.1/data"},
				}
				return f, call, ExecContext{}, nil
			},
			wantError:    true,
			wantContains: "private",
		},
		{
			name: "blocks loopback via DNS",
			setup: func(t *testing.T) (*urlFetcher, provider.ToolCall, ExecContext, func()) {
				t.Helper()
				f := &urlFetcher{
					resolver: &fakeURLFetchResolver{
						addrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}},
					},
					checkHost: hostcheck.CheckHost,
					timeout:   15 * time.Second,
				}
				call := provider.ToolCall{
					ID:        "test-loopback",
					Name:      "url_fetch",
					Arguments: map[string]string{"url": "http://evil.example.com/steal"},
				}
				return f, call, ExecContext{}, nil
			},
			wantError:    true,
			wantContains: "private",
		},
		{
			name: "redirect to disallowed host blocked",
			setup: func(t *testing.T) (*urlFetcher, provider.ToolCall, ExecContext, func()) {
				t.Helper()
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					_, _ = w.Write([]byte("should not reach here"))
				}))
				targetURL, _ := url.Parse(target.URL)
				redirectDest := "http://localhost:" + targetURL.Port() + "/secret"

				source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, redirectDest, http.StatusMovedPermanently)
				}))
				sourceURL, _ := url.Parse(source.URL)
				sourceHost := sourceURL.Hostname()

				f := &urlFetcher{
					resolver: net.DefaultResolver,
					checkHost: func(_ context.Context, hostname string, allowlist []string, _ hostcheck.Resolver) (net.IP, error) {
						if len(allowlist) > 0 && !hostcheck.IsAllowed(hostname, allowlist) {
							return nil, fmt.Errorf("host %q is not in allowed_hosts", hostname)
						}
						return net.ParseIP("127.0.0.1"), nil
					},
					timeout: 15 * time.Second,
				}
				call := provider.ToolCall{
					ID:        "test-redirect",
					Name:      "url_fetch",
					Arguments: map[string]string{"url": source.URL + "/start"},
				}
				ec := ExecContext{AllowedHosts: []string{sourceHost}}
				cleanup := func() {
					source.Close()
					target.Close()
				}
				return f, call, ec, cleanup
			},
			wantError:    true,
			wantContains: "not in allowed_hosts",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, call, ec, cleanup := tc.setup(t)
			if cleanup != nil {
				defer cleanup()
			}

			result := f.execute(context.Background(), call, ec)

			if result.IsError != tc.wantError {
				t.Fatalf("IsError = %v, want %v; content: %s", result.IsError, tc.wantError, result.Content)
			}
			if tc.wantContains != "" && !strings.Contains(result.Content, tc.wantContains) {
				t.Errorf("Content = %q, want substring %q", result.Content, tc.wantContains)
			}
		})
	}
}

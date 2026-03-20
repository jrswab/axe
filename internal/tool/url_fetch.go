package tool

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/jrswab/axe/internal/hostcheck"
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/toolname"
)

const maxReadBytes = 10000

type urlFetcher struct {
	resolver  hostcheck.Resolver
	checkHost func(ctx context.Context, hostname string, allowlist []string, resolver hostcheck.Resolver) (net.IP, error)
	timeout   time.Duration
}

func newURLFetcher() *urlFetcher {
	return &urlFetcher{
		resolver:  net.DefaultResolver,
		checkHost: hostcheck.CheckHost,
		timeout:   15 * time.Second,
	}
}

func truncateURL(urlStr string, maxLen int) string {
	if len(urlStr) <= maxLen {
		return urlStr
	}
	return urlStr[:maxLen] + "..."
}

func sanitizeURL(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "invalid-url"
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "invalid-url"
	}

	path := parsedURL.EscapedPath()
	if path == "" {
		return parsedURL.Scheme + "://" + parsedURL.Host
	}

	return parsedURL.Scheme + "://" + parsedURL.Host + path
}

func urlFetchEntry() ToolEntry {
	f := newURLFetcher()
	return ToolEntry{
		Definition: urlFetchDefinition,
		Execute:    f.execute,
	}
}

func urlFetchDefinition() provider.Tool {
	return provider.Tool{
		Name:        toolname.URLFetch,
		Description: "Fetch content from a URL using HTTP GET and return the response body as text.",
		Parameters: map[string]provider.ToolParameter{
			"url": {
				Type:        "string",
				Description: "The URL to fetch.",
				Required:    true,
			},
		},
	}
}

func stripHTML(raw string) string {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return raw
	}

	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			b.WriteByte(' ')
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode {
			b.WriteByte(' ')
		}
	}
	walk(doc)

	return strings.Join(strings.Fields(b.String()), " ")
}

func (f *urlFetcher) execute(ctx context.Context, call provider.ToolCall, ec ExecContext) (result provider.ToolResult) {
	urlStr := call.Arguments["url"]
	statusCode := 0

	defer func() {
		safeURL := truncateURL(sanitizeURL(urlStr), 120)
		summary := fmt.Sprintf("url %q", safeURL)
		if statusCode != 0 {
			summary = fmt.Sprintf("url %q (HTTP %d)", safeURL, statusCode)
		} else if result.IsError {
			summary = fmt.Sprintf("%s: %s", summary, truncateURL(result.Content, 120))
		}
		toolVerboseLog(ec, toolname.URLFetch, result, summary)
	}()

	if urlStr == "" {
		return provider.ToolResult{CallID: call.ID, Content: "url is required", IsError: true}
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("unsupported scheme %q: only http and https are allowed", parsedURL.Scheme),
			IsError: true,
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	// Host allowlist and private IP check.
	_, hostErr := f.checkHost(reqCtx, parsedURL.Hostname(), ec.AllowedHosts, f.resolver)
	if hostErr != nil {
		return provider.ToolResult{CallID: call.ID, Content: hostErr.Error(), IsError: true}
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, urlStr, nil)
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}

	client := &http.Client{
		Timeout: f.timeout,
		Transport: &http.Transport{
			DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
				host, port, splitErr := net.SplitHostPort(addr)
				if splitErr != nil {
					return nil, splitErr
				}
				dialIP, checkErr := f.checkHost(dialCtx, host, ec.AllowedHosts, f.resolver)
				if checkErr != nil {
					return nil, checkErr
				}
				if dialIP != nil && port != "" {
					return (&net.Dialer{}).DialContext(dialCtx, network, net.JoinHostPort(dialIP.String(), port))
				}
				return (&net.Dialer{}).DialContext(dialCtx, network, addr)
			},
		},
		CheckRedirect: func(redirectReq *http.Request, via []*http.Request) error {
			if len(ec.AllowedHosts) > 0 && !hostcheck.IsAllowed(redirectReq.URL.Hostname(), ec.AllowedHosts) {
				return fmt.Errorf("host %q is not in allowed_hosts", redirectReq.URL.Hostname())
			}
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	statusCode = resp.StatusCode

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReadBytes+1))
	if err != nil {
		return provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
	}

	bodyStr := string(body)

	// Strip HTML if Content-Type is text/html
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType == "text/html" {
		bodyStr = stripHTML(bodyStr)
	}

	if len(bodyStr) > maxReadBytes {
		bodyStr = bodyStr[:maxReadBytes] + "\n... [response truncated, exceeded 10000 characters]"
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, bodyStr),
			IsError: true,
		}
	}

	return provider.ToolResult{CallID: call.ID, Content: bodyStr, IsError: false}
}

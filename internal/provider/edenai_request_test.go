package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestEdenAIRequests(t *testing.T) {
	transports := []struct {
		name string
		tls  bool
	}{
		{name: "HTTP loopback"},
		{name: "HTTPS", tls: true},
	}
	methods := []struct {
		name   string
		stream bool
	}{
		{name: "Send"},
		{name: "SendStream", stream: true},
	}
	for _, transport := range transports {
		for _, method := range methods {
			t.Run(transport.name+"/"+method.name, func(t *testing.T) {
				var requests atomic.Int32
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if r.Method != http.MethodPost || r.URL.RequestURI() != "/v3/chat/completions" {
						t.Errorf("request = %s %s, want POST /v3/chat/completions", r.Method, r.URL.RequestURI())
					}
					if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
						t.Errorf("Authorization = %q, want Bearer test-key", got)
					}
					var body struct {
						Model  string `json:"model"`
						Stream bool   `json:"stream"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("decode request: %v", err)
					} else if body.Model != "vendor/model" || body.Stream != method.stream {
						t.Errorf("request model = %q, stream = %t; want vendor/model, %t", body.Model, body.Stream, method.stream)
					}
					if method.stream {
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n")
						return
					}
					_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"hello"}}]}`)
				})
				var server *httptest.Server
				if transport.tls {
					server = httptest.NewTLSServer(handler)
				} else {
					server = httptest.NewServer(handler)
				}
				defer server.Close()

				p, err := NewEdenAI("test-key", server.URL+"/v3/")
				if err != nil {
					t.Fatalf("NewEdenAI: %v", err)
				}
				if transport.tls {
					p.client = server.Client()
				}
				req := &Request{Model: "vendor/model", Messages: []Message{{Role: "user", Content: "Hi"}}}
				if method.stream {
					stream, err := p.SendStream(context.Background(), req)
					if err != nil {
						t.Fatalf("SendStream: %v", err)
					}
					defer func() { _ = stream.Close() }()
					want := []StreamEvent{{Type: StreamEventText, Text: "hello"}, {Type: StreamEventDone}}
					if got := drainStream(t, stream); !reflect.DeepEqual(got, want) {
						t.Errorf("events = %+v, want %+v", got, want)
					}
				} else {
					resp, err := p.Send(context.Background(), req)
					if err != nil {
						t.Fatalf("Send: %v", err)
					}
					if resp.Content != "hello" {
						t.Errorf("content = %q, want hello", resp.Content)
					}
				}
				if got := requests.Load(); got != 1 {
					t.Errorf("requests = %d, want 1", got)
				}
			})
		}
	}
}

func TestEdenAIRequestsRejectRedirects(t *testing.T) {
	for _, method := range []struct {
		name   string
		stream bool
	}{
		{name: "Send"},
		{name: "SendStream", stream: true},
	} {
		t.Run(method.name, func(t *testing.T) {
			var sourceRequests, targetRequests atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				targetRequests.Add(1)
				_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"unexpected"}}]}`)
			}))
			defer target.Close()
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sourceRequests.Add(1)
				http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
			}))
			defer source.Close()

			p, err := NewEdenAI("test-key", source.URL+"/v3/")
			if err != nil {
				t.Fatalf("NewEdenAI: %v", err)
			}
			req := &Request{Model: "vendor/model"}
			if method.stream {
				var stream *EventStream
				stream, err = p.SendStream(context.Background(), req)
				if stream != nil {
					defer func() { _ = stream.Close() }()
				}
			} else {
				_, err = p.Send(context.Background(), req)
			}
			if got := sourceRequests.Load(); got != 1 {
				t.Errorf("source requests = %d, want 1", got)
			}
			if got := targetRequests.Load(); got != 0 {
				t.Errorf("redirect target requests = %d, want 0", got)
			}
			var providerErr *ProviderError
			if !errors.As(err, &providerErr) {
				t.Fatalf("error = %v, want ProviderError", err)
			}
			if providerErr.Category != ErrCategoryServer || providerErr.Status != http.StatusTemporaryRedirect {
				t.Errorf("error = %+v, want server error with status 307", providerErr)
			}
		})
	}
}

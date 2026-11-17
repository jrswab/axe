package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jrswab/axe/internal/testutil"
)

func TestIntegration_EdenAI(t *testing.T) {
	const model = "openai/gpt-4o-mini"

	for _, tt := range []struct {
		name      string
		stream    bool
		envConfig bool
	}{
		{name: "config nonstream"},
		{name: "config stream", stream: true},
		{name: "environment overrides config nonstream", envConfig: true},
		{name: "environment overrides config stream", stream: true, envConfig: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetRunCmd(t)
			configDir, _ := testutil.SetupXDGDirs(t)
			writeAgentConfig(t, configDir, "edenai-integration", fmt.Sprintf(`name = "edenai-integration"
model = "edenai/%s"
workdir = %q
[params]
temperature = 0
`, model, t.TempDir()))

			wantKey := "config-key"
			if tt.envConfig {
				wantKey = "env-key"
			}
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if n := requests.Add(1); n != 1 {
					t.Errorf("unexpected request %d", n)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				if r.Method != http.MethodPost || r.URL.Path != "/v3/chat/completions" {
					t.Errorf("request = %s %s, want POST /v3/chat/completions", r.Method, r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer "+wantKey {
					t.Errorf("Authorization = %q, want Bearer %s", got, wantKey)
				}
				var body struct {
					Model       string   `json:"model"`
					Stream      bool     `json:"stream"`
					Temperature *float64 `json:"temperature"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode request: %v", err)
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				if body.Model != model || body.Stream != tt.stream {
					t.Errorf("model/stream = %q/%t, want %q/%t", body.Model, body.Stream, model, tt.stream)
				}
				if body.Temperature == nil || *body.Temperature != 0 {
					t.Errorf("temperature = %v, want explicit 0", body.Temperature)
				}
				if tt.stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"pong"}}]}`+"\n\n"+
						`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n"+
						`data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":4}}`+"\n\ndata: [DONE]\n\n")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"model":%q,"choices":[{"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":4}}`, model)
			}))
			defer server.Close()

			// Prevent even a provider routing regression from reaching a live API.
			transport := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if addr != server.Listener.Addr().String() {
					return nil, fmt.Errorf("unexpected network address %q", addr)
				}
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			}}
			originalTransport := http.DefaultTransport
			http.DefaultTransport = transport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
				transport.CloseIdleConnections()
			})

			configURL := server.URL + "/v3"
			t.Setenv("EDENAI_API_KEY", "")
			t.Setenv("AXE_EDENAI_BASE_URL", "")
			if tt.envConfig {
				configURL = "http://127.0.0.1:0/v3"
				t.Setenv("EDENAI_API_KEY", wantKey)
				t.Setenv("AXE_EDENAI_BASE_URL", server.URL+"/v3/")
			}
			config := fmt.Sprintf("[providers.edenai]\napi_key = %q\nbase_url = %q\n", "config-key", configURL)
			if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}

			stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
			rootCmd.SetIn(strings.NewReader(""))
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			args := []string{"run", "edenai-integration", "--json", "-p", "ping"}
			if tt.stream {
				args = append(args, "--stream")
			}
			rootCmd.SetArgs(args)
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("execute: %v\nstderr: %s", err, stderr.String())
			}
			if got := requests.Load(); got != 1 {
				t.Errorf("requests = %d, want 1", got)
			}

			var got struct {
				Content      string `json:"content"`
				Model        string `json:"model"`
				StopReason   string `json:"stop_reason"`
				InputTokens  int    `json:"input_tokens"`
				OutputTokens int    `json:"output_tokens"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON output: %v\nstdout: %s", err, stdout.String())
			}
			if got.Content != "pong" || got.Model != model || got.StopReason != "stop" {
				t.Errorf("content/model/stop = %q/%q/%q, want pong/%s/stop", got.Content, got.Model, got.StopReason, model)
			}
			if got.InputTokens != 10 || got.OutputTokens != 4 {
				t.Errorf("input/output tokens = %d/%d, want 10/4", got.InputTokens, got.OutputTokens)
			}
		})
	}
}

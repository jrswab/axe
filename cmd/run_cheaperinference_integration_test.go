package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// @anchor:provider:integration Exercise Cheaper Inference through CLI configuration and a local compatible API.
func TestIntegration_CheaperInference(t *testing.T) {
	const model = "meta-llama/Llama-3.3-70B-Instruct"

	for _, tt := range []struct {
		name      string
		stream    bool
		envConfig bool
	}{
		{name: "canonical environment key and endpoint", envConfig: true},
		{name: "canonical environment key and endpoint stream", envConfig: true, stream: true},
		{name: "config nonstream"},
		{name: "config stream", stream: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetRunCmd(t)
			configDir, _ := testutil.SetupXDGDirs(t)
			writeAgentConfig(t, configDir, "cheaperinference-integration", fmt.Sprintf(`name = "cheaperinference-integration"
model = "cheaperinference/%s"
workdir = %q
`, model, t.TempDir()))

			wantKey := "config-key"
			if tt.envConfig {
				wantKey = "canonical-env-key"
			}
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if n := requests.Add(1); n != 1 {
					t.Errorf("unexpected request %d", n)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
					t.Errorf("request = %s %s, want POST /v1/chat/completions", r.Method, r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer "+wantKey {
					t.Errorf("Authorization = %q, want Bearer %s", got, wantKey)
				}
				var body struct {
					Model  string `json:"model"`
					Stream bool   `json:"stream"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode request: %v", err)
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				if body.Model != model || body.Stream != tt.stream {
					t.Errorf("model/stream = %q/%t, want %q/%t", body.Model, body.Stream, model, tt.stream)
				}
				if tt.stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"cheaper "}}]}`+"\n\n"+
						`data: {"choices":[{"delta":{"content":"pong"}}]}`+"\n\n"+
						`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n"+
						`data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":4}}`+"\n\ndata: [DONE]\n\n")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"model":%q,"choices":[{"message":{"role":"assistant","content":"cheaper pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":4}}`, model)
			}))
			defer server.Close()
			restrictCheaperInferenceNetwork(t, server.Listener.Addr().String())

			configURL := server.URL + "/v1"
			t.Setenv("CHEAPER_INFERENCE_API_KEY", "")
			// Never let the noncanonical spelling satisfy the credential requirement.
			t.Setenv("CHEAPERINFERENCE_API_KEY", "")
			t.Setenv("AXE_CHEAPERINFERENCE_BASE_URL", "")
			if tt.envConfig {
				configURL = server.URL + "/wrong"
				t.Setenv("CHEAPER_INFERENCE_API_KEY", wantKey)
				t.Setenv("AXE_CHEAPERINFERENCE_BASE_URL", server.URL+"/v1")
			}
			config := fmt.Sprintf("[providers.cheaperinference]\napi_key = %q\nbase_url = %q\n", "config-key", configURL)
			if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}

			stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
			rootCmd.SetIn(strings.NewReader(""))
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			args := []string{"run", "cheaperinference-integration", "--json", "-p", "ping"}
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
			if got.Content != "cheaper pong" || got.Model != model || got.StopReason != "stop" {
				t.Errorf("content/model/stop = %q/%q/%q, want cheaper pong/%s/stop", got.Content, got.Model, got.StopReason, model)
			}
			if got.InputTokens != 10 || got.OutputTokens != 4 {
				t.Errorf("input/output tokens = %d/%d, want 10/4", got.InputTokens, got.OutputTokens)
			}
		})
	}
}

func TestIntegration_CheaperInferenceMissingKey(t *testing.T) {
	resetRunCmd(t)
	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "cheaperinference-missing-key", fmt.Sprintf(`name = "cheaperinference-missing-key"
model = "cheaperinference/meta-llama/Llama-3.3-70B-Instruct"
workdir = %q
`, t.TempDir()))
	t.Setenv("CHEAPER_INFERENCE_API_KEY", "")
	t.Setenv("CHEAPERINFERENCE_API_KEY", "")
	t.Setenv("AXE_CHEAPERINFERENCE_BASE_URL", "")
	restrictCheaperInferenceNetwork(t, "")

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{"run", "cheaperinference-missing-key", "-p", "ping"})

	err := rootCmd.Execute()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected configuration ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 2 {
		t.Errorf("exit code = %d, want 2 (configuration error)", exitErr.Code)
	}
	for _, hint := range []string{"CHEAPER_INFERENCE_API_KEY", "config.toml"} {
		if !strings.Contains(err.Error(), hint) {
			t.Errorf("error = %q, want actionable hint %q", err, hint)
		}
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty output on configuration failure", stdout.String())
	}
}

// restrictCheaperInferenceNetwork prevents a routing regression from contacting a live API.
// An empty address denies all network access, including when testing configuration failures.
func restrictCheaperInferenceNetwork(t *testing.T, allowedAddr string) {
	t.Helper()
	transport := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		if allowedAddr == "" || addr != allowedAddr {
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
}

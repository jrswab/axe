package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/jrswab/axe/internal/testutil"
)

func TestIntegration_LiteLLM_JSONWithTool(t *testing.T) {
	const model = "openai/team/model-alias"
	const toolOutput = "1: hello from fixture"

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
			workdir := t.TempDir()
			if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("hello from fixture\n"), 0644); err != nil {
				t.Fatal(err)
			}
			writeAgentConfig(t, configDir, "litellm-test", fmt.Sprintf(`name = "litellm-test"
model = "litellm/%s"
tools = ["read_file"]
workdir = %q
`, model, workdir))

			wantKey := "config-key"
			if tt.envConfig {
				wantKey = "env-key"
			}
			var mu sync.Mutex
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				requests++
				turn := requests
				if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
					t.Errorf("request %d: %s %s, want POST /v1/chat/completions", turn, r.Method, r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer "+wantKey {
					t.Errorf("request %d: Authorization = %q, want Bearer %s", turn, got, wantKey)
				}
				var body struct {
					Model    string `json:"model"`
					Stream   bool   `json:"stream"`
					Messages []struct {
						Role       string `json:"role"`
						Content    string `json:"content"`
						ToolCallID string `json:"tool_call_id"`
						ToolCalls  []struct {
							ID       string `json:"id"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"messages"`
					Tools []struct {
						Function struct {
							Name string `json:"name"`
						} `json:"function"`
					} `json:"tools"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("request %d: decode: %v", turn, err)
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				if body.Model != model || body.Stream != tt.stream {
					t.Errorf("request %d: model/stream = %q/%t, want %q/%t", turn, body.Model, body.Stream, model, tt.stream)
				}
				if len(body.Tools) != 1 || body.Tools[0].Function.Name != "read_file" {
					t.Errorf("request %d: tools = %+v, want read_file", turn, body.Tools)
				}
				if turn == 2 {
					if len(body.Messages) < 2 {
						t.Errorf("request 2: missing tool conversation: %+v", body.Messages)
					} else {
						assistant := body.Messages[len(body.Messages)-2]
						result := body.Messages[len(body.Messages)-1]
						if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 {
							t.Errorf("request 2: assistant tool calls = %+v", assistant)
						} else {
							call := assistant.ToolCalls[0]
							var args map[string]string
							if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil || call.ID != "call_1" || call.Function.Name != "read_file" || !reflect.DeepEqual(args, map[string]string{"path": "fixture.txt"}) {
								t.Errorf("request 2: replayed call = %+v, args = %v, error = %v", call, args, err)
							}
						}
						if result.Role != "tool" || result.ToolCallID != "call_1" || result.Content != toolOutput {
							t.Errorf("request 2: tool result = %+v", result)
						}
					}
				}
				if turn > 2 {
					t.Errorf("unexpected request %d", turn)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}

				cost, inputTokens, outputTokens := "0.125", 10, 4
				if turn == 2 {
					cost, inputTokens, outputTokens = "0.25", 20, 6
				}
				w.Header().Set("x-litellm-response-cost", cost)
				if tt.stream {
					w.Header().Set("Content-Type", "text/event-stream")
					if turn == 1 {
						_, _ = fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"fixture.txt\"}"}}]},"finish_reason":"tool_calls"}]}`+"\n\n")
					} else {
						_, _ = fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"File read"}}]}`+"\n\n"+
							`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
					}
					_, _ = fmt.Fprintf(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d}}\n\ndata: [DONE]\n\n", inputTokens, outputTokens)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if turn == 1 {
					_, _ = fmt.Fprintf(w, `{"model":%q,"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"fixture.txt\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":%d,"completion_tokens":%d}}`, model, inputTokens, outputTokens)
				} else {
					_, _ = fmt.Fprintf(w, `{"model":%q,"choices":[{"message":{"role":"assistant","content":"File read"},"finish_reason":"stop"}],"usage":{"prompt_tokens":%d,"completion_tokens":%d}}`, model, inputTokens, outputTokens)
				}
			}))
			defer server.Close()

			configURL := server.URL
			t.Setenv("LITELLM_API_KEY", "")
			t.Setenv("AXE_LITELLM_BASE_URL", "")
			if tt.envConfig {
				configURL = "http://127.0.0.1:0"
				t.Setenv("LITELLM_API_KEY", wantKey)
				t.Setenv("AXE_LITELLM_BASE_URL", server.URL+"/v1/")
			}
			config := fmt.Sprintf("[providers.litellm]\napi_key = %q\nbase_url = %q\n", "config-key", configURL)
			if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}

			stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			args := []string{"run", "litellm-test", "--json", "-p", "Read the file"}
			if tt.stream {
				args = append(args, "--stream")
			}
			rootCmd.SetArgs(args)
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("execute: %v\nstderr: %s", err, stderr.String())
			}
			mu.Lock()
			gotRequests := requests
			mu.Unlock()
			if gotRequests != 2 {
				t.Errorf("requests = %d, want 2", gotRequests)
			}

			var got struct {
				Content         string  `json:"content"`
				Model           string  `json:"model"`
				Cost            float64 `json:"cost"`
				InputTokens     int     `json:"input_tokens"`
				OutputTokens    int     `json:"output_tokens"`
				StopReason      string  `json:"stop_reason"`
				ToolCalls       int     `json:"tool_calls"`
				ToolCallDetails []struct {
					Name    string            `json:"name"`
					Input   map[string]string `json:"input"`
					Output  string            `json:"output"`
					IsError bool              `json:"is_error"`
					Turn    int               `json:"turn"`
				} `json:"tool_call_details"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON output: %v\nstdout: %s", err, stdout.String())
			}
			if got.Content != "File read" || got.Model != model || got.StopReason != "stop" {
				t.Errorf("content/model/stop = %q/%q/%q, want File read/%s/stop", got.Content, got.Model, got.StopReason, model)
			}
			// Cost follows the existing result contract: final response only,
			// while token usage accumulates over the full conversation.
			if got.Cost != 0.25 || got.InputTokens != 30 || got.OutputTokens != 10 {
				t.Errorf("cost/input/output = %v/%d/%d, want 0.25/30/10", got.Cost, got.InputTokens, got.OutputTokens)
			}
			if got.ToolCalls != 1 || len(got.ToolCallDetails) != 1 {
				t.Fatalf("tool calls/details = %d/%+v, want one", got.ToolCalls, got.ToolCallDetails)
			}
			detail := got.ToolCallDetails[0]
			if detail.Name != "read_file" || !reflect.DeepEqual(detail.Input, map[string]string{"path": "fixture.txt"}) || detail.Output != toolOutput || detail.IsError || detail.Turn != 0 {
				t.Errorf("tool detail = %+v", detail)
			}
		})
	}
}

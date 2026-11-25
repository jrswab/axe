package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrswab/axe/internal/testutil"
)

// Literal provider-shaped usage fixtures. Anthropic usage fields follow
// https://platform.claude.com/docs/en/api/messages; OpenAI Chat cached reads
// follow usage.prompt_tokens_details.cached_tokens in
// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create.md
// and its streaming usage chunk. Counts are provider reports, not cache hits.
const (
	vcAnthropicTool  = `{"id":"msg_tool","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[{"type":"tool_use","id":"list-usage","name":"list_directory","input":{"path":"."}}],"stop_reason":"tool_use","usage":{"input_tokens":20,"output_tokens":10,"cache_read_input_tokens":150,"cache_creation_input_tokens":50}}`
	vcAnthropicFinal = `{"id":"msg_final","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[{"type":"text","text":"Done"}],"stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":4,"cache_read_input_tokens":30,"cache_creation_input_tokens":10}}`

	vcStreamTool = `event: message_start
 data: {"type":"message_start","message":{"id":"msg_tool","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[],"usage":{"input_tokens":20,"output_tokens":0,"cache_read_input_tokens":150,"cache_creation_input_tokens":50}}}

 event: content_block_start
 data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"list-usage","name":"list_directory","input":{}}}

 event: content_block_delta
 data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\".\"}"}}

 event: content_block_stop
 data: {"type":"content_block_stop","index":0}

 event: message_delta
 data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":10}}

 event: message_stop
 data: {"type":"message_stop"}

`
	vcStreamStart = `event: message_start
 data: {"type":"message_start","message":{"id":"msg_final","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[],"usage":{"input_tokens":8,"output_tokens":0,"cache_read_input_tokens":30,"cache_creation_input_tokens":10}}}

`
	vcStreamZeroStart = `event: message_start
 data: {"type":"message_start","message":{"usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}

`
	vcStreamText = `event: content_block_start
 data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

 event: content_block_delta
 data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Done"}}

 event: content_block_stop
 data: {"type":"content_block_stop","index":0}

`
	vcStreamDone = `event: message_delta
 data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}

`
	vcStreamZeroDone = `event: message_delta
 data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":0}}

`
	vcStreamStop = `event: message_stop
 data: {"type":"message_stop"}

`
	vcStreamError = `event: error
 data: {"type":"error","error":{"type":"api_error","message":"verbose fixture interrupted"}}

`

	vcOpenAIFinal   = `{"model":"gpt-4o","choices":[{"message":{"content":"Done"},"finish_reason":"stop"}],"usage":{"prompt_tokens":40,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":32}}}`
	vcOpenAITool    = `{"model":"gpt-4o","choices":[{"message":{"content":null,"tool_calls":[{"id":"list-usage","type":"function","function":{"name":"list_directory","arguments":"{\"path\":\".\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":30,"completion_tokens":6,"prompt_tokens_details":{"cached_tokens":16}}}`
	vcOpenAILoopEnd = `{"model":"gpt-4o","choices":[{"message":{"content":"Done"},"finish_reason":"stop"}],"usage":{"prompt_tokens":50,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":24}}}`

	vcOpenAIStreamText = `data: {"model":"gpt-4o","choices":[{"delta":{"content":"Done"},"index":0}]}

data: {"model":"gpt-4o","choices":[{"delta":{},"index":0,"finish_reason":"stop"}]}

`
	vcOpenAIStreamUsage = `data: {"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":40,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":32}}}

`
	vcOpenAIStreamEnd = "data: [DONE]\n\n"
)

func vcNormal(body string) testutil.MockLLMResponse {
	return testutil.MockLLMResponse{StatusCode: 200, Body: body}
}

// vcSSE strips the literal indentation from SSE lines without synthesizing usage.
func vcSSE(parts ...string) testutil.MockLLMResponse {
	return testutil.MockLLMResponse{StatusCode: 200, ContentType: "text/event-stream", Body: strings.ReplaceAll(strings.Join(parts, ""), "\n ", "\n")}
}

func vcAnthropicNormal(read, write int) testutil.MockLLMResponse {
	return vcNormal(fmt.Sprintf(`{"content":[{"type":"text","text":"Done"}],"stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":4,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}`, read, write))
}

type verboseCacheCase struct {
	name                  string
	provider              string // "anthropic" or "openai"; Bedrock and delegation have their own rows
	stream, tools, noJSON bool
	quiet                 bool // Verbose: false
	budget                int
	responses             []testutil.MockLLMResponse
	read, write           int
	wantCache             string // exact Cache: line, or "" for none
	wantError             bool
}

func TestRun_VerboseCacheCounts(t *testing.T) {
	// Subtests stay sequential: base URLs use t.Setenv, and the Bedrock row
	// replaces the process-wide http.DefaultTransport through parityIntercept.
	cases := []verboseCacheCase{
		{name: "anthropic_single_normal", provider: "anthropic", responses: []testutil.MockLLMResponse{vcNormal(vcAnthropicFinal)},
			read: 30, write: 10, wantCache: "Cache:    30 read, 10 written\n"},
		{name: "anthropic_single_stream", provider: "anthropic", stream: true, responses: []testutil.MockLLMResponse{vcSSE(vcStreamStart, vcStreamText, vcStreamDone, vcStreamStop)},
			read: 30, write: 10, wantCache: "Cache:    30 read, 10 written\n"},
		{name: "anthropic_loop_normal", provider: "anthropic", tools: true, responses: []testutil.MockLLMResponse{vcNormal(vcAnthropicTool), vcNormal(vcAnthropicFinal)},
			read: 180, write: 60, wantCache: "Cache:    180 read, 60 written\n"},
		{name: "anthropic_loop_stream", provider: "anthropic", stream: true, tools: true, responses: []testutil.MockLLMResponse{vcSSE(vcStreamTool), vcSSE(vcStreamStart, vcStreamText, vcStreamDone, vcStreamStop)},
			read: 180, write: 60, wantCache: "Cache:    180 read, 60 written\n"},
		{name: "openai_single_normal_reads_only", provider: "openai", responses: []testutil.MockLLMResponse{vcNormal(vcOpenAIFinal)},
			read: 32, wantCache: "Cache:    32 read\n"},
		{name: "openai_single_stream_reads_only", provider: "openai", stream: true, responses: []testutil.MockLLMResponse{vcSSE(vcOpenAIStreamText, vcOpenAIStreamUsage, vcOpenAIStreamEnd)},
			read: 32, wantCache: "Cache:    32 read\n"},
		{name: "openai_loop_reads_only", provider: "openai", tools: true, responses: []testutil.MockLLMResponse{vcNormal(vcOpenAITool), vcNormal(vcOpenAILoopEnd)},
			read: 40, wantCache: "Cache:    40 read\n"},
		{name: "anthropic_write_only", provider: "anthropic", responses: []testutil.MockLLMResponse{vcAnthropicNormal(0, 10)},
			write: 10, wantCache: "Cache:    10 written\n"},
		{name: "zero_normal", provider: "anthropic", responses: []testutil.MockLLMResponse{vcAnthropicNormal(0, 0)}},
		{name: "zero_stream", provider: "anthropic", stream: true, responses: []testutil.MockLLMResponse{vcSSE(vcStreamZeroStart, vcStreamText, vcStreamZeroDone, vcStreamStop)}},
		{name: "absent_normal", provider: "anthropic", responses: []testutil.MockLLMResponse{vcNormal(`{"content":[{"type":"text","text":"Done"}],"stop_reason":"end_turn"}`)}},
		{name: "absent_stream_without_usage_chunk", provider: "openai", stream: true, responses: []testutil.MockLLMResponse{vcSSE(vcOpenAIStreamText, vcOpenAIStreamEnd)}},
		// Preservation (spec R2).
		{name: "budget_line_unchanged_with_zero", provider: "anthropic", budget: 100, responses: []testutil.MockLLMResponse{vcAnthropicNormal(30, 0)},
			read: 30, wantCache: "Cache:    30 read, 0 written\n"},
		{name: "not_verbose", provider: "anthropic", quiet: true, responses: []testutil.MockLLMResponse{vcNormal(vcAnthropicFinal)}, read: 30, write: 10},
		{name: "stream_error_after_partial_output", provider: "anthropic", stream: true, noJSON: true, wantError: true,
			responses: []testutil.MockLLMResponse{vcSSE(vcStreamStart, vcStreamText, vcStreamError)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { runVerboseCacheCase(t, tc) })
	}
	t.Run("bedrock_single_read_and_write", verboseCacheBedrock)
	t.Run("nested_counts_stay_opaque", verboseCacheNested)
}

func runVerboseCacheCase(t *testing.T, tc verboseCacheCase) {
	t.Helper()
	model, keyEnv, urlEnv := "anthropic/claude-sonnet-4-20250514", "ANTHROPIC_API_KEY", "AXE_ANTHROPIC_BASE_URL"
	if tc.provider == "openai" {
		model, keyEnv, urlEnv = "openai/gpt-4o", "OPENAI_API_KEY", "AXE_OPENAI_BASE_URL"
	}
	definition := fmt.Sprintf("name = \"verbose-cache\"\nmodel = %q\n", model)
	if tc.tools {
		definition += "tools = [\"list_directory\"]\n"
	}
	if tc.budget > 0 {
		definition += fmt.Sprintf("[budget]\nmax_tokens = %d\n", tc.budget)
	}
	setupTestEnv(t, "", "verbose-cache", definition)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv(keyEnv, "local-test-key")
	server := testutil.NewMockLLMServer(t, tc.responses)
	t.Setenv(urlEnv, server.URL())
	stdout, stderr := newMockStdoutStderr()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := Run(ctx, Options{
		AgentName: "verbose-cache", Workdir: t.TempDir(), Prompt: "Inspect then finish.",
		Stdin: strings.NewReader(""), Stdout: stdout, Stderr: stderr,
		JSON: !tc.noJSON, Stream: tc.stream, Verbose: !tc.quiet,
	})

	if tc.wantError {
		if err == nil || !strings.Contains(err.Error(), "verbose fixture interrupted") {
			t.Fatalf("Run() error = %v, want the original stream error", err)
		}
		if !strings.Contains(stdout.String(), "Done") {
			t.Errorf("stdout = %q, want the partial streamed text", stdout)
		}
		if strings.Contains(stderr.String(), "Tokens:") || strings.Contains(stderr.String(), "Cache:") {
			t.Errorf("failed Run printed usage: stderr = %q", stderr)
		}
		return
	}
	if err != nil {
		t.Fatalf("Run() error = %v; stderr = %s", err, stderr)
	}
	if result.CacheReadTokens != tc.read || result.CacheWriteTokens != tc.write {
		t.Errorf("Result cache read/write = %d/%d, want %d/%d", result.CacheReadTokens, result.CacheWriteTokens, tc.read, tc.write)
	}
	verboseCacheJSON(t, stdout.Bytes(), tc.read, tc.write)

	if tc.quiet {
		if strings.Contains(stderr.String(), "Cache:") {
			t.Errorf("non-verbose stderr = %q, want no Cache: text", stderr)
		}
		return
	}
	tokens := fmt.Sprintf("Tokens:   %d input, %d output\n", result.InputTokens, result.OutputTokens)
	switch {
	case tc.budget > 0:
		used := result.InputTokens + result.OutputTokens
		if result.Budget.Used != used {
			t.Errorf("budget used = %d, want input+output %d only", result.Budget.Used, used)
		}
		var output map[string]json.RawMessage
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil || string(output["budget_used_tokens"]) != fmt.Sprint(used) {
			t.Errorf("JSON budget_used_tokens = %s (%v), want %d", output["budget_used_tokens"], err, used)
		}
		tokens = fmt.Sprintf("Tokens:   %d input, %d output (cumulative, budget: %d/%d)\n", result.InputTokens, result.OutputTokens, used, tc.budget)
	case tc.tools:
		tokens = fmt.Sprintf("Tokens:   %d input, %d output (cumulative)\n", result.InputTokens, result.OutputTokens)
	}
	verboseCacheSummary(t, stderr.String(), tokens, tc.wantCache)
}

// verboseCacheSummary requires the Tokens:, optional Cache: and Stop: lines to be adjacent and exact.
func verboseCacheSummary(t *testing.T, stderr, tokens, cache string) {
	t.Helper()
	if want := tokens + cache + "Stop:     "; !strings.Contains(stderr, want) {
		t.Errorf("verbose stderr = %q; want consecutive %q", stderr, want)
	}
	if cache == "" && strings.Contains(stderr, "Cache:") {
		t.Errorf("verbose stderr = %q; want no Cache: line", stderr)
	}
}

// verboseCacheJSON requires JSON cache fields to equal the verbose counts and stay omitted when zero.
func verboseCacheJSON(t *testing.T, stdout []byte, read, write int) {
	t.Helper()
	var output map[string]json.RawMessage
	if err := json.Unmarshal(stdout, &output); err != nil {
		t.Fatalf("JSON stdout = %q: %v", stdout, err)
	}
	for key, want := range map[string]int{"cache_read_tokens": read, "cache_write_tokens": write} {
		raw, present := output[key]
		if want == 0 {
			if present {
				t.Errorf("zero %s must stay omitted, got %s", key, raw)
			}
		} else if !present || string(raw) != fmt.Sprint(want) {
			t.Errorf("JSON %s = %s (present %t), want %d", key, raw, present, want)
		}
	}
}

func verboseCacheBedrock(t *testing.T) {
	f := newParityFixture(t)
	f.agent(t, "main", "Static verbose usage.", "")
	exchanges := parityIntercept(t, parityRegion, parityEnvCredentials, map[string][]parityReplyFunc{
		"main": {parityHTTP(200, parityReply+`,"usage":{"inputTokens":8,"outputTokens":3,"cacheReadInputTokens":7,"cacheWriteInputTokens":11}}`)},
	})
	var stdout, stderr bytes.Buffer
	opts := parityMainOptions(&stdout)
	opts.Stderr, opts.Verbose, opts.JSON = &stderr, true, true
	result, err := Run(context.Background(), opts)
	parityCount(t, exchanges, 1, err)
	if err != nil {
		t.Fatalf("Run() error = %v; stderr = %s", err, stderr.String())
	}
	if result.CacheReadTokens != 7 || result.CacheWriteTokens != 11 {
		t.Errorf("Result cache read/write = %d/%d, want 7/11", result.CacheReadTokens, result.CacheWriteTokens)
	}
	verboseCacheJSON(t, stdout.Bytes(), 7, 11)
	verboseCacheSummary(t, stderr.String(), "Tokens:   8 input, 3 output\n", "Cache:    7 read, 11 written\n")
}

func verboseCacheNested(t *testing.T) {
	configDir := setupTestEnv(t, "", "verbose-main", "name = \"verbose-main\"\nmodel = \"anthropic/claude-sonnet-4-20250514\"\nsub_agents = [\"verbose-nested\"]\n")
	nested := "name = \"verbose-nested\"\nmodel = \"anthropic/claude-sonnet-4-20250514\"\n"
	if err := os.WriteFile(filepath.Join(configDir, "axe", "agents", "verbose-nested.toml"), []byte(nested), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "local-test-key")
	server := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicToolUseResponse("Delegating.", []testutil.MockToolCall{
			{ID: "delegate-1", Name: "call_agent", Input: map[string]string{"agent": "verbose-nested", "task": "Summarize."}},
		}),
		testutil.AnthropicResponseWithCacheTokens("Nested summary.", 7, 8, 90, 100),
		testutil.AnthropicResponse("Main finished."),
	})
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL())
	stdout, stderr := newMockStdoutStderr()

	result, err := Run(context.Background(), Options{
		AgentName: "verbose-main", Workdir: t.TempDir(), Prompt: "Delegate.",
		Stdin: strings.NewReader(""), Stdout: stdout, Stderr: stderr, JSON: true, Verbose: true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v; stderr = %s", err, stderr)
	}
	if got := server.RequestCount(); got != 3 {
		t.Fatalf("requests = %d, want Main/Nested/Main", got)
	}
	if result.CacheReadTokens != 0 || result.CacheWriteTokens != 0 {
		t.Errorf("Main counters include Nested usage: %+v", result)
	}
	verboseCacheJSON(t, stdout.Bytes(), 0, 0)
	tokens := fmt.Sprintf("Tokens:   %d input, %d output (cumulative)\n", result.InputTokens, result.OutputTokens)
	verboseCacheSummary(t, stderr.String(), tokens, "")
}

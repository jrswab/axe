package runner

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/provider"
)

type largeIntegerJSONOutput struct {
	Messages []struct {
		ToolCalls []struct {
			Arguments map[string]string `json:"arguments"`
		} `json:"tool_calls"`
	} `json:"messages"`
	ToolCallDetails []ToolCallDetail `json:"tool_call_details"`
}

func TestLargeIntegerArgs_Ollama_BufferedHistory(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "ollama/llama3"
tools = ["read_file"]
`)
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("controlled result"), 0600); err != nil {
		t.Fatal(err)
	}
	// Native argument values are raw JSON, including exceptional long tokens.
	raw := `{"path":"fixture.txt","million":1000000,"negative_million":-1000000,"signed":-1234567,"precise":9007199254740993,"negative_precise":-9007199254740993,"exponent":1e6,"decimal":1000000.0,"negative":-1e6,"precise_decimal":9007199254740993.0,"precise_exponent":90071992547409930e-1,"beyond":99999999999999999999999,"beyond_decimal":99999999999999999999999.0,"beyond_exponent":999999999999999999999990e-1,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"rounded_fraction":1000000.00000000001,"rounded_threshold":999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"false":false,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50],"recursive":{"a":[1000000,1.50]},"underflow":1e-1000000000,"zero_exponent":0e1000000000,"negative_zero_exponent":-0e1000000000,"exception_a":1` + strings.Repeat("0", 100006) + `e-100000,"exception_b":0.` + strings.Repeat("0", 9999) + `1e100000}`
	want := map[string]string{
		"path": "fixture.txt", "million": "1000000", "negative_million": "-1000000", "signed": "-1234567", "precise": "9007199254740993", "negative_precise": "-9007199254740993",
		"exponent": "1000000", "decimal": "1000000", "negative": "-1000000", "precise_decimal": "9007199254740993", "precise_exponent": "9007199254740993",
		"beyond": "99999999999999999999999", "beyond_decimal": "99999999999999999999999", "beyond_exponent": "99999999999999999999999",
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999", "zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "rounded_fraction": "1e+06", "rounded_threshold": "1e+06", "text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993",
		"boolean": "true", "false": "false", "null": "<nil>", "object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]", "array": "[1.5]", "recursive": "map[a:[1e+06 1.5]]",
		"underflow": "0", "zero_exponent": "0", "negative_zero_exponent": "-0", "exception_a": "0", "exception_b": "1",
	}
	bodies := make(chan string, 2)
	bodies <- `{"model":"llama3","message":{"content":"Read values","tool_calls":[{"function":{"name":"read_file","arguments":` + raw + `}}]},"done":true,"done_reason":"stop"}` + "\n"
	bodies <- `{"model":"llama3","message":{"content":"All done"},"done":true,"done_reason":"stop"}` + "\n"
	requests := make(chan []byte, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/chat" {
			t.Errorf("request = %s %s, want POST /api/chat", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- body
		select {
		case response := <-bodies:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, response)
		default:
			t.Error("unexpected extra request")
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	var hooks []ToolCall
	var events []ToolEvent
	stdout, stderr := newMockStdoutStderr()
	seed := Message{Role: "user", Content: "Keep this history"}
	result, err := Run(context.Background(), Options{
		AgentName: "test-agent", Workdir: workdir, BaseURL: server.URL,
		Messages: []Message{seed}, JSON: true, Stdout: stdout, Stderr: stderr,
		ToolHook: func(_ context.Context, call ToolCall) (ToolDecision, error) {
			hooks = append(hooks, call)
			return ToolDecision{Action: ToolAllow}, nil
		},
		ToolObserver: func(_ context.Context, event ToolEvent) { events = append(events, event) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "All done" || result.StopReason != "stop" || result.ToolCalls != 1 || len(result.Messages) != 4 || len(result.Messages[1].ToolCalls) != 1 || len(result.Messages[2].ToolResults) != 1 {
		t.Fatalf("result = %+v", result)
	}
	call := result.Messages[1].ToolCalls[0]
	wantResult := ToolResult{CallID: "ollama_0", Content: "1: controlled result"}
	if !reflect.DeepEqual(result.Messages[0], seed) || result.Messages[1].Role != "assistant" || result.Messages[1].Content != "Read values" || result.Messages[2].Role != "tool" || !reflect.DeepEqual(result.Messages[2].ToolResults[0], wantResult) || result.Messages[3].Role != "assistant" || result.Messages[3].Content != "All done" {
		t.Errorf("history = %+v", result.Messages)
	}
	if len(hooks) != 1 || len(events) != 1 || len(result.ToolCallDetails) != 1 {
		t.Fatalf("hook/observer/detail counts = %d/%d/%d", len(hooks), len(events), len(result.ToolCallDetails))
	}
	if call.ID != "ollama_0" || call.Name != "read_file" || hooks[0].ID != call.ID || hooks[0].Name != call.Name || events[0].Call.ID != call.ID || events[0].Call.Name != call.Name {
		t.Errorf("history/hook/observer calls = %+v/%+v/%+v", call, hooks[0], events[0].Call)
	}
	if ev := events[0]; ev.CallID != call.ID || ev.Action != ToolAllow || ev.IsError || ev.Turn != 0 || ev.ContentLength != len(wantResult.Content) {
		t.Errorf("observer = %+v", ev)
	}
	detail := result.ToolCallDetails[0]
	if detail.Name != call.Name || detail.Output != wantResult.Content || detail.IsError || detail.Turn != 0 {
		t.Errorf("detail = %+v", detail)
	}
	var output largeIntegerJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Messages) != 4 || len(output.Messages[1].ToolCalls) != 1 || len(output.ToolCallDetails) != 1 || len(requests) != 2 {
		t.Fatalf("JSON history/details = %+v, requests = %d", output, len(requests))
	}
	type wireRequest struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string            `json:"name"`
					Arguments map[string]string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	var initial, followup wireRequest
	if err := json.Unmarshal(<-requests, &initial); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(<-requests, &followup); err != nil {
		t.Fatalf("decode replay arguments as strings: %v", err)
	}
	if initial.Model != "llama3" || initial.Stream || len(initial.Messages) != 1 || initial.Messages[0].Role != seed.Role || initial.Messages[0].Content != seed.Content {
		t.Errorf("initial request = %+v", initial)
	}
	if followup.Model != "llama3" || followup.Stream || len(followup.Messages) != 3 || len(followup.Messages[1].ToolCalls) != 1 {
		t.Fatalf("follow-up = %+v", followup)
	}
	wireCall := followup.Messages[1].ToolCalls[0].Function
	if followup.Messages[0].Role != seed.Role || followup.Messages[0].Content != seed.Content || followup.Messages[1].Role != "assistant" || followup.Messages[1].Content != "Read values" || wireCall.Name != call.Name || followup.Messages[2].Role != "tool" || followup.Messages[2].Content != wantResult.Content {
		t.Errorf("follow-up messages = %+v", followup.Messages)
	}
	for name, got := range map[string]map[string]string{
		"history": call.Arguments, "hook": hooks[0].Arguments, "observer": events[0].Call.Arguments, "detail": detail.Input,
		"JSON history": output.Messages[1].ToolCalls[0].Arguments, "JSON detail": output.ToolCallDetails[0].Input, "replay": wireCall.Arguments,
	} {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s arguments = %v, want %v", name, got, want)
		}
	}
}

func TestLargeIntegerArgs_Ollama_StreamHistory(t *testing.T) {
	// The 100015-byte exceptional token A exceeds Ollama's existing scanner limit;
	// exceptional token B fits and exercises the approved legacy fallback here.
	raw := `{"path":"fixture.txt","million":1000000,"negative_million":-1000000,"signed":-1234567,"positive_integer":1234567,"precise":9007199254740993,"negative_precise":-9007199254740993,"exponent":1e6,"negative_exponent":-1e6,"decimal":1000000.0,"negative_decimal":-1000000.0,"precise_decimal":9007199254740993.0,"negative_precise_decimal":-9007199254740993.0,"precise_exponent":90071992547409930e-1,"negative_precise_exponent":-90071992547409930e-1,"beyond":99999999999999999999999,"negative_beyond":-99999999999999999999999,"beyond_decimal":99999999999999999999999.0,"negative_beyond_decimal":-99999999999999999999999.0,"beyond_exponent":999999999999999999999990e-1,"negative_beyond_exponent":-999999999999999999999990e-1,"cap":1e308,"negative_cap":-1e308,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"negative_small_decimal":-999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"negative_fraction":-1.5,"rounded_fraction":1000000.00000000001,"negative_rounded_fraction":-1000000.00000000001,"rounded_threshold":999999.99999999999,"negative_rounded_threshold":-999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"false":false,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50],"recursive":{"a":[1000000,1.50]},"underflow":1e-1000000000,"negative_underflow":-1e-1000000000,"zero_exponent":0e1000000000,"negative_zero_exponent":-0e1000000000,"exception_b":0.` + strings.Repeat("0", 9999) + `1e100000}`
	want := map[string]string{
		"path": "fixture.txt", "million": "1000000", "negative_million": "-1000000", "signed": "-1234567", "positive_integer": "1234567", "precise": "9007199254740993", "negative_precise": "-9007199254740993",
		"exponent": "1000000", "negative_exponent": "-1000000", "decimal": "1000000", "negative_decimal": "-1000000", "precise_decimal": "9007199254740993", "negative_precise_decimal": "-9007199254740993", "precise_exponent": "9007199254740993", "negative_precise_exponent": "-9007199254740993",
		"beyond": "99999999999999999999999", "negative_beyond": "-99999999999999999999999", "beyond_decimal": "99999999999999999999999", "negative_beyond_decimal": "-99999999999999999999999", "beyond_exponent": "99999999999999999999999", "negative_beyond_exponent": "-99999999999999999999999", "cap": "1" + strings.Repeat("0", 308), "negative_cap": "-1" + strings.Repeat("0", 308),
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999", "negative_small_decimal": "-999999", "zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "negative_fraction": "-1.5", "rounded_fraction": "1e+06", "negative_rounded_fraction": "-1e+06", "rounded_threshold": "1e+06", "negative_rounded_threshold": "-1e+06",
		"text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993", "boolean": "true", "false": "false", "null": "<nil>",
		"object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]", "array": "[1.5]", "recursive": "map[a:[1e+06 1.5]]", "underflow": "0", "negative_underflow": "-0", "zero_exponent": "0", "negative_zero_exponent": "-0", "exception_b": "1",
	}
	toolBody := `{"message":{"content":"Read values"},"done":false}` + "\n" +
		`{"message":{"tool_calls":[{"function":{"name":"read_file","arguments":` + raw + `}}]},"done":false}` + "\n" +
		`{"message":{"content":""},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}` + "\n"
	finalBody := `{"message":{"content":"All done"},"done":false}` + "\n" + `{"done":true,"done_reason":"stop","prompt_eval_count":12,"eval_count":2}` + "\n"

	t.Run("real_provider_drain", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, toolBody)
		}))
		defer server.Close()
		o, err := provider.NewOllama(provider.WithOllamaBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		stream, err := o.SendStream(context.Background(), &provider.Request{Model: "llama3", Messages: []provider.Message{{Role: "user", Content: "Read values"}}})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := drainEventStream(stream, nil)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Content != "Read values" || resp.StopReason != "stop" || resp.InputTokens != 10 || resp.OutputTokens != 5 || len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "ollama_0" || resp.ToolCalls[0].Name != "read_file" || !reflect.DeepEqual(resp.ToolCalls[0].Arguments, want) {
			t.Fatalf("drained response = %+v, want arguments %v", resp, want)
		}
	})

	for _, arg := range []string{"{}", "null"} {
		t.Run("empty_map_"+arg, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, `{"message":{"tool_calls":[{"function":{"name":"example","arguments":`+arg+`}}]}}`+"\n")
			}))
			defer server.Close()
			o, err := provider.NewOllama(provider.WithOllamaBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			stream, err := o.SendStream(context.Background(), &provider.Request{Model: "llama3", Messages: []provider.Message{{Role: "user", Content: "Read values"}}})
			if err != nil {
				t.Fatal(err)
			}
			resp, err := drainEventStream(stream, nil)
			if err != nil || len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Arguments == nil || len(resp.ToolCalls[0].Arguments) != 0 {
				t.Fatalf("drain(%s) = %+v, %v; want nonnil empty Arguments", arg, resp, err)
			}
		})
	}

	t.Run("real_run_history_and_replay", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
		setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "ollama/llama3"
tools = ["read_file"]
`)
		workdir := t.TempDir()
		if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("controlled result"), 0600); err != nil {
			t.Fatal(err)
		}
		bodies := make(chan string, 2)
		bodies <- toolBody
		bodies <- finalBody
		requests := make(chan []byte, 3)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/api/chat" {
				t.Errorf("request = %s %s", r.Method, r.URL.Path)
				http.NotFound(w, r)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			requests <- body
			select {
			case response := <-bodies:
				w.Header().Set("Content-Type", "application/x-ndjson")
				_, _ = io.WriteString(w, response)
			default:
				t.Error("unexpected extra request")
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		defer server.Close()
		var hooks []ToolCall
		var events []ToolEvent
		stdout, stderr := newMockStdoutStderr()
		seed := Message{Role: "user", Content: "Keep this history"}
		result, err := Run(context.Background(), Options{
			AgentName: "test-agent", Workdir: workdir, BaseURL: server.URL, Stream: true,
			Messages: []Message{seed}, JSON: true, Stdout: stdout, Stderr: stderr,
			ToolHook: func(_ context.Context, call ToolCall) (ToolDecision, error) {
				hooks = append(hooks, call)
				return ToolDecision{Action: ToolAllow}, nil
			},
			ToolObserver: func(_ context.Context, event ToolEvent) { events = append(events, event) },
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.Content != "All done" || result.StopReason != "stop" || result.ToolCalls != 1 || len(result.Messages) != 4 || len(result.Messages[1].ToolCalls) != 1 || len(result.Messages[2].ToolResults) != 1 {
			t.Fatalf("result = %+v", result)
		}
		call := result.Messages[1].ToolCalls[0]
		wantResult := ToolResult{CallID: "ollama_0", Content: "1: controlled result"}
		if !reflect.DeepEqual(result.Messages[0], seed) || result.Messages[1].Role != "assistant" || result.Messages[1].Content != "Read values" || result.Messages[2].Role != "tool" || !reflect.DeepEqual(result.Messages[2].ToolResults[0], wantResult) || result.Messages[3].Role != "assistant" || result.Messages[3].Content != "All done" {
			t.Errorf("history = %+v", result.Messages)
		}
		if len(hooks) != 1 || len(events) != 1 || len(result.ToolCallDetails) != 1 {
			t.Fatalf("hook/observer/detail counts = %d/%d/%d", len(hooks), len(events), len(result.ToolCallDetails))
		}
		if call.ID != "ollama_0" || call.Name != "read_file" || hooks[0].ID != call.ID || hooks[0].Name != call.Name || events[0].Call.ID != call.ID || events[0].Call.Name != call.Name {
			t.Errorf("history/hook/observer calls = %+v/%+v/%+v", call, hooks[0], events[0].Call)
		}
		if ev := events[0]; ev.CallID != call.ID || ev.Action != ToolAllow || ev.IsError || ev.Turn != 0 || ev.ContentLength != len(wantResult.Content) {
			t.Errorf("observer = %+v", ev)
		}
		detail := result.ToolCallDetails[0]
		if detail.Name != call.Name || detail.Output != wantResult.Content || detail.IsError || detail.Turn != 0 {
			t.Errorf("detail = %+v", detail)
		}
		var output largeIntegerJSONOutput
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
			t.Fatal(err)
		}
		if len(output.Messages) != 4 || len(output.Messages[1].ToolCalls) != 1 || len(output.ToolCallDetails) != 1 || len(requests) != 2 {
			t.Fatalf("JSON history/details = %+v, requests = %d", output, len(requests))
		}
		type wireRequest struct {
			Model    string `json:"model"`
			Stream   bool   `json:"stream"`
			Messages []struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string            `json:"name"`
						Arguments map[string]string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"messages"`
		}
		var initial, followup wireRequest
		if err := json.Unmarshal(<-requests, &initial); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(<-requests, &followup); err != nil {
			t.Fatalf("decode replay arguments as strings: %v", err)
		}
		if initial.Model != "llama3" || !initial.Stream || len(initial.Messages) != 1 || initial.Messages[0].Role != seed.Role || initial.Messages[0].Content != seed.Content {
			t.Errorf("initial request = %+v", initial)
		}
		if followup.Model != "llama3" || !followup.Stream || len(followup.Messages) != 3 || len(followup.Messages[1].ToolCalls) != 1 {
			t.Fatalf("follow-up = %+v", followup)
		}
		wireCall := followup.Messages[1].ToolCalls[0].Function
		if followup.Messages[0].Role != seed.Role || followup.Messages[0].Content != seed.Content || followup.Messages[1].Role != "assistant" || followup.Messages[1].Content != "Read values" || wireCall.Name != call.Name || followup.Messages[2].Role != "tool" || followup.Messages[2].Content != wantResult.Content {
			t.Errorf("follow-up messages = %+v", followup.Messages)
		}
		for name, got := range map[string]map[string]string{
			"history": call.Arguments, "hook": hooks[0].Arguments, "observer": events[0].Call.Arguments, "detail": detail.Input,
			"JSON history": output.Messages[1].ToolCalls[0].Arguments, "JSON detail": output.ToolCallDetails[0].Input, "replay": wireCall.Arguments,
		} {
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s arguments = %v, want %v", name, got, want)
			}
		}
	})
}

func TestLargeIntegerArgs_Gemini_BufferedHistory(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "google/gemini-2.0-flash"
tools = ["read_file"]
`)
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("controlled result"), 0600); err != nil {
		t.Fatal(err)
	}
	// This native object contains raw numeric tokens, not a JSON-encoded string.
	const raw = `{"path":"fixture.txt","million":1000000,"signed":-1234567,"precise":9007199254740993,"negative_precise":-9007199254740993,"exponent":1e6,"decimal":1000000.0,"negative":-1e6,"precise_decimal":9007199254740993.0,"precise_exponent":90071992547409930e-1,"beyond":99999999999999999999999,"beyond_decimal":99999999999999999999999.0,"beyond_exponent":999999999999999999999990e-1,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"rounded_fraction":1000000.00000000001,"rounded_threshold":999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"false":false,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50],"recursive":{"a":[1000000,1.50]}}`
	want := map[string]string{
		"path": "fixture.txt", "million": "1000000", "signed": "-1234567", "precise": "9007199254740993", "negative_precise": "-9007199254740993",
		"exponent": "1000000", "decimal": "1000000", "negative": "-1000000", "precise_decimal": "9007199254740993", "precise_exponent": "9007199254740993",
		"beyond": "99999999999999999999999", "beyond_decimal": "99999999999999999999999", "beyond_exponent": "99999999999999999999999",
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999", "zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "rounded_fraction": "1e+06", "rounded_threshold": "1e+06", "text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993",
		"boolean": "true", "false": "false", "null": "<nil>", "object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]", "array": "[1.5]", "recursive": "map[a:[1e+06 1.5]]",
	}
	bodies := make(chan string, 2)
	bodies <- `{"candidates":[{"content":{"role":"model","parts":[{"text":"Read values"},{"functionCall":{"name":"read_file","args":` + raw + `}}]},"finishReason":"STOP"}]}`
	bodies <- `{"candidates":[{"content":{"role":"model","parts":[{"text":"All done"}]},"finishReason":"STOP"}]}`
	requests := make(chan []byte, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1beta/models/gemini-2.0-flash:generateContent" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- body
		select {
		case response := <-bodies:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, response)
		default:
			t.Error("unexpected extra request")
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	var hooks []ToolCall
	var events []ToolEvent
	stdout, stderr := newMockStdoutStderr()
	seed := Message{Role: "user", Content: "Keep this history"}
	result, err := Run(context.Background(), Options{
		AgentName: "test-agent", Workdir: workdir, APIKey: "test-key", BaseURL: server.URL,
		Messages: []Message{seed}, JSON: true, Stdout: stdout, Stderr: stderr,
		ToolHook: func(_ context.Context, call ToolCall) (ToolDecision, error) {
			hooks = append(hooks, call)
			return ToolDecision{Action: ToolAllow}, nil
		},
		ToolObserver: func(_ context.Context, event ToolEvent) { events = append(events, event) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "All done" || result.StopReason != "STOP" || result.ToolCalls != 1 {
		t.Errorf("result content/stop/calls = %q/%q/%d", result.Content, result.StopReason, result.ToolCalls)
	}
	if len(result.Messages) != 4 || len(result.Messages[1].ToolCalls) != 1 || len(result.Messages[2].ToolResults) != 1 {
		t.Fatalf("history = %+v, want user, assistant call, tool result, final assistant", result.Messages)
	}
	call := result.Messages[1].ToolCalls[0]
	wantResult := ToolResult{CallID: "gemini_0", Content: "1: controlled result"}
	if !reflect.DeepEqual(result.Messages[0], seed) || result.Messages[1].Role != "assistant" || result.Messages[1].Content != "Read values" || result.Messages[2].Role != "tool" || !reflect.DeepEqual(result.Messages[2].ToolResults[0], wantResult) || result.Messages[3].Role != "assistant" || result.Messages[3].Content != "All done" {
		t.Errorf("history messages = %+v", result.Messages)
	}
	if len(hooks) != 1 || len(events) != 1 || len(result.ToolCallDetails) != 1 {
		t.Fatalf("hook/observer/detail counts = %d/%d/%d", len(hooks), len(events), len(result.ToolCallDetails))
	}
	if call.ID != "gemini_0" || call.Name != "read_file" || hooks[0].ID != call.ID || hooks[0].Name != call.Name || events[0].Call.ID != call.ID || events[0].Call.Name != call.Name {
		t.Errorf("history/hook/observer calls = %+v/%+v/%+v", call, hooks[0], events[0].Call)
	}
	if ev := events[0]; ev.CallID != call.ID || ev.Action != ToolAllow || ev.IsError || ev.Turn != 0 || ev.ContentLength != len(wantResult.Content) {
		t.Errorf("observer metadata = %+v", ev)
	}
	detail := result.ToolCallDetails[0]
	if detail.Name != call.Name || detail.Output != wantResult.Content || detail.IsError || detail.Turn != 0 {
		t.Errorf("tool detail = %+v", detail)
	}
	var output largeIntegerJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Messages) != 4 || len(output.Messages[1].ToolCalls) != 1 || len(output.ToolCallDetails) != 1 {
		t.Fatalf("JSON history/details = %+v", output)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	type wireRequest struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text         string `json:"text"`
				FunctionCall *struct {
					Name string            `json:"name"`
					Args map[string]string `json:"args"`
				} `json:"functionCall"`
				FunctionResponse *struct {
					Name     string `json:"name"`
					Response struct {
						Result string `json:"result"`
					} `json:"response"`
				} `json:"functionResponse"`
			} `json:"parts"`
		} `json:"contents"`
	}
	var initial, followup wireRequest
	if err := json.Unmarshal(<-requests, &initial); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(<-requests, &followup); err != nil {
		t.Fatalf("decode follow-up args as strings: %v", err)
	}
	if len(initial.Contents) != 1 || initial.Contents[0].Role != "user" || len(initial.Contents[0].Parts) != 1 || initial.Contents[0].Parts[0].Text != seed.Content {
		t.Errorf("initial request = %+v", initial)
	}
	if len(followup.Contents) != 3 || followup.Contents[0].Role != "user" || len(followup.Contents[0].Parts) != 1 || followup.Contents[0].Parts[0].Text != seed.Content || followup.Contents[1].Role != "model" || len(followup.Contents[1].Parts) != 2 || followup.Contents[1].Parts[0].Text != "Read values" || followup.Contents[2].Role != "user" || len(followup.Contents[2].Parts) != 1 {
		t.Fatalf("follow-up contents = %+v", followup.Contents)
	}
	wireCall := followup.Contents[1].Parts[1].FunctionCall
	wireResult := followup.Contents[2].Parts[0].FunctionResponse
	if wireCall == nil || wireCall.Name != call.Name || wireResult == nil || wireResult.Name != call.Name || wireResult.Response.Result != wantResult.Content {
		t.Fatalf("follow-up call/result = %+v/%+v", wireCall, wireResult)
	}
	for name, got := range map[string]map[string]string{
		"history": call.Arguments, "hook": hooks[0].Arguments, "observer": events[0].Call.Arguments, "detail": detail.Input,
		"JSON history": output.Messages[1].ToolCalls[0].Arguments, "JSON detail": output.ToolCallDetails[0].Input, "replay": wireCall.Args,
	} {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s arguments = %v, want %v", name, got, want)
		}
	}
}

func TestLargeIntegerArgs_Gemini_StreamHistory(t *testing.T) {
	// Values are raw JSON; only the runner's formatting boundary produces strings.
	raw := `{"path":"fixture.txt","million":1000000,"signed":-1234567,"precise":9007199254740993,"negative_precise":-9007199254740993,"exponent":1e6,"decimal":1000000.0,"negative":-1e6,"precise_decimal":9007199254740993.0,"precise_exponent":90071992547409930e-1,"beyond":99999999999999999999999,"beyond_decimal":99999999999999999999999.0,"beyond_exponent":999999999999999999999990e-1,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"rounded_fraction":1000000.00000000001,"rounded_threshold":999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"false":false,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50],"recursive":{"a":[1000000,1.50]},"underflow":1e-1000000000,"zero_exponent":0e1000000000,"negative_zero_exponent":-0e1000000000,"exception_b":0.` + strings.Repeat("0", 9999) + `1e100000}`
	want := map[string]string{
		"path": "fixture.txt", "million": "1000000", "signed": "-1234567", "precise": "9007199254740993", "negative_precise": "-9007199254740993",
		"exponent": "1000000", "decimal": "1000000", "negative": "-1000000", "precise_decimal": "9007199254740993", "precise_exponent": "9007199254740993",
		"beyond": "99999999999999999999999", "beyond_decimal": "99999999999999999999999", "beyond_exponent": "99999999999999999999999",
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999", "zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "rounded_fraction": "1e+06", "rounded_threshold": "1e+06", "text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993",
		"boolean": "true", "false": "false", "null": "<nil>", "object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]", "array": "[1.5]", "recursive": "map[a:[1e+06 1.5]]",
		"underflow": "0", "zero_exponent": "0", "negative_zero_exponent": "-0", "exception_b": "1",
	}
	toolBody := `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"Read values"},{"functionCall":{"name":"read_file","args":` + raw + `}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}` + "\n\n"
	finalBody := `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"All done"}]},"finishReason":"STOP"}]}` + "\n\n"

	t.Run("real_provider_drain", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, toolBody)
		}))
		defer server.Close()
		g, err := provider.NewGemini("test-key", provider.WithGeminiBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		stream, err := g.SendStream(context.Background(), &provider.Request{Model: "gemini-2.0-flash", Messages: []provider.Message{{Role: "user", Content: "Read values"}}})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := drainEventStream(stream, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "gemini_0" || resp.ToolCalls[0].Name != "read_file" || !reflect.DeepEqual(resp.ToolCalls[0].Arguments, want) {
			t.Fatalf("drained calls = %+v, want arguments %v", resp.ToolCalls, want)
		}
	})

	t.Run("real_run_history_and_replay", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
		setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "google/gemini-2.0-flash"
tools = ["read_file"]
`)
		workdir := t.TempDir()
		if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("controlled result"), 0600); err != nil {
			t.Fatal(err)
		}
		bodies := make(chan string, 2)
		bodies <- toolBody
		bodies <- finalBody
		requests := make(chan []byte, 3)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1beta/models/gemini-2.0-flash:streamGenerateContent" || r.URL.RawQuery != "alt=sse" {
				t.Errorf("request = %s %s", r.Method, r.URL.String())
				http.NotFound(w, r)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			requests <- body
			select {
			case response := <-bodies:
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, response)
			default:
				t.Error("unexpected extra request")
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		defer server.Close()
		var hooks []ToolCall
		var events []ToolEvent
		stdout, stderr := newMockStdoutStderr()
		seed := Message{Role: "user", Content: "Keep this history"}
		result, err := Run(context.Background(), Options{
			AgentName: "test-agent", Workdir: workdir, APIKey: "test-key", BaseURL: server.URL, Stream: true,
			Messages: []Message{seed}, JSON: true, Stdout: stdout, Stderr: stderr,
			ToolHook: func(_ context.Context, call ToolCall) (ToolDecision, error) {
				hooks = append(hooks, call)
				return ToolDecision{Action: ToolAllow}, nil
			},
			ToolObserver: func(_ context.Context, event ToolEvent) { events = append(events, event) },
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.Content != "All done" || result.StopReason != "STOP" || result.ToolCalls != 1 || len(result.Messages) != 4 || len(result.Messages[1].ToolCalls) != 1 || len(result.Messages[2].ToolResults) != 1 {
			t.Fatalf("result = %+v", result)
		}
		call := result.Messages[1].ToolCalls[0]
		wantResult := ToolResult{CallID: "gemini_0", Content: "1: controlled result"}
		if !reflect.DeepEqual(result.Messages[0], seed) || result.Messages[1].Role != "assistant" || result.Messages[1].Content != "Read values" || result.Messages[2].Role != "tool" || !reflect.DeepEqual(result.Messages[2].ToolResults[0], wantResult) || result.Messages[3].Role != "assistant" || result.Messages[3].Content != "All done" {
			t.Errorf("history messages = %+v", result.Messages)
		}
		if len(hooks) != 1 || len(events) != 1 || len(result.ToolCallDetails) != 1 {
			t.Fatalf("hook/observer/detail counts = %d/%d/%d", len(hooks), len(events), len(result.ToolCallDetails))
		}
		if call.ID != "gemini_0" || call.Name != "read_file" || hooks[0].ID != call.ID || hooks[0].Name != call.Name || events[0].Call.ID != call.ID || events[0].Call.Name != call.Name {
			t.Errorf("history/hook/observer calls = %+v/%+v/%+v", call, hooks[0], events[0].Call)
		}
		if ev := events[0]; ev.CallID != call.ID || ev.Action != ToolAllow || ev.IsError || ev.Turn != 0 || ev.ContentLength != len(wantResult.Content) {
			t.Errorf("observer metadata = %+v", ev)
		}
		detail := result.ToolCallDetails[0]
		if detail.Name != call.Name || detail.Output != wantResult.Content || detail.IsError || detail.Turn != 0 {
			t.Errorf("tool detail = %+v", detail)
		}
		var output largeIntegerJSONOutput
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
			t.Fatal(err)
		}
		if len(output.Messages) != 4 || len(output.Messages[1].ToolCalls) != 1 || len(output.ToolCallDetails) != 1 || len(requests) != 2 {
			t.Fatalf("JSON history/details = %+v, requests = %d", output, len(requests))
		}
		type wireRequest struct {
			Contents []struct {
				Role  string `json:"role"`
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string            `json:"name"`
						Args map[string]string `json:"args"`
					} `json:"functionCall"`
					FunctionResponse *struct {
						Name     string `json:"name"`
						Response struct {
							Result string `json:"result"`
						} `json:"response"`
					} `json:"functionResponse"`
				} `json:"parts"`
			} `json:"contents"`
		}
		var initial, followup wireRequest
		if err := json.Unmarshal(<-requests, &initial); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(<-requests, &followup); err != nil {
			t.Fatalf("decode follow-up args as strings: %v", err)
		}
		if len(initial.Contents) != 1 || initial.Contents[0].Role != "user" || len(initial.Contents[0].Parts) != 1 || initial.Contents[0].Parts[0].Text != seed.Content {
			t.Errorf("initial request = %+v", initial)
		}
		if len(followup.Contents) != 3 || followup.Contents[0].Role != "user" || len(followup.Contents[0].Parts) != 1 || followup.Contents[0].Parts[0].Text != seed.Content || followup.Contents[1].Role != "model" || len(followup.Contents[1].Parts) != 2 || followup.Contents[1].Parts[0].Text != "Read values" || followup.Contents[2].Role != "user" || len(followup.Contents[2].Parts) != 1 {
			t.Fatalf("follow-up contents = %+v", followup.Contents)
		}
		wireCall := followup.Contents[1].Parts[1].FunctionCall
		wireResult := followup.Contents[2].Parts[0].FunctionResponse
		if wireCall == nil || wireCall.Name != call.Name || wireResult == nil || wireResult.Name != call.Name || wireResult.Response.Result != wantResult.Content {
			t.Fatalf("follow-up call/result = %+v/%+v", wireCall, wireResult)
		}
		for name, got := range map[string]map[string]string{
			"history": call.Arguments, "hook": hooks[0].Arguments, "observer": events[0].Call.Arguments, "detail": detail.Input,
			"JSON history": output.Messages[1].ToolCalls[0].Arguments, "JSON detail": output.ToolCallDetails[0].Input, "replay": wireCall.Args,
		} {
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s arguments = %v, want %v", name, got, want)
			}
		}
	})
}

func TestLargeIntegerArgs_OpenAI_BufferedHistory(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", "")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "openai/gpt-4o"
tools = ["read_file"]
`)
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("controlled result"), 0600); err != nil {
		t.Fatal(err)
	}
	// Only the outer arguments string is encoded; all numeric tokens stay raw.
	const raw = `{"path":"fixture.txt","million":1000000,"precise":9007199254740993,"exponent":1e6,"decimal":1000000.0,"negative":-1e6,"precise_decimal":9007199254740993.0,"precise_exponent":90071992547409930e-1,"beyond":99999999999999999999999,"beyond_decimal":99999999999999999999999.0,"beyond_exponent":999999999999999999999990e-1,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"rounded_fraction":1000000.00000000001,"rounded_threshold":999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50],"recursive":{"a":[1000000,1.50]}}`
	want := map[string]string{
		"path": "fixture.txt", "million": "1000000", "precise": "9007199254740993",
		"exponent": "1000000", "decimal": "1000000", "negative": "-1000000",
		"precise_decimal": "9007199254740993", "precise_exponent": "9007199254740993",
		"beyond": "99999999999999999999999", "beyond_decimal": "99999999999999999999999", "beyond_exponent": "99999999999999999999999",
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999",
		"zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "rounded_fraction": "1e+06", "rounded_threshold": "1e+06",
		"text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993",
		"boolean": "true", "null": "<nil>", "object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]",
		"array": "[1.5]", "recursive": "map[a:[1e+06 1.5]]",
	}
	quoted, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"model":"gpt-4o","choices":[{"message":{"content":"Read values","tool_calls":[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":` + string(quoted) + `}}]},"finish_reason":"tool_calls"}]}`
	first := make(chan struct{}, 1)
	first <- struct{}{}
	requests := make(chan []byte, 3)
	replayed := make(chan map[string]string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- request
		w.Header().Set("Content-Type", "application/json")
		select {
		case <-first:
			_, _ = io.WriteString(w, body)
		default:
			var req largeIntegerOpenAIHistoryRequest
			if err := json.Unmarshal(request, &req); err != nil {
				t.Errorf("decode follow-up: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if len(req.Messages) != 3 || len(req.Messages[1].ToolCalls) != 1 {
				t.Errorf("follow-up messages = %+v, want user, assistant call, tool result", req.Messages)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var args map[string]string
			if err := json.Unmarshal([]byte(req.Messages[1].ToolCalls[0].Function.Arguments), &args); err != nil {
				t.Errorf("decode replay arguments as strings: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			replayed <- args
			_, _ = io.WriteString(w, `{"model":"gpt-4o","choices":[{"message":{"content":"All done"},"finish_reason":"stop"}]}`)
		}
	}))
	defer server.Close()
	t.Setenv("AXE_OPENAI_BASE_URL", server.URL)
	var hooks []ToolCall
	var events []ToolEvent
	stdout, stderr := newMockStdoutStderr()
	seed := Message{Role: "user", Content: "Keep this history"}
	result, err := Run(context.Background(), Options{
		AgentName: "test-agent", Workdir: workdir,
		Messages: []Message{seed}, JSON: true,
		Stdout: stdout, Stderr: stderr,
		ToolHook: func(_ context.Context, call ToolCall) (ToolDecision, error) {
			hooks = append(hooks, call)
			return ToolDecision{Action: ToolAllow}, nil
		},
		ToolObserver: func(_ context.Context, ev ToolEvent) {
			events = append(events, ev)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "All done" || result.StopReason != "stop" || result.ToolCalls != 1 {
		t.Errorf("result content/stop/calls = %q/%q/%d, want All done/stop/1", result.Content, result.StopReason, result.ToolCalls)
	}
	if len(result.Messages) != 4 || len(result.Messages[1].ToolCalls) != 1 || len(result.Messages[2].ToolResults) != 1 {
		t.Fatalf("history = %+v, want user, assistant call, tool result, final assistant", result.Messages)
	}
	if !reflect.DeepEqual(result.Messages[0], seed) || result.Messages[1].Role != "assistant" || result.Messages[1].Content != "Read values" || result.Messages[2].Role != "tool" || result.Messages[3].Role != "assistant" || result.Messages[3].Content != "All done" {
		t.Errorf("history messages = %+v", result.Messages)
	}
	wantResult := ToolResult{CallID: "call_1", Content: "1: controlled result"}
	if got := result.Messages[2].ToolResults[0]; !reflect.DeepEqual(got, wantResult) {
		t.Errorf("tool result = %+v, want %+v", got, wantResult)
	}
	if len(hooks) != 1 || len(events) != 1 || len(result.ToolCallDetails) != 1 {
		t.Fatalf("hook/observer/detail counts = %d/%d/%d, want 1/1/1", len(hooks), len(events), len(result.ToolCallDetails))
	}
	call := result.Messages[1].ToolCalls[0]
	if call.ID != "call_1" || call.Name != "read_file" || hooks[0].ID != call.ID || hooks[0].Name != call.Name || events[0].Call.ID != call.ID || events[0].Call.Name != call.Name {
		t.Errorf("history/hook/observer calls = %+v/%+v/%+v", call, hooks[0], events[0].Call)
	}
	// ToolObserver has one completed-call event, not separate Start/End events.
	if ev := events[0]; ev.CallID != "call_1" || ev.Action != ToolAllow || ev.IsError || ev.Turn != 0 || ev.ContentLength != len(wantResult.Content) {
		t.Errorf("observer metadata = %+v", ev)
	}
	detail := result.ToolCallDetails[0]
	if detail.Name != "read_file" || detail.Output != wantResult.Content || detail.IsError || detail.Turn != 0 {
		t.Errorf("tool detail = %+v", detail)
	}
	// Message has no JSON tags for tool calls; decode the wire names directly.
	var output largeIntegerJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Messages) != 4 || len(output.Messages[1].ToolCalls) != 1 || len(output.ToolCallDetails) != 1 {
		t.Fatalf("JSON history/details = %+v", output)
	}
	if len(requests) != 2 || len(replayed) != 1 {
		t.Fatalf("requests = %d, replay captures = %d, want 2 and 1", len(requests), len(replayed))
	}
	for name, got := range map[string]map[string]string{
		"history": call.Arguments, "hook": hooks[0].Arguments,
		"observer": events[0].Call.Arguments, "detail": detail.Input,
		"JSON history": output.Messages[1].ToolCalls[0].Arguments,
		"JSON detail":  output.ToolCallDetails[0].Input, "replay": <-replayed,
	} {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s arguments = %v, want %v", name, got, want)
		}
	}
	var initial, followup largeIntegerOpenAIHistoryRequest
	if err := json.Unmarshal(<-requests, &initial); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(<-requests, &followup); err != nil {
		t.Fatal(err)
	}
	if initial.Stream || len(initial.Messages) != 1 || initial.Messages[0].Role != seed.Role || initial.Messages[0].Content != seed.Content {
		t.Errorf("initial request = %+v, want buffered pre-built history", initial)
	}
	if followup.Stream || followup.Messages[0].Role != seed.Role || followup.Messages[0].Content != seed.Content || followup.Messages[1].Role != "assistant" || followup.Messages[1].Content != "Read values" {
		t.Errorf("follow-up history = %+v", followup)
	}
	wireCall := followup.Messages[1].ToolCalls[0]
	if wireCall.ID != call.ID || wireCall.Type != "function" || wireCall.Function.Name != call.Name {
		t.Errorf("replayed call = %+v", wireCall)
	}
	toolResult := followup.Messages[2]
	if toolResult.Role != "tool" || toolResult.ToolCallID != wantResult.CallID || toolResult.Content != wantResult.Content {
		t.Errorf("replayed tool result = %+v", toolResult)
	}
	for _, key := range []string{"million", "precise", "exponent", "decimal", "negative", "precise_decimal", "precise_exponent", "beyond", "beyond_decimal", "beyond_exponent"} {
		if strings.ContainsAny(want[key], "eE") || !strings.Contains(wireCall.Function.Arguments, `"`+key+`":"`+want[key]+`"`) {
			t.Errorf("replay lacks quoted exact digits for %s: %s", key, wireCall.Function.Arguments)
		}
	}
}

// largeIntegerOpenAIHistoryRequest decodes only the history wire surface;
// function.arguments remains a JSON string until explicitly checked as strings.
type largeIntegerOpenAIHistoryRequest struct {
	Stream   bool `json:"stream"`
	Messages []struct {
		Role       string `json:"role"`
		Content    string `json:"content"`
		ToolCallID string `json:"tool_call_id"`
		ToolCalls  []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"messages"`
}

func TestLargeIntegerArgs_OpenAI_StreamHistory(t *testing.T) {
	// Arguments text carries raw unquoted numeric tokens; only the outer SSE
	// arguments string is JSON-encoded.
	const raw = `{"path":"fixture.txt","million":1000000,"precise":9007199254740993,"exponent":1e6,"decimal":1000000.0,"negative":-1e6,"precise_decimal":9007199254740993.0,"precise_exponent":90071992547409930e-1,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"rounded_fraction":1000000.00000000001,"rounded_threshold":999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50]}`
	want := map[string]string{
		"path": "fixture.txt", "million": "1000000", "precise": "9007199254740993",
		"exponent": "1000000", "decimal": "1000000", "negative": "-1000000",
		"precise_decimal": "9007199254740993", "precise_exponent": "9007199254740993",
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999",
		"zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "rounded_fraction": "1e+06", "rounded_threshold": "1e+06",
		"text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993",
		"boolean": "true", "null": "<nil>", "object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]", "array": "[1.5]",
	}
	// Split inside the "precise" token so drain must concatenate deltas.
	split := strings.Index(raw, "740993,")
	if split < 0 {
		t.Fatal("fixture split point missing")
	}
	sseToolCall := func(deltas ...string) string {
		var sb strings.Builder
		sb.WriteString(`data: {"model":"gpt-4o","choices":[{"delta":{"content":"Read values"},"index":0}]}` + "\n\n")
		sb.WriteString(`data: {"model":"gpt-4o","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file","arguments":""}}]},"index":0}]}` + "\n\n")
		for _, d := range deltas {
			quoted, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			sb.WriteString(`data: {"model":"gpt-4o","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":` + string(quoted) + `}}]},"index":0}]}` + "\n\n")
		}
		sb.WriteString(`data: {"model":"gpt-4o","choices":[{"delta":{},"index":0,"finish_reason":"tool_calls"}]}` + "\n\n")
		sb.WriteString(`data: {"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5}}` + "\n\n")
		sb.WriteString("data: [DONE]\n\n")
		return sb.String()
	}
	toolBody := sseToolCall(raw[:split], raw[split:])
	const finalBody = `data: {"model":"gpt-4o","choices":[{"delta":{"content":"All done"},"index":0}]}` + "\n\n" +
		`data: {"model":"gpt-4o","choices":[{"delta":{},"index":0,"finish_reason":"stop"}]}` + "\n\n" +
		`data: {"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":12,"completion_tokens":2}}` + "\n\n" +
		"data: [DONE]\n\n"

	t.Run("drain_openai_concatenated_deltas", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, toolBody)
		}))
		defer server.Close()
		o, err := provider.NewOpenAI("test-key", provider.WithOpenAIBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		stream, err := o.SendStream(context.Background(), &provider.Request{
			Model:    "gpt-4o",
			Messages: []provider.Message{{Role: "user", Content: "Read values"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := drainEventStream(stream, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "call_1" || resp.ToolCalls[0].Name != "read_file" {
			t.Fatalf("drained tool calls = %+v, want one read_file call_1", resp.ToolCalls)
		}
		if got := resp.ToolCalls[0].Arguments; !reflect.DeepEqual(got, want) {
			t.Errorf("drained delta arguments = %v, want %v", got, want)
		}
	})

	t.Run("drain_start_input", func(t *testing.T) {
		stream := makeStream([]provider.StreamEvent{
			{Type: provider.StreamEventToolStart, ToolCallID: "call_1", ToolName: "read_file", ToolInput: raw},
			{Type: provider.StreamEventToolEnd, ToolCallID: "call_1"},
			{Type: provider.StreamEventDone, StopReason: "tool_calls"},
		})
		resp, err := drainEventStream(stream, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.ToolCalls) != 1 {
			t.Fatalf("drained tool calls = %+v, want 1", resp.ToolCalls)
		}
		if got := resp.ToolCalls[0].Arguments; !reflect.DeepEqual(got, want) {
			t.Errorf("drained start-input arguments = %v, want %v", got, want)
		}
	})

	t.Run("run_stream_history", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "")
		t.Setenv("AXE_ANTHROPIC_BASE_URL", "")
		t.Setenv("OPENAI_API_KEY", "test-key")
		t.Setenv("XDG_DATA_HOME", t.TempDir())
		setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "openai/gpt-4o"
stream = true
tools = ["read_file"]
`)
		workdir := t.TempDir()
		if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("controlled result"), 0600); err != nil {
			t.Fatal(err)
		}
		bodies := make(chan string, 2)
		bodies <- toolBody
		bodies <- finalBody
		requests := make(chan []byte, 3)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			request, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			requests <- request
			select {
			case body := <-bodies:
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, body)
			default:
				t.Errorf("unexpected extra request")
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		defer server.Close()
		t.Setenv("AXE_OPENAI_BASE_URL", server.URL)

		var hooks []ToolCall
		var events []ToolEvent
		stdout, stderr := newMockStdoutStderr()
		seed := Message{Role: "user", Content: "Keep this history"}
		result, err := Run(context.Background(), Options{
			AgentName: "test-agent", Workdir: workdir,
			Messages: []Message{seed}, JSON: true,
			Stdout: stdout, Stderr: stderr,
			ToolHook: func(_ context.Context, call ToolCall) (ToolDecision, error) {
				hooks = append(hooks, call)
				return ToolDecision{Action: ToolAllow}, nil
			},
			ToolObserver: func(_ context.Context, ev ToolEvent) {
				events = append(events, ev)
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Messages) != 4 || len(result.Messages[1].ToolCalls) != 1 {
			t.Fatalf("history = %+v, want user, assistant call, tool result, final assistant", result.Messages)
		}
		call := result.Messages[1].ToolCalls[0]
		if !reflect.DeepEqual(call.Arguments, want) {
			t.Errorf("history arguments = %v, want %v", call.Arguments, want)
		}
		if !reflect.DeepEqual(result.Messages[0], seed) || call.ID != "call_1" || call.Name != "read_file" ||
			result.Messages[2].Role != "tool" || result.Messages[3].Content != "All done" {
			t.Errorf("history messages = %+v", result.Messages)
		}
		if result.Content != "All done" || result.ToolCalls != 1 {
			t.Errorf("result content/calls = %q/%d, want All done/1", result.Content, result.ToolCalls)
		}
		if len(hooks) != 1 || len(events) != 1 || len(result.ToolCallDetails) != 1 {
			t.Fatalf("hook/observer/detail counts = %d/%d/%d, want 1/1/1", len(hooks), len(events), len(result.ToolCallDetails))
		}
		// Message has no JSON tags for tool calls; decode the wire names directly.
		var output largeIntegerJSONOutput
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
			t.Fatal(err)
		}
		if len(output.Messages) != 4 || len(output.Messages[1].ToolCalls) != 1 || len(output.ToolCallDetails) != 1 {
			t.Fatalf("JSON history/details = %+v", output)
		}
		for name, got := range map[string]map[string]string{
			"hook": hooks[0].Arguments, "observer": events[0].Call.Arguments,
			"detail": result.ToolCallDetails[0].Input, "JSON history": output.Messages[1].ToolCalls[0].Arguments,
			"JSON detail": output.ToolCallDetails[0].Input,
		} {
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s arguments = %v, want %v", name, got, want)
			}
		}
		if len(requests) != 2 {
			t.Fatalf("requests = %d, want 2", len(requests))
		}
		var initial, followup largeIntegerOpenAIHistoryRequest
		if err := json.Unmarshal(<-requests, &initial); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(<-requests, &followup); err != nil {
			t.Fatal(err)
		}
		if !initial.Stream || len(initial.Messages) != 1 || initial.Messages[0].Content != seed.Content {
			t.Errorf("initial request = %+v, want streamed pre-built history", initial)
		}
		if !followup.Stream || len(followup.Messages) != 3 || len(followup.Messages[1].ToolCalls) != 1 {
			t.Fatalf("follow-up request = %+v, want streamed user, assistant call, tool result", followup)
		}
		var replayed map[string]string
		if err := json.Unmarshal([]byte(followup.Messages[1].ToolCalls[0].Function.Arguments), &replayed); err != nil {
			t.Fatalf("decode replay arguments as strings: %v", err)
		}
		if !reflect.DeepEqual(replayed, want) {
			t.Errorf("replay arguments = %v, want %v", replayed, want)
		}
	})
}

func TestLargeIntegerArgs_Anthropic_BufferedHistory(t *testing.T) {
	testLargeIntegerArgsAnthropicHistory(t, false)
}

func TestLargeIntegerArgs_Anthropic_StreamHistory(t *testing.T) {
	testLargeIntegerArgsAnthropicHistory(t, true)
}

func testLargeIntegerArgsAnthropicHistory(t *testing.T, stream bool) {
	t.Helper()
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
tools = ["read_file"]
`)
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("controlled result"), 0600); err != nil {
		t.Fatal(err)
	}
	const raw = `{"path":"fixture.txt","million":1000000,"precise":9007199254740993,"exponent":1e6,"decimal":1000000.0,"negative":-1e6,"precise_decimal":9007199254740993.0,"precise_exponent":90071992547409930e-1,"beyond":99999999999999999999999,"beyond_decimal":99999999999999999999999.0,"beyond_exponent":999999999999999999999990e-1,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"rounded_fraction":1000000.00000000001,"rounded_threshold":999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50],"recursive":{"a":[1000000,1.50]}}`
	want := map[string]string{
		"path": "fixture.txt", "million": "1000000", "precise": "9007199254740993",
		"exponent": "1000000", "decimal": "1000000", "negative": "-1000000",
		"precise_decimal": "9007199254740993", "precise_exponent": "9007199254740993",
		"beyond": "99999999999999999999999", "beyond_decimal": "99999999999999999999999", "beyond_exponent": "99999999999999999999999",
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999",
		"zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "rounded_fraction": "1e+06", "rounded_threshold": "1e+06",
		"text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993",
		"boolean": "true", "null": "<nil>", "object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]",
		"array": "[1.5]", "recursive": "map[a:[1e+06 1.5]]",
	}
	toolBody := `{"content":[{"type":"text","text":"Read values"},{"type":"tool_use","id":"toolu_1","name":"read_file","input":` + raw + `}],"model":"claude-sonnet-4-20250514","stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":5}}`
	finalBody := `{"content":[{"type":"text","text":"All done"}],"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":2}}`
	if stream {
		split := strings.Index(raw, "740993,")
		if split < 0 {
			t.Fatal("fixture split point missing")
		}
		var body strings.Builder
		body.WriteString("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10}}}\n\n")
		body.WriteString("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n")
		body.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Read values\"}}\n\n")
		body.WriteString("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		body.WriteString("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"read_file\"}}\n\n")
		for _, delta := range []string{raw[:split], raw[split:]} {
			quoted, err := json.Marshal(delta)
			if err != nil {
				t.Fatal(err)
			}
			body.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":" + string(quoted) + "}}\n\n")
		}
		body.WriteString("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n")
		body.WriteString("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"output_tokens\":5}}\n\n")
		body.WriteString("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		toolBody = body.String()
		finalBody = "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n" +
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"All done\"}}\n\n" +
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n" +
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	}
	bodies := make(chan string, 2)
	bodies <- toolBody
	bodies <- finalBody
	requests := make(chan []byte, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- request
		select {
		case body := <-bodies:
			if stream {
				w.Header().Set("Content-Type", "text/event-stream")
			} else {
				w.Header().Set("Content-Type", "application/json")
			}
			_, _ = io.WriteString(w, body)
		default:
			t.Error("unexpected extra request")
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	t.Setenv("AXE_ANTHROPIC_BASE_URL", server.URL)
	var hooks []ToolCall
	var events []ToolEvent
	stdout, stderr := newMockStdoutStderr()
	seed := Message{Role: "user", Content: "Keep this history"}
	result, err := Run(context.Background(), Options{
		AgentName: "test-agent", Workdir: workdir, Stream: stream,
		Messages: []Message{seed}, JSON: true, Stdout: stdout, Stderr: stderr,
		ToolHook: func(_ context.Context, call ToolCall) (ToolDecision, error) {
			hooks = append(hooks, call)
			return ToolDecision{Action: ToolAllow}, nil
		},
		ToolObserver: func(_ context.Context, event ToolEvent) { events = append(events, event) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "All done" || result.StopReason != "end_turn" || result.ToolCalls != 1 {
		t.Errorf("result content/stop/calls = %q/%q/%d, want All done/end_turn/1", result.Content, result.StopReason, result.ToolCalls)
	}
	if len(result.Messages) != 4 || len(result.Messages[1].ToolCalls) != 1 || len(result.Messages[2].ToolResults) != 1 {
		t.Fatalf("history = %+v, want user, assistant call, tool result, final assistant", result.Messages)
	}
	call := result.Messages[1].ToolCalls[0]
	wantResult := ToolResult{CallID: "toolu_1", Content: "1: controlled result"}
	if !reflect.DeepEqual(result.Messages[0], seed) || result.Messages[1].Role != "assistant" || result.Messages[1].Content != "Read values" || result.Messages[2].Role != "tool" || !reflect.DeepEqual(result.Messages[2].ToolResults[0], wantResult) || result.Messages[3].Role != "assistant" || result.Messages[3].Content != "All done" {
		t.Errorf("history messages = %+v", result.Messages)
	}
	if len(hooks) != 1 || len(events) != 1 || len(result.ToolCallDetails) != 1 {
		t.Fatalf("hook/observer/detail counts = %d/%d/%d, want 1/1/1", len(hooks), len(events), len(result.ToolCallDetails))
	}
	if call.ID != "toolu_1" || call.Name != "read_file" || hooks[0].ID != call.ID || hooks[0].Name != call.Name || events[0].Call.ID != call.ID || events[0].Call.Name != call.Name {
		t.Errorf("history/hook/observer calls = %+v/%+v/%+v", call, hooks[0], events[0].Call)
	}
	if ev := events[0]; ev.CallID != call.ID || ev.Action != ToolAllow || ev.IsError || ev.Turn != 0 || ev.ContentLength != len(wantResult.Content) {
		t.Errorf("observer metadata = %+v", ev)
	}
	detail := result.ToolCallDetails[0]
	if detail.Name != call.Name || detail.Output != wantResult.Content || detail.IsError || detail.Turn != 0 {
		t.Errorf("tool detail = %+v", detail)
	}
	var output largeIntegerJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Messages) != 4 || len(output.Messages[1].ToolCalls) != 1 || len(output.ToolCallDetails) != 1 {
		t.Fatalf("JSON history/details = %+v", output)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	type wireRequest struct {
		Stream   bool `json:"stream"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	var initial, followup wireRequest
	if err := json.Unmarshal(<-requests, &initial); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(<-requests, &followup); err != nil {
		t.Fatal(err)
	}
	if initial.Stream != stream || len(initial.Messages) != 1 || initial.Messages[0].Role != seed.Role || string(initial.Messages[0].Content) != `"Keep this history"` {
		t.Errorf("initial request = %+v, want pre-built history", initial)
	}
	if followup.Stream != stream || len(followup.Messages) != 3 || followup.Messages[0].Role != seed.Role || string(followup.Messages[0].Content) != `"Keep this history"` || followup.Messages[1].Role != "assistant" || followup.Messages[2].Role != "user" {
		t.Fatalf("follow-up request = %+v, want user, assistant call, user tool result", followup)
	}
	var blocks []struct {
		Type  string            `json:"type"`
		Text  string            `json:"text"`
		ID    string            `json:"id"`
		Name  string            `json:"name"`
		Input map[string]string `json:"input"`
	}
	if err := json.Unmarshal(followup.Messages[1].Content, &blocks); err != nil {
		t.Fatalf("decode replay input as strings: %v", err)
	}
	if len(blocks) != 2 || blocks[0].Type != "text" || blocks[0].Text != "Read values" || blocks[1].Type != "tool_use" || blocks[1].ID != call.ID || blocks[1].Name != call.Name {
		t.Fatalf("replayed assistant blocks = %+v", blocks)
	}
	var resultBlocks []struct {
		Type      string `json:"type"`
		ToolUseID string `json:"tool_use_id"`
		Content   string `json:"content"`
		IsError   bool   `json:"is_error"`
	}
	if err := json.Unmarshal(followup.Messages[2].Content, &resultBlocks); err != nil {
		t.Fatal(err)
	}
	if len(resultBlocks) != 1 || resultBlocks[0].Type != "tool_result" || resultBlocks[0].ToolUseID != wantResult.CallID || resultBlocks[0].Content != wantResult.Content || resultBlocks[0].IsError {
		t.Errorf("replayed tool result = %+v", resultBlocks)
	}
	for name, got := range map[string]map[string]string{
		"history": call.Arguments, "hook": hooks[0].Arguments,
		"observer": events[0].Call.Arguments, "detail": detail.Input,
		"JSON history": output.Messages[1].ToolCalls[0].Arguments,
		"JSON detail":  output.ToolCallDetails[0].Input, "replay": blocks[1].Input,
	} {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s arguments = %v, want %v", name, got, want)
		}
	}
}

func TestLargeIntegerArgs_OpenRouter_BufferedHistory(t *testing.T) {
	testLargeIntegerArgsOpenRouterHistory(t, false, false)
}

func TestLargeIntegerArgs_OpenRouter_StreamHistory(t *testing.T) {
	for _, split := range []bool{false, true} {
		name := "unsplit"
		if split {
			name = "split"
		}
		t.Run(name, func(t *testing.T) { testLargeIntegerArgsOpenRouterHistory(t, true, split) })
	}
}

func testLargeIntegerArgsOpenRouterHistory(t *testing.T, stream, split bool) {
	t.Helper()
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "openrouter/openai/gpt-5"
tools = ["read_file"]
`)
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("controlled result"), 0600); err != nil {
		t.Fatal(err)
	}
	const raw = `{"path":"fixture.txt","million":1000000,"precise":9007199254740993,"exponent":1e6,"decimal":1000000.0,"negative":-1e6,"precise_decimal":9007199254740993.0,"precise_exponent":90071992547409930e-1,"beyond":99999999999999999999999,"beyond_decimal":99999999999999999999999.0,"beyond_exponent":999999999999999999999990e-1,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"rounded_fraction":1000000.00000000001,"rounded_threshold":999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"false":false,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50],"recursive":{"a":[1000000,1.50]}}`
	want := map[string]string{
		"path": "fixture.txt", "million": "1000000", "precise": "9007199254740993",
		"exponent": "1000000", "decimal": "1000000", "negative": "-1000000",
		"precise_decimal": "9007199254740993", "precise_exponent": "9007199254740993",
		"beyond": "99999999999999999999999", "beyond_decimal": "99999999999999999999999", "beyond_exponent": "99999999999999999999999",
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999",
		"zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "rounded_fraction": "1e+06", "rounded_threshold": "1e+06",
		"text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993",
		"boolean": "true", "false": "false", "null": "<nil>", "object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]",
		"array": "[1.5]", "recursive": "map[a:[1e+06 1.5]]",
	}
	quoted, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	toolBody := `{"model":"openai/gpt-5","choices":[{"message":{"content":"Read values","tool_calls":[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":` + string(quoted) + `}}]},"finish_reason":"tool_calls"}]}`
	finalBody := `{"model":"openai/gpt-5","choices":[{"message":{"content":"All done"},"finish_reason":"stop"}]}`
	if stream {
		deltas := []string{raw}
		if split {
			at := strings.Index(raw, "740993,")
			if at < 0 {
				t.Fatal("fixture split point missing")
			}
			deltas = []string{raw[:at], raw[at:]}
		}
		var body strings.Builder
		body.WriteString(`data: {"choices":[{"delta":{"content":"Read values"}}]}` + "\n\n")
		body.WriteString(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file"}}]}}]}` + "\n\n")
		for _, delta := range deltas {
			encoded, err := json.Marshal(delta)
			if err != nil {
				t.Fatal(err)
			}
			body.WriteString(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":` + string(encoded) + `}}]}}]}` + "\n\n")
		}
		body.WriteString(`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n" + `data: {"usage":{"prompt_tokens":10,"completion_tokens":5,"cost":0.01}}` + "\n\n" + "data: [DONE]\n\n")
		toolBody = body.String()
		finalBody = `data: {"choices":[{"delta":{"content":"All done"}}]}` + "\n\n" + `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}` + "\n\n" + `data: {"usage":{"prompt_tokens":12,"completion_tokens":2,"cost":0.01}}` + "\n\n" + "data: [DONE]\n\n"
	}
	bodies := make(chan string, 2)
	bodies <- toolBody
	bodies <- finalBody
	requests := make(chan []byte, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- request
		select {
		case body := <-bodies:
			if stream {
				w.Header().Set("Content-Type", "text/event-stream")
			} else {
				w.Header().Set("Content-Type", "application/json")
			}
			_, _ = io.WriteString(w, body)
		default:
			t.Error("unexpected extra request")
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	t.Setenv("AXE_OPENROUTER_BASE_URL", server.URL)
	var hooks []ToolCall
	var events []ToolEvent
	stdout, stderr := newMockStdoutStderr()
	seed := Message{Role: "user", Content: "Keep this history"}
	result, err := Run(context.Background(), Options{
		AgentName: "test-agent", Workdir: workdir, Stream: stream,
		Messages: []Message{seed}, JSON: true, Stdout: stdout, Stderr: stderr,
		ToolHook: func(_ context.Context, call ToolCall) (ToolDecision, error) {
			hooks = append(hooks, call)
			return ToolDecision{Action: ToolAllow}, nil
		},
		ToolObserver: func(_ context.Context, event ToolEvent) { events = append(events, event) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "All done" || result.StopReason != "stop" || result.ToolCalls != 1 {
		t.Errorf("result content/stop/calls = %q/%q/%d", result.Content, result.StopReason, result.ToolCalls)
	}
	if len(result.Messages) != 4 || len(result.Messages[1].ToolCalls) != 1 || len(result.Messages[2].ToolResults) != 1 {
		t.Fatalf("history = %+v, want user, assistant call, tool result, final assistant", result.Messages)
	}
	call := result.Messages[1].ToolCalls[0]
	wantResult := ToolResult{CallID: "call_1", Content: "1: controlled result"}
	if !reflect.DeepEqual(result.Messages[0], seed) || result.Messages[1].Role != "assistant" || result.Messages[1].Content != "Read values" || result.Messages[2].Role != "tool" || !reflect.DeepEqual(result.Messages[2].ToolResults[0], wantResult) || result.Messages[3].Role != "assistant" || result.Messages[3].Content != "All done" {
		t.Errorf("history messages = %+v", result.Messages)
	}
	if len(hooks) != 1 || len(events) != 1 || len(result.ToolCallDetails) != 1 {
		t.Fatalf("hook/observer/detail counts = %d/%d/%d, want 1/1/1", len(hooks), len(events), len(result.ToolCallDetails))
	}
	if call.ID != "call_1" || call.Name != "read_file" || hooks[0].ID != call.ID || hooks[0].Name != call.Name || events[0].Call.ID != call.ID || events[0].Call.Name != call.Name {
		t.Errorf("history/hook/observer calls = %+v/%+v/%+v", call, hooks[0], events[0].Call)
	}
	if ev := events[0]; ev.CallID != call.ID || ev.Action != ToolAllow || ev.IsError || ev.Turn != 0 || ev.ContentLength != len(wantResult.Content) {
		t.Errorf("observer metadata = %+v", ev)
	}
	detail := result.ToolCallDetails[0]
	if detail.Name != call.Name || detail.Output != wantResult.Content || detail.IsError || detail.Turn != 0 {
		t.Errorf("tool detail = %+v", detail)
	}
	var output largeIntegerJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Messages) != 4 || len(output.Messages[1].ToolCalls) != 1 || len(output.ToolCallDetails) != 1 {
		t.Fatalf("JSON history/details = %+v", output)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	var initial, followup largeIntegerOpenAIHistoryRequest
	if err := json.Unmarshal(<-requests, &initial); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(<-requests, &followup); err != nil {
		t.Fatal(err)
	}
	if initial.Stream != stream || len(initial.Messages) != 1 || initial.Messages[0].Role != seed.Role || initial.Messages[0].Content != seed.Content {
		t.Errorf("initial request = %+v", initial)
	}
	if followup.Stream != stream || len(followup.Messages) != 3 || len(followup.Messages[1].ToolCalls) != 1 {
		t.Fatalf("follow-up request = %+v, want user, assistant call, tool result", followup)
	}
	wireCall := followup.Messages[1].ToolCalls[0]
	if followup.Messages[1].Role != "assistant" || wireCall.ID != call.ID || wireCall.Type != "function" || wireCall.Function.Name != call.Name || followup.Messages[2].Role != "tool" || followup.Messages[2].ToolCallID != wantResult.CallID || followup.Messages[2].Content != wantResult.Content {
		t.Errorf("follow-up messages = %+v", followup.Messages)
	}
	var replayed map[string]string
	if err := json.Unmarshal([]byte(wireCall.Function.Arguments), &replayed); err != nil {
		t.Fatalf("decode replay arguments as strings: %v", err)
	}
	for name, got := range map[string]map[string]string{
		"history": call.Arguments, "hook": hooks[0].Arguments,
		"observer": events[0].Call.Arguments, "detail": detail.Input,
		"JSON history": output.Messages[1].ToolCalls[0].Arguments,
		"JSON detail":  output.ToolCallDetails[0].Input, "replay": replayed,
	} {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s arguments = %v, want %v", name, got, want)
		}
	}
}

func TestLargeIntegerArgs_OpenCodeClaude_BufferedHistory(t *testing.T) {
	testLargeIntegerArgsOpenCodeClaudeHistory(t, false, false)
}

func TestLargeIntegerArgs_OpenCodeClaude_StreamHistory(t *testing.T) {
	for _, split := range []bool{false, true} {
		name := "unsplit"
		if split {
			name = "split"
		}
		t.Run(name, func(t *testing.T) { testLargeIntegerArgsOpenCodeClaudeHistory(t, true, split) })
	}
}

func testLargeIntegerArgsOpenCodeClaudeHistory(t *testing.T, stream, split bool) {
	t.Helper()
	t.Setenv("OPENCODE_API_KEY", "test-key")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "opencode/claude-sonnet-4-6"
tools = ["read_file"]
`)
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("controlled result"), 0600); err != nil {
		t.Fatal(err)
	}
	const raw = `{"path":"fixture.txt","million":1000000,"signed":-1234567,"precise":9007199254740993,"negative_precise":-9007199254740993,"exponent":1e6,"decimal":1000000.0,"negative":-1e6,"precise_decimal":9007199254740993.0,"precise_exponent":90071992547409930e-1,"beyond":99999999999999999999999,"negative_beyond":-99999999999999999999999,"beyond_decimal":99999999999999999999999.0,"beyond_exponent":999999999999999999999990e-1,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"rounded_fraction":1000000.00000000001,"rounded_threshold":999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"false":false,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50],"recursive":{"a":[1000000,1.50]}}`
	want := map[string]string{
		"path": "fixture.txt", "million": "1000000", "signed": "-1234567", "precise": "9007199254740993", "negative_precise": "-9007199254740993",
		"exponent": "1000000", "decimal": "1000000", "negative": "-1000000", "precise_decimal": "9007199254740993", "precise_exponent": "9007199254740993",
		"beyond": "99999999999999999999999", "negative_beyond": "-99999999999999999999999", "beyond_decimal": "99999999999999999999999", "beyond_exponent": "99999999999999999999999",
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999", "zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "rounded_fraction": "1e+06", "rounded_threshold": "1e+06", "text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993",
		"boolean": "true", "false": "false", "null": "<nil>", "object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]", "array": "[1.5]", "recursive": "map[a:[1e+06 1.5]]",
	}
	toolBody := `{"content":[{"type":"text","text":"Read values"},{"type":"tool_use","id":"toolu_1","name":"read_file","input":` + raw + `}],"model":"claude-sonnet-4-6","stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":5}}`
	finalBody := `{"content":[{"type":"text","text":"All done"}],"model":"claude-sonnet-4-6","stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":2}}`
	if stream {
		deltas := []string{raw}
		if split {
			at := strings.Index(raw, "740993,")
			if at < 0 {
				t.Fatal("fixture split point missing")
			}
			deltas = []string{raw[:at], raw[at:]}
		}
		var body strings.Builder
		body.WriteString("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10}}}\n\n")
		body.WriteString("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n")
		body.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Read values\"}}\n\n")
		body.WriteString("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		body.WriteString("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"read_file\"}}\n\n")
		for _, delta := range deltas {
			encoded, err := json.Marshal(delta)
			if err != nil {
				t.Fatal(err)
			}
			body.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":" + string(encoded) + "}}\n\n")
		}
		body.WriteString("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n")
		body.WriteString("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"output_tokens\":5}}\n\n")
		body.WriteString("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		toolBody = body.String()
		finalBody = "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n" +
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"All done\"}}\n\n" +
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n" +
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	}
	bodies := make(chan string, 2)
	bodies <- toolBody
	bodies <- finalBody
	requests := make(chan []byte, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			t.Errorf("request = %s %s, want POST /v1/messages", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		request, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- request
		select {
		case body := <-bodies:
			if stream {
				w.Header().Set("Content-Type", "text/event-stream")
			} else {
				w.Header().Set("Content-Type", "application/json")
			}
			_, _ = io.WriteString(w, body)
		default:
			t.Error("unexpected extra request")
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	var hooks []ToolCall
	var events []ToolEvent
	stdout, stderr := newMockStdoutStderr()
	seed := Message{Role: "user", Content: "Keep this history"}
	result, err := Run(context.Background(), Options{
		AgentName: "test-agent", Workdir: workdir, BaseURL: server.URL, Stream: stream,
		Messages: []Message{seed}, JSON: true, Stdout: stdout, Stderr: stderr,
		ToolHook: func(_ context.Context, call ToolCall) (ToolDecision, error) {
			hooks = append(hooks, call)
			return ToolDecision{Action: ToolAllow}, nil
		},
		ToolObserver: func(_ context.Context, event ToolEvent) { events = append(events, event) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "All done" || result.StopReason != "end_turn" || result.ToolCalls != 1 {
		t.Errorf("result content/stop/calls = %q/%q/%d", result.Content, result.StopReason, result.ToolCalls)
	}
	if len(result.Messages) != 4 || len(result.Messages[1].ToolCalls) != 1 || len(result.Messages[2].ToolResults) != 1 {
		t.Fatalf("history = %+v, want user, assistant call, tool result, final assistant", result.Messages)
	}
	call := result.Messages[1].ToolCalls[0]
	wantResult := ToolResult{CallID: "toolu_1", Content: "1: controlled result"}
	if !reflect.DeepEqual(result.Messages[0], seed) || result.Messages[1].Role != "assistant" || result.Messages[1].Content != "Read values" || result.Messages[2].Role != "tool" || !reflect.DeepEqual(result.Messages[2].ToolResults[0], wantResult) || result.Messages[3].Role != "assistant" || result.Messages[3].Content != "All done" {
		t.Errorf("history messages = %+v", result.Messages)
	}
	if len(hooks) != 1 || len(events) != 1 || len(result.ToolCallDetails) != 1 {
		t.Fatalf("hook/observer/detail counts = %d/%d/%d, want 1/1/1", len(hooks), len(events), len(result.ToolCallDetails))
	}
	if call.ID != "toolu_1" || call.Name != "read_file" || hooks[0].ID != call.ID || hooks[0].Name != call.Name || events[0].Call.ID != call.ID || events[0].Call.Name != call.Name {
		t.Errorf("history/hook/observer calls = %+v/%+v/%+v", call, hooks[0], events[0].Call)
	}
	if ev := events[0]; ev.CallID != call.ID || ev.Action != ToolAllow || ev.IsError || ev.Turn != 0 || ev.ContentLength != len(wantResult.Content) {
		t.Errorf("observer metadata = %+v", ev)
	}
	detail := result.ToolCallDetails[0]
	if detail.Name != call.Name || detail.Output != wantResult.Content || detail.IsError || detail.Turn != 0 {
		t.Errorf("tool detail = %+v", detail)
	}
	var output largeIntegerJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Messages) != 4 || len(output.Messages[1].ToolCalls) != 1 || len(output.ToolCallDetails) != 1 {
		t.Fatalf("JSON history/details = %+v", output)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	var initial, followup struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(<-requests, &initial); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(<-requests, &followup); err != nil {
		t.Fatal(err)
	}
	if initial.Model != "claude-sonnet-4-6" || initial.Stream != stream || len(initial.Messages) != 1 || initial.Messages[0].Role != seed.Role || string(initial.Messages[0].Content) != `"Keep this history"` {
		t.Errorf("initial request = %+v", initial)
	}
	if followup.Model != "claude-sonnet-4-6" || followup.Stream != stream || len(followup.Messages) != 3 || followup.Messages[0].Role != seed.Role || string(followup.Messages[0].Content) != `"Keep this history"` || followup.Messages[1].Role != "assistant" || followup.Messages[2].Role != "user" {
		t.Fatalf("follow-up request = %+v", followup)
	}
	var blocks []struct {
		Type  string            `json:"type"`
		Text  string            `json:"text"`
		ID    string            `json:"id"`
		Name  string            `json:"name"`
		Input map[string]string `json:"input"`
	}
	if err := json.Unmarshal(followup.Messages[1].Content, &blocks); err != nil {
		t.Fatalf("decode replay input as strings: %v", err)
	}
	if len(blocks) != 2 || blocks[0].Type != "text" || blocks[0].Text != "Read values" || blocks[1].Type != "tool_use" || blocks[1].ID != call.ID || blocks[1].Name != call.Name {
		t.Fatalf("replayed assistant blocks = %+v", blocks)
	}
	var results []struct {
		Type      string `json:"type"`
		ToolUseID string `json:"tool_use_id"`
		Content   string `json:"content"`
		IsError   bool   `json:"is_error"`
	}
	if err := json.Unmarshal(followup.Messages[2].Content, &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Type != "tool_result" || results[0].ToolUseID != wantResult.CallID || results[0].Content != wantResult.Content || results[0].IsError {
		t.Errorf("replayed tool result = %+v", results)
	}
	for name, got := range map[string]map[string]string{
		"history": call.Arguments, "hook": hooks[0].Arguments, "observer": events[0].Call.Arguments, "detail": detail.Input,
		"JSON history": output.Messages[1].ToolCalls[0].Arguments, "JSON detail": output.ToolCallDetails[0].Input, "replay": blocks[1].Input,
	} {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s arguments = %v, want %v", name, got, want)
		}
	}
}

func TestLargeIntegerArgs_OpenCodeChat_BufferedHistory(t *testing.T) {
	testLargeIntegerArgsOpenCodeChatHistory(t, false, false)
}

func TestLargeIntegerArgs_OpenCodeChat_StreamHistory(t *testing.T) {
	for _, split := range []bool{false, true} {
		name := "unsplit"
		if split {
			name = "split"
		}
		t.Run(name, func(t *testing.T) { testLargeIntegerArgsOpenCodeChatHistory(t, true, split) })
	}
}

func testLargeIntegerArgsOpenCodeChatHistory(t *testing.T, stream, split bool) {
	t.Helper()
	t.Setenv("OPENCODE_API_KEY", "test-key")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "opencode/gemini-2.5-pro"
tools = ["read_file"]
`)
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("controlled result"), 0600); err != nil {
		t.Fatal(err)
	}
	// Numeric tokens are raw JSON; only the enclosing argument string is encoded.
	const raw = `{"path":"fixture.txt","million":1000000,"signed":-1234567,"precise":9007199254740993,"negative_precise":-9007199254740993,"exponent":1e6,"decimal":1000000.0,"negative":-1e6,"precise_decimal":9007199254740993.0,"precise_exponent":90071992547409930e-1,"beyond":99999999999999999999999,"negative_beyond":-99999999999999999999999,"beyond_decimal":99999999999999999999999.0,"beyond_exponent":999999999999999999999990e-1,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"rounded_fraction":1000000.00000000001,"rounded_threshold":999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"false":false,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50],"recursive":{"a":[1000000,1.50]}}`
	want := map[string]string{
		"path": "fixture.txt", "million": "1000000", "signed": "-1234567", "precise": "9007199254740993", "negative_precise": "-9007199254740993",
		"exponent": "1000000", "decimal": "1000000", "negative": "-1000000", "precise_decimal": "9007199254740993", "precise_exponent": "9007199254740993",
		"beyond": "99999999999999999999999", "negative_beyond": "-99999999999999999999999", "beyond_decimal": "99999999999999999999999", "beyond_exponent": "99999999999999999999999",
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999", "zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "rounded_fraction": "1e+06", "rounded_threshold": "1e+06", "text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993",
		"boolean": "true", "false": "false", "null": "<nil>", "object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]", "array": "[1.5]", "recursive": "map[a:[1e+06 1.5]]",
	}
	quoted, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	toolBody := `{"model":"gemini-2.5-pro","choices":[{"finish_reason":"tool_calls","message":{"content":"Read values","tool_calls":[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":` + string(quoted) + `}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`
	finalBody := `{"model":"gemini-2.5-pro","choices":[{"finish_reason":"stop","message":{"content":"All done"}}],"usage":{"prompt_tokens":12,"completion_tokens":2}}`
	if stream {
		deltas := []string{raw}
		if split {
			at := strings.Index(raw, "740993,")
			if at < 0 {
				t.Fatal("fixture split point missing")
			}
			deltas = []string{raw[:at], raw[at:]}
		}
		var body strings.Builder
		body.WriteString(`data: {"choices":[{"delta":{"content":"Read values"}}]}` + "\n\n")
		body.WriteString(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file"}}]}}]}` + "\n\n")
		for _, delta := range deltas {
			encoded, err := json.Marshal(delta)
			if err != nil {
				t.Fatal(err)
			}
			body.WriteString(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":` + string(encoded) + `}}]}}]}` + "\n\n")
		}
		body.WriteString(`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n" + `data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5}}` + "\n\n" + "data: [DONE]\n\n")
		toolBody = body.String()
		finalBody = `data: {"choices":[{"delta":{"content":"All done"}}]}` + "\n\n" + `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}` + "\n\n" + `data: {"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":2}}` + "\n\n" + "data: [DONE]\n\n"
	}
	bodies := make(chan string, 2)
	bodies <- toolBody
	bodies <- finalBody
	requests := make(chan []byte, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Errorf("request = %s %s, want POST /v1/chat/completions", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		request, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- request
		select {
		case body := <-bodies:
			if stream {
				w.Header().Set("Content-Type", "text/event-stream")
			} else {
				w.Header().Set("Content-Type", "application/json")
			}
			_, _ = io.WriteString(w, body)
		default:
			t.Error("unexpected extra request")
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	var hooks []ToolCall
	var events []ToolEvent
	stdout, stderr := newMockStdoutStderr()
	seed := Message{Role: "user", Content: "Keep this history"}
	result, err := Run(context.Background(), Options{
		AgentName: "test-agent", Workdir: workdir, BaseURL: server.URL, Stream: stream,
		Messages: []Message{seed}, JSON: true, Stdout: stdout, Stderr: stderr,
		ToolHook: func(_ context.Context, call ToolCall) (ToolDecision, error) {
			hooks = append(hooks, call)
			return ToolDecision{Action: ToolAllow}, nil
		},
		ToolObserver: func(_ context.Context, event ToolEvent) { events = append(events, event) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "All done" || result.StopReason != "stop" || result.ToolCalls != 1 {
		t.Errorf("result content/stop/calls = %q/%q/%d", result.Content, result.StopReason, result.ToolCalls)
	}
	if len(result.Messages) != 4 || len(result.Messages[1].ToolCalls) != 1 || len(result.Messages[2].ToolResults) != 1 {
		t.Fatalf("history = %+v, want user, assistant call, tool result, final assistant", result.Messages)
	}
	call := result.Messages[1].ToolCalls[0]
	wantResult := ToolResult{CallID: "call_1", Content: "1: controlled result"}
	if !reflect.DeepEqual(result.Messages[0], seed) || result.Messages[1].Role != "assistant" || result.Messages[1].Content != "Read values" || result.Messages[2].Role != "tool" || !reflect.DeepEqual(result.Messages[2].ToolResults[0], wantResult) || result.Messages[3].Role != "assistant" || result.Messages[3].Content != "All done" {
		t.Errorf("history messages = %+v", result.Messages)
	}
	if len(hooks) != 1 || len(events) != 1 || len(result.ToolCallDetails) != 1 {
		t.Fatalf("hook/observer/detail counts = %d/%d/%d, want 1/1/1", len(hooks), len(events), len(result.ToolCallDetails))
	}
	if call.ID != "call_1" || call.Name != "read_file" || hooks[0].ID != call.ID || hooks[0].Name != call.Name || events[0].Call.ID != call.ID || events[0].Call.Name != call.Name {
		t.Errorf("history/hook/observer calls = %+v/%+v/%+v", call, hooks[0], events[0].Call)
	}
	if ev := events[0]; ev.CallID != call.ID || ev.Action != ToolAllow || ev.IsError || ev.Turn != 0 || ev.ContentLength != len(wantResult.Content) {
		t.Errorf("observer metadata = %+v", ev)
	}
	detail := result.ToolCallDetails[0]
	if detail.Name != call.Name || detail.Output != wantResult.Content || detail.IsError || detail.Turn != 0 {
		t.Errorf("tool detail = %+v", detail)
	}
	var output largeIntegerJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Messages) != 4 || len(output.Messages[1].ToolCalls) != 1 || len(output.ToolCallDetails) != 1 {
		t.Fatalf("JSON history/details = %+v", output)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	var initial, followup struct {
		Model string `json:"model"`
		largeIntegerOpenAIHistoryRequest
	}
	if err := json.Unmarshal(<-requests, &initial); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(<-requests, &followup); err != nil {
		t.Fatal(err)
	}
	if initial.Model != "gemini-2.5-pro" || initial.Stream != stream || len(initial.Messages) != 1 || initial.Messages[0].Role != seed.Role || initial.Messages[0].Content != seed.Content {
		t.Errorf("initial request = %+v", initial)
	}
	if followup.Model != "gemini-2.5-pro" || followup.Stream != stream || len(followup.Messages) != 3 || len(followup.Messages[1].ToolCalls) != 1 {
		t.Fatalf("follow-up request = %+v, want user, assistant call, tool result", followup)
	}
	wireCall := followup.Messages[1].ToolCalls[0]
	if followup.Messages[0].Role != seed.Role || followup.Messages[0].Content != seed.Content || followup.Messages[1].Role != "assistant" || wireCall.ID != call.ID || wireCall.Type != "function" || wireCall.Function.Name != call.Name || followup.Messages[2].Role != "tool" || followup.Messages[2].ToolCallID != wantResult.CallID || followup.Messages[2].Content != wantResult.Content {
		t.Errorf("follow-up messages = %+v", followup.Messages)
	}
	var replayed map[string]string
	if err := json.Unmarshal([]byte(wireCall.Function.Arguments), &replayed); err != nil {
		t.Fatalf("decode replay arguments as strings: %v", err)
	}
	for name, got := range map[string]map[string]string{
		"history": call.Arguments, "hook": hooks[0].Arguments, "observer": events[0].Call.Arguments, "detail": detail.Input,
		"JSON history": output.Messages[1].ToolCalls[0].Arguments, "JSON detail": output.ToolCallDetails[0].Input, "replay": replayed,
	} {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s arguments = %v, want %v", name, got, want)
		}
	}
}

func TestLargeIntegerArgs_OpenCodeResponses_BufferedHistory(t *testing.T) {
	testLargeIntegerArgsOpenCodeResponsesHistory(t, false, false)
}

func TestLargeIntegerArgs_OpenCodeResponses_StreamHistory(t *testing.T) {
	for _, split := range []bool{false, true} {
		name := "unsplit"
		if split {
			name = "split"
		}
		t.Run(name, func(t *testing.T) { testLargeIntegerArgsOpenCodeResponsesHistory(t, true, split) })
	}
}

func testLargeIntegerArgsOpenCodeResponsesHistory(t *testing.T, stream, split bool) {
	t.Helper()
	t.Setenv("OPENCODE_API_KEY", "test-key")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "opencode/gpt-5"
tools = ["read_file"]
`)
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "fixture.txt"), []byte("controlled result"), 0600); err != nil {
		t.Fatal(err)
	}
	// The arguments string is quoted for the Responses wire, but numeric tokens
	// inside it are independent raw JSON, never float64-derived fixture values.
	const raw = `{"path":"fixture.txt","million":1000000,"signed":-1234567,"precise":9007199254740993,"negative_precise":-9007199254740993,"exponent":1e6,"decimal":1000000.0,"negative":-1e6,"precise_decimal":9007199254740993.0,"precise_exponent":90071992547409930e-1,"beyond":99999999999999999999999,"negative_beyond":-99999999999999999999999,"beyond_decimal":99999999999999999999999.0,"beyond_exponent":999999999999999999999990e-1,"small":999999,"negative_small":-999999,"small_decimal":999999.0,"zero":0,"negative_zero":-0,"negative_decimal_zero":-0.0,"fraction":1.5,"rounded_fraction":1000000.00000000001,"rounded_threshold":999999.99999999999,"text":"text","string_exponent":"1e6","string_decimal":"1000000.0","string_precise":"9007199254740993","boolean":true,"false":false,"null":null,"object_exponent":{"a":1e6},"object_integer":{"a":1000000},"array":[1.50],"recursive":{"a":[1000000,1.50]}}`
	want := map[string]string{
		"path": "fixture.txt", "million": "1000000", "signed": "-1234567", "precise": "9007199254740993", "negative_precise": "-9007199254740993",
		"exponent": "1000000", "decimal": "1000000", "negative": "-1000000", "precise_decimal": "9007199254740993", "precise_exponent": "9007199254740993",
		"beyond": "99999999999999999999999", "negative_beyond": "-99999999999999999999999", "beyond_decimal": "99999999999999999999999", "beyond_exponent": "99999999999999999999999",
		"small": "999999", "negative_small": "-999999", "small_decimal": "999999", "zero": "0", "negative_zero": "-0", "negative_decimal_zero": "-0",
		"fraction": "1.5", "rounded_fraction": "1e+06", "rounded_threshold": "1e+06", "text": "text", "string_exponent": "1e6", "string_decimal": "1000000.0", "string_precise": "9007199254740993",
		"boolean": "true", "false": "false", "null": "<nil>", "object_exponent": "map[a:1e+06]", "object_integer": "map[a:1e+06]", "array": "[1.5]", "recursive": "map[a:[1e+06 1.5]]",
	}
	quoted, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	toolBody := `{"model":"gpt-5","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Read values"}]},{"type":"function_call","id":"call_1","name":"read_file","arguments":` + string(quoted) + `}],"usage":{"input_tokens":10,"output_tokens":5}}`
	finalBody := `{"model":"gpt-5","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"All done"}]}],"usage":{"input_tokens":12,"output_tokens":2}}`
	if stream {
		deltas := []string{raw}
		if split {
			at := strings.Index(raw, "740993,")
			if at < 0 {
				t.Fatal("fixture split point missing")
			}
			deltas = []string{raw[:at], raw[at:]}
		}
		var body strings.Builder
		body.WriteString(`data: {"type":"response.content_part.delta","delta":"Read values"}` + "\n\n")
		body.WriteString(`data: {"type":"response.output_item.added","item":{"type":"function_call","id":"call_1","name":"read_file"}}` + "\n\n")
		for _, delta := range deltas {
			encoded, err := json.Marshal(delta)
			if err != nil {
				t.Fatal(err)
			}
			body.WriteString(`data: {"type":"response.function_call_arguments.delta","item_id":"call_1","delta":` + string(encoded) + `}` + "\n\n")
		}
		body.WriteString(`data: {"type":"response.output_item.done","item":{"type":"function_call","id":"call_1"}}` + "\n\n")
		body.WriteString(`data: {"type":"response.completed","response":{"usage":{"input_tokens":10,"output_tokens":5}}}` + "\n\n" + "data: [DONE]\n\n")
		toolBody = body.String()
		finalBody = `data: {"type":"response.content_part.delta","delta":"All done"}` + "\n\n" + `data: {"type":"response.completed","response":{"usage":{"input_tokens":12,"output_tokens":2}}}` + "\n\n" + "data: [DONE]\n\n"
	}
	bodies := make(chan string, 2)
	bodies <- toolBody
	bodies <- finalBody
	requests := make(chan []byte, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
			t.Errorf("request = %s %s, want POST /v1/responses", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		request, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- request
		select {
		case body := <-bodies:
			if stream {
				w.Header().Set("Content-Type", "text/event-stream")
			} else {
				w.Header().Set("Content-Type", "application/json")
			}
			_, _ = io.WriteString(w, body)
		default:
			t.Error("unexpected extra request")
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	var hooks []ToolCall
	var events []ToolEvent
	stdout, stderr := newMockStdoutStderr()
	seed := Message{Role: "user", Content: "Keep this history"}
	result, err := Run(context.Background(), Options{
		AgentName: "test-agent", Workdir: workdir, BaseURL: server.URL, Stream: stream,
		Messages: []Message{seed}, JSON: true, Stdout: stdout, Stderr: stderr,
		ToolHook: func(_ context.Context, call ToolCall) (ToolDecision, error) {
			hooks = append(hooks, call)
			return ToolDecision{Action: ToolAllow}, nil
		},
		ToolObserver: func(_ context.Context, event ToolEvent) { events = append(events, event) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "All done" || result.ToolCalls != 1 {
		t.Errorf("result content/calls = %q/%d", result.Content, result.ToolCalls)
	}
	if len(result.Messages) != 4 || len(result.Messages[1].ToolCalls) != 1 || len(result.Messages[2].ToolResults) != 1 {
		t.Fatalf("history = %+v, want user, assistant call, tool result, final assistant", result.Messages)
	}
	call := result.Messages[1].ToolCalls[0]
	wantResult := ToolResult{CallID: "call_1", Content: "1: controlled result"}
	if !reflect.DeepEqual(result.Messages[0], seed) || result.Messages[1].Role != "assistant" || result.Messages[1].Content != "Read values" || result.Messages[2].Role != "tool" || !reflect.DeepEqual(result.Messages[2].ToolResults[0], wantResult) || result.Messages[3].Role != "assistant" || result.Messages[3].Content != "All done" {
		t.Errorf("history messages = %+v", result.Messages)
	}
	if len(hooks) != 1 || len(events) != 1 || len(result.ToolCallDetails) != 1 {
		t.Fatalf("hook/observer/detail counts = %d/%d/%d, want 1/1/1", len(hooks), len(events), len(result.ToolCallDetails))
	}
	if call.ID != "call_1" || call.Name != "read_file" || hooks[0].ID != call.ID || hooks[0].Name != call.Name || events[0].Call.ID != call.ID || events[0].Call.Name != call.Name {
		t.Errorf("history/hook/observer calls = %+v/%+v/%+v", call, hooks[0], events[0].Call)
	}
	if ev := events[0]; ev.CallID != call.ID || ev.Action != ToolAllow || ev.IsError || ev.Turn != 0 || ev.ContentLength != len(wantResult.Content) {
		t.Errorf("observer metadata = %+v", ev)
	}
	detail := result.ToolCallDetails[0]
	if detail.Name != call.Name || detail.Output != wantResult.Content || detail.IsError || detail.Turn != 0 {
		t.Errorf("tool detail = %+v", detail)
	}
	var output largeIntegerJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Messages) != 4 || len(output.Messages[1].ToolCalls) != 1 || len(output.ToolCallDetails) != 1 {
		t.Fatalf("JSON history/details = %+v", output)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	var initial, followup struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
		Input  []struct {
			Role       string `json:"role"`
			Content    string `json:"content"`
			ToolCallID string `json:"tool_call_id"`
			ToolCalls  []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"input"`
	}
	if err := json.Unmarshal(<-requests, &initial); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(<-requests, &followup); err != nil {
		t.Fatal(err)
	}
	if initial.Model != "gpt-5" || initial.Stream != stream || len(initial.Input) != 1 || initial.Input[0].Role != seed.Role || initial.Input[0].Content != seed.Content {
		t.Errorf("initial request = %+v", initial)
	}
	if followup.Model != "gpt-5" || followup.Stream != stream || len(followup.Input) != 3 || len(followup.Input[1].ToolCalls) != 1 {
		t.Fatalf("follow-up request = %+v, want user, assistant call, tool result", followup)
	}
	wireCall := followup.Input[1].ToolCalls[0]
	if followup.Input[0].Role != seed.Role || followup.Input[0].Content != seed.Content || followup.Input[1].Role != "assistant" || followup.Input[1].Content != "Read values" || wireCall.ID != call.ID || wireCall.Type != "function" || wireCall.Function.Name != call.Name || followup.Input[2].Role != "tool" || followup.Input[2].ToolCallID != wantResult.CallID || followup.Input[2].Content != wantResult.Content {
		t.Errorf("follow-up input = %+v", followup.Input)
	}
	var replayed map[string]string
	if err := json.Unmarshal([]byte(wireCall.Function.Arguments), &replayed); err != nil {
		t.Fatalf("decode replay arguments as strings: %v", err)
	}
	for name, got := range map[string]map[string]string{
		"history": call.Arguments, "hook": hooks[0].Arguments, "observer": events[0].Call.Arguments, "detail": detail.Input,
		"JSON history": output.Messages[1].ToolCalls[0].Arguments, "JSON detail": output.ToolCallDetails[0].Input, "replay": replayed,
	} {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s arguments = %v, want %v", name, got, want)
		}
	}
}

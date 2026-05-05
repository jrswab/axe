package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/testutil"
)

// --- Integration Tests ---

func TestIntegration_Streaming_SingleShot_OpenAI(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIStreamResponse("Hello streamed", 10, 5),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "stream-single", `name = "stream-single"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "stream-single", "--stream"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nstderr: %s", err, errBuf.String())
	}

	output := buf.String()
	if output != "Hello streamed" {
		t.Errorf("expected stdout %q, got %q", "Hello streamed", output)
	}
}

func TestIntegration_Streaming_JSON_Buffers(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIStreamResponse("buffered text", 10, 5),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "stream-json", `name = "stream-json"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "stream-json", "--stream", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nstderr: %s", err, errBuf.String())
	}

	output := buf.String()
	var envelope map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(output)), &envelope); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", jsonErr, output)
	}

	content, _ := envelope["content"].(string)
	if content != "buffered text" {
		t.Errorf("JSON content = %q, want %q", content, "buffered text")
	}
}

func TestIntegration_Streaming_SingleShot_Anthropic(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicStreamResponse("Fallback works", 10, 5),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "stream-anthropic", `name = "stream-anthropic"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "stream-anthropic", "--stream"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nstderr: %s", err, errBuf.String())
	}

	output := buf.String()
	if output != "Fallback works" {
		t.Errorf("expected stdout %q, got %q", "Fallback works", output)
	}
}

func TestIntegration_Streaming_WithTools(t *testing.T) {
	resetRunCmd(t)

	// First response: tool call via streaming
	// Second response: final text (non-streaming is fine since loop checks each turn)
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIStreamToolCallResponse("", []testutil.MockToolCall{
			{ID: "call_1", Name: "list_directory", Input: map[string]string{"path": "."}},
		}, 10, 15),
		testutil.OpenAIStreamResponse("Done listing", 5, 3),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	workdir := t.TempDir()
	// Create a file so list_directory has something to find
	if err := os.WriteFile(workdir+"/test.txt", []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	writeAgentConfig(t, configDir, "stream-tools", fmt.Sprintf(`name = "stream-tools"
model = "openai/gpt-4o"
tools = ["list_directory"]
workdir = %q
`, workdir))

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "stream-tools", "--stream"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nstderr: %s", err, errBuf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "Done listing") {
		t.Errorf("expected stdout to contain %q, got %q", "Done listing", output)
	}

	if mock.RequestCount() != 2 {
		t.Errorf("expected 2 requests (tool call + final), got %d", mock.RequestCount())
	}
}

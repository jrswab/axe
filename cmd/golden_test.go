package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/testutil"
)

// ---------------------------------------------------------------------------
// Update flag (Phase A)
// ---------------------------------------------------------------------------

var updateGolden = flag.Bool("update-golden", false, "Regenerate golden files from current output")

func isUpdateGolden() bool {
	if *updateGolden {
		return true
	}
	return os.Getenv("UPDATE_GOLDEN") == "1"
}

// ---------------------------------------------------------------------------
// Masking types and rules (Phase B)
// ---------------------------------------------------------------------------

// maskRule defines a single non-deterministic value replacement.
type maskRule struct {
	// Pattern is a compiled regexp matching the non-deterministic value.
	Pattern *regexp.Regexp
	// Replacement is the fixed placeholder string.
	Replacement string
}

// maskOutput applies a sequence of masking rules to the given output string.
// Rules are applied in order; each rule operates on the result of the previous.
func maskOutput(output string, rules []maskRule) string {
	for _, r := range rules {
		output = r.Pattern.ReplaceAllString(output, r.Replacement)
	}
	return output
}

// maskDryRunOutput replaces non-deterministic values in --dry-run output.
// Currently masks the workdir path after "Workdir:  " with {{WORKDIR}}.
func maskDryRunOutput(output string, workdir string) string {
	if workdir == "" {
		return output
	}
	rules := []maskRule{
		{
			Pattern:     regexp.MustCompile(regexp.QuoteMeta(workdir)),
			Replacement: "{{WORKDIR}}",
		},
	}
	return maskOutput(output, rules)
}

// maskJSONOutput replaces non-deterministic values in --json output.
// Parses JSON, replaces duration_ms with placeholder, re-serializes prettily.
func maskJSONOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
		// If we can't parse, fall back to regex replacement.
		re := regexp.MustCompile(`"duration_ms":\s*\d+`)
		return re.ReplaceAllString(trimmed, `"duration_ms":"{{DURATION_MS}}"`)
	}

	envelope["duration_ms"] = "{{DURATION_MS}}"
	if details, ok := envelope["tool_call_details"].([]interface{}); ok {
		for _, raw := range details {
			entry, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if _, hasOutput := entry["output"]; hasOutput {
				entry["output"] = "{{TOOL_OUTPUT}}"
			}
			if _, hasDuration := entry["duration_ms"]; hasDuration {
				entry["duration_ms"] = "{{TOOL_DURATION_MS}}"
			}
		}
	}

	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return trimmed
	}
	return string(data) + "\n"
}

// ---------------------------------------------------------------------------
// Golden file I/O helpers (Phase C)
// ---------------------------------------------------------------------------

// readGoldenFile reads the golden file at path. It fails the test if the file
// is missing or empty, with messages matching the spec.
func readGoldenFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("golden file not found: %s. Run with -update-golden to generate.", path)
		}
		t.Fatalf("failed to read golden file %s: %v", path, err)
	}

	if len(data) == 0 {
		t.Fatalf("golden file is empty: %s. Run with -update-golden to regenerate.", path)
	}

	return string(data)
}

// writeGoldenFile writes content to the golden file at path, creating parent
// directories as needed.
func writeGoldenFile(t *testing.T, path string, content string) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create golden file directory %s: %v", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write golden file %s: %v", path, err)
	}

	t.Logf("wrote golden file: %s", path)
}

// diffStrings returns a human-readable unified-style diff between expected and
// actual strings. Returns empty string if they are equal.
func diffStrings(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var diff strings.Builder
	maxLines := len(expectedLines)
	if len(actualLines) > maxLines {
		maxLines = len(actualLines)
	}

	hasDiff := false
	for i := 0; i < maxLines; i++ {
		var eLine, aLine string
		haveE, haveA := i < len(expectedLines), i < len(actualLines)

		if haveE {
			eLine = expectedLines[i]
		}
		if haveA {
			aLine = actualLines[i]
		}

		if haveE && haveA && eLine == aLine {
			fmt.Fprintf(&diff, " %s\n", eLine)
		} else {
			hasDiff = true
			if haveE {
				fmt.Fprintf(&diff, "-%s\n", eLine)
			}
			if haveA {
				fmt.Fprintf(&diff, "+%s\n", aLine)
			}
		}
	}

	if !hasDiff {
		return ""
	}
	return diff.String()
}

// ---------------------------------------------------------------------------
// Golden test runner (Phases D & E)
// ---------------------------------------------------------------------------

// goldenTestCase defines a single golden file test entry.
type goldenTestCase struct {
	agent      string
	mode       string // "dry-run" or "json"
	goldenPath string
	// For JSON tests: mock responses and env overrides.
	mockResponses []testutil.MockLLMResponse
	extraEnv      map[string]string
}

func goldenPath(mode, agent, ext string) string {
	return filepath.Join("testdata", "golden", mode, agent+ext)
}

func TestGolden(t *testing.T) {
	t.Parallel()

	// Build the list of dry-run test cases.
	agents := []string{"basic", "with_skill", "with_files", "with_memory", "with_subagents", "with_tools"}

	var cases []goldenTestCase

	// Dry-run cases (no mock server needed).
	for _, a := range agents {
		cases = append(cases, goldenTestCase{
			agent:      a,
			mode:       "dry-run",
			goldenPath: goldenPath("dry-run", a, ".txt"),
		})
	}

	// JSON cases (need mock server).
	for _, a := range agents {
		tc := goldenTestCase{
			agent:      a,
			mode:       "json",
			goldenPath: goldenPath("json", a, ".json"),
			extraEnv:   map[string]string{},
		}

		switch a {
		case "with_subagents":
			// Parent is anthropic, children (basic, with_skill) are openai.
			// Parent calls tool_use -> 2 child calls -> parent final.
			tc.mockResponses = []testutil.MockLLMResponse{
				testutil.AnthropicToolUseResponse("Delegating to sub-agents.", []testutil.MockToolCall{
					{ID: "tc_1", Name: "call_agent", Input: map[string]string{"agent": "basic", "task": "hello"}},
					{ID: "tc_2", Name: "call_agent", Input: map[string]string{"agent": "with_skill", "task": "hello"}},
				}),
				testutil.OpenAIResponse("Hello from basic."),
				testutil.OpenAIResponse("Hello from with_skill."),
				testutil.AnthropicResponse("Final answer from sub-agents."),
			}
		case "with_tools":
			// OpenAI agent with tools. Turn 1: tool call, Turn 2: final response.
			tc.mockResponses = []testutil.MockLLMResponse{
				testutil.OpenAIToolCallResponse("Let me list the directory.", []testutil.MockToolCall{
					{ID: "tc_1", Name: "list_directory", Input: map[string]string{"path": "."}},
				}),
				testutil.OpenAIResponse("Directory listed successfully."),
			}
		default:
			// All other agents use openai.
			tc.mockResponses = []testutil.MockLLMResponse{
				testutil.OpenAIResponse("Hello from mock."),
			}
		}

		cases = append(cases, tc)
	}

	for _, tc := range cases {
		tc := tc // capture
		t.Run(tc.mode+"/"+tc.agent, func(t *testing.T) {
			t.Parallel()

			configDir, _, env := setupSmokeEnv(t)
			testutil.SeedFixtureAgents(t, "testdata/agents", filepath.Join(configDir, "agents"))
			testutil.SeedFixtureSkills(t, "testdata/skills", filepath.Join(configDir, "skills"))

			if tc.mode == "dry-run" {
				stripAPIKeys(env)
				stdout, stderr, exitCode := runAxe(t, env, "", "run", tc.agent, "--dry-run")
				if exitCode != 0 {
					t.Fatalf("dry-run exited %d; stderr: %s", exitCode, stderr)
				}

				// The binary inherits the test process CWD (the cmd/ dir).
				// Mask the CWD so golden files are portable across machines.
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatalf("failed to get working directory: %v", err)
				}
				masked := maskDryRunOutput(stdout, cwd)

				compareOrUpdate(t, tc.goldenPath, masked)
			} else {
				// JSON mode: start mock server.
				mock := testutil.NewMockLLMServer(t, tc.mockResponses)

				// Set up provider URLs and keys.
				env["AXE_OPENAI_BASE_URL"] = mock.URL()
				env["OPENAI_API_KEY"] = "test-key"
				env["AXE_ANTHROPIC_BASE_URL"] = mock.URL()
				env["ANTHROPIC_API_KEY"] = "test-key"

				for k, v := range tc.extraEnv {
					env[k] = v
				}

				stdout, stderr, exitCode := runAxe(t, env, "", "run", tc.agent, "--json")
				if exitCode != 0 {
					t.Fatalf("json run exited %d; stderr: %s", exitCode, stderr)
				}

				masked := maskJSONOutput(stdout)
				compareOrUpdate(t, tc.goldenPath, masked)
			}
		})
	}
}

// compareOrUpdate either updates the golden file or compares against it.
func compareOrUpdate(t *testing.T, goldenFilePath, actual string) {
	t.Helper()

	// Normalize trailing whitespace/newlines.
	actual = strings.TrimRight(actual, " \t\r\n") + "\n"

	if isUpdateGolden() {
		writeGoldenFile(t, goldenFilePath, actual)
		return
	}

	expected := readGoldenFile(t, goldenFilePath)
	expected = strings.TrimRight(expected, " \t\r\n") + "\n"

	if expected == actual {
		return
	}

	d := diffStrings(expected, actual)
	t.Fatalf("golden file mismatch: %s\n\nDiff (- expected, + actual):\n%s\nRun with -update-golden to regenerate.", goldenFilePath, d)
}

// ---------------------------------------------------------------------------
// Unit tests for masking helpers (Phase B tests)
// ---------------------------------------------------------------------------

func TestMaskDryRunOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		workdir string
		want    string
	}{
		{
			name:    "replaces workdir path",
			input:   "Workdir:  /tmp/test-abc123/config/axe\nModel:    openai/gpt-4o\n",
			workdir: "/tmp/test-abc123",
			want:    "Workdir:  {{WORKDIR}}/config/axe\nModel:    openai/gpt-4o\n",
		},
		{
			name:    "replaces multiple occurrences",
			input:   "Path: /tmp/xyz\nAgain: /tmp/xyz/sub\n",
			workdir: "/tmp/xyz",
			want:    "Path: {{WORKDIR}}\nAgain: {{WORKDIR}}/sub\n",
		},
		{
			name:    "no match leaves output unchanged",
			input:   "Workdir:  /other/path\nModel:    openai/gpt-4o\n",
			workdir: "/tmp/no-match",
			want:    "Workdir:  /other/path\nModel:    openai/gpt-4o\n",
		},
		{
			name:    "empty workdir does not corrupt output",
			input:   "some output\n",
			workdir: "",
			want:    "some output\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := maskDryRunOutput(tt.input, tt.workdir)
			if got != tt.want {
				t.Errorf("maskDryRunOutput()\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestMaskJSONOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		check func(t *testing.T, got string)
	}{
		{
			name:  "masks tool_call_details outputs",
			input: `{"model":"gpt-4o","duration_ms":1,"tool_call_details":[{"name":"read_file","input":{"path":"a.txt"},"output":"dynamic text","is_error":false}]}`,
			check: func(t *testing.T, got string) {
				if !strings.Contains(got, `"output": "{{TOOL_OUTPUT}}"`) {
					t.Errorf("expected tool_call_details output to be masked, got:\n%s", got)
				}
			},
		},
		{
			name:  "masks duration_ms",
			input: `{"model":"gpt-4o","content":"hello","duration_ms":42,"input_tokens":10,"output_tokens":5,"stop_reason":"stop","tool_calls":0}`,
			check: func(t *testing.T, got string) {
				if !strings.Contains(got, `"duration_ms": "{{DURATION_MS}}"`) {
					t.Errorf("expected duration_ms placeholder, got:\n%s", got)
				}
				if !strings.Contains(got, `"model": "gpt-4o"`) {
					t.Errorf("expected model field preserved, got:\n%s", got)
				}
				if !strings.Contains(got, `"content": "hello"`) {
					t.Errorf("expected content field preserved, got:\n%s", got)
				}
			},
		},
		{
			name:  "output is pretty-printed with 2-space indent",
			input: `{"model":"gpt-4o","duration_ms":100}`,
			check: func(t *testing.T, got string) {
				if !strings.Contains(got, "  ") {
					t.Errorf("expected 2-space indentation, got:\n%s", got)
				}
				// Should end with newline.
				if !strings.HasSuffix(got, "\n") {
					t.Errorf("expected trailing newline, got:\n%q", got)
				}
			},
		},
		{
			name:  "masks tool_call_details duration_ms",
			input: `{"model":"gpt-4o","duration_ms":1,"tool_call_details":[{"name":"read_file","input":{"path":"a.txt"},"output":"text","is_error":false,"turn":0,"duration_ms":42}]}` ,
			check: func(t *testing.T, got string) {
				if !strings.Contains(got, `"duration_ms": "{{TOOL_DURATION_MS}}"`) {
					t.Errorf("expected tool_call_details duration_ms to be masked, got:\n%s", got)
				}
				// turn should NOT be masked - it should remain a number
				if strings.Contains(got, `"turn": "{{`) {
					t.Errorf("expected turn to NOT be masked, got:\n%s", got)
				}
				if !strings.Contains(got, `"turn": 0`) {
					t.Errorf("expected turn to be preserved as number 0, got:\n%s", got)
				}
			},
		},
		{
			name:  "empty input returns empty",
			input: "",
			check: func(t *testing.T, got string) {
				if got != "" {
					t.Errorf("expected empty string, got: %q", got)
				}
			},
		},
		{
			name:  "other fields preserved",
			input: `{"model":"gpt-4o","content":"test","input_tokens":10,"output_tokens":5,"stop_reason":"stop","duration_ms":99,"tool_calls":0}`,
			check: func(t *testing.T, got string) {
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(got), &parsed); err != nil {
					t.Fatalf("output is not valid JSON: %v\n%s", err, got)
				}
				if parsed["model"] != "gpt-4o" {
					t.Errorf("model field changed: %v", parsed["model"])
				}
				if parsed["content"] != "test" {
					t.Errorf("content field changed: %v", parsed["content"])
				}
				if parsed["duration_ms"] != "{{DURATION_MS}}" {
					t.Errorf("duration_ms not masked: %v", parsed["duration_ms"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := maskJSONOutput(tt.input)
			tt.check(t, got)
		})
	}
}

func TestDiffStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		actual   string
		wantDiff bool
	}{
		{
			name:     "identical strings produce no diff",
			expected: "line1\nline2\n",
			actual:   "line1\nline2\n",
			wantDiff: false,
		},
		{
			name:     "different strings produce diff",
			expected: "line1\nline2\n",
			actual:   "line1\nchanged\n",
			wantDiff: true,
		},
		{
			name:     "extra lines in actual",
			expected: "line1\n",
			actual:   "line1\nline2\n",
			wantDiff: true,
		},
		{
			name:     "missing lines in actual",
			expected: "line1\nline2\n",
			actual:   "line1\n",
			wantDiff: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := diffStrings(tt.expected, tt.actual)
			if tt.wantDiff && d == "" {
				t.Error("expected non-empty diff, got empty")
			}
			if !tt.wantDiff && d != "" {
				t.Errorf("expected empty diff, got:\n%s", d)
			}
		})
	}
}

func TestReadGoldenFile_Missing(t *testing.T) {
	t.Parallel()

	// We can't directly test t.Fatalf, but we can verify the function
	// works correctly with an existing file.
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.golden")
	content := "expected content\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := readGoldenFile(t, path)
	if got != content {
		t.Errorf("readGoldenFile() = %q, want %q", got, content)
	}
}

func TestWriteGoldenFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sub", "dir", "test.golden")
	content := "golden content\n"

	writeGoldenFile(t, path, content)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written golden file: %v", err)
	}
	if string(got) != content {
		t.Errorf("written content = %q, want %q", string(got), content)
	}
}

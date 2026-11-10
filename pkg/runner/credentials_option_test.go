package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/testutil"
)

const credentialsOptionAgentTOML = `name = "test-agent"
model = "anthropic/claude-sonnet-4-20250514"
`

func TestCredentialsOption_LiveRunUsesOptions(t *testing.T) {
	setupTestEnv(t, "", "test-agent", credentialsOptionAgentTOML)
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", "")
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{testutil.AnthropicResponse("ok")})

	stdout, stderr := newMockStdoutStderr()
	_, err := Run(context.Background(), Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		APIKey:    "sk-options-key-1",
		BaseURL:   mock.URL(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := mock.RequestCount(); got != 1 {
		t.Fatalf("RequestCount = %d, want 1", got)
	}
	if mock.Requests[0].Headers.Get("x-api-key") != "sk-options-key-1" {
		t.Error("x-api-key does not match Options.APIKey")
	}
}

func TestCredentialsOption_OptionsWinOverEnv(t *testing.T) {
	setupTestEnv(t,
		`[providers.anthropic]
api_key = "config-key"
base_url = "http://localhost:1"
`,
		"test-agent", credentialsOptionAgentTOML,
	)
	t.Setenv("ANTHROPIC_API_KEY", "env-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", "http://localhost:1")
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{testutil.AnthropicResponse("ok")})

	stdout, stderr := newMockStdoutStderr()
	_, err := Run(context.Background(), Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		APIKey:    "sk-options-key-2",
		BaseURL:   mock.URL(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := mock.RequestCount(); got != 1 {
		t.Fatalf("RequestCount = %d, want 1", got)
	}
	if mock.Requests[0].Headers.Get("x-api-key") != "sk-options-key-2" {
		t.Error("x-api-key does not match Options.APIKey")
	}
}

func TestCredentialsOption_ParallelRun(t *testing.T) {
	t.Parallel()
	agentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(agentDir, "test-agent.toml"), []byte(credentialsOptionAgentTOML), 0644); err != nil {
		t.Fatalf("failed to write agent toml: %v", err)
	}
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{testutil.AnthropicResponse("ok")})

	stdout, stderr := newMockStdoutStderr()
	_, err := Run(context.Background(), Options{
		AgentName:  "test-agent",
		AgentsDirs: []string{agentDir},
		Stdout:     stdout,
		Stderr:     stderr,
		APIKey:     "sk-options-key-4",
		BaseURL:    mock.URL(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := mock.RequestCount(); got != 1 {
		t.Fatalf("RequestCount = %d, want 1", got)
	}
	if mock.Requests[0].Headers.Get("x-api-key") != "sk-options-key-4" {
		t.Error("x-api-key does not match Options.APIKey")
	}
}

func TestCredentialsOption_NotPrinted(t *testing.T) {
	const keyProbe = "sk-options-secret-probe"
	const baseURLProbe = "http://options-base-url-probe:9"
	tests := []struct {
		name    string
		baseURL string
		dryRun  bool
	}{
		{name: "live"},
		{name: "live-fail", baseURL: baseURLProbe},
		{name: "dry", baseURL: baseURLProbe, dryRun: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ANTHROPIC_API_KEY", "")
			t.Setenv("AXE_ANTHROPIC_BASE_URL", "")
			setupTestEnv(t, "", "test-agent", credentialsOptionAgentTOML)
			baseURL := tt.baseURL
			if tt.name == "live" {
				mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{testutil.AnthropicResponse("ok")})
				baseURL = mock.URL()
			}
			stdout, stderr := newMockStdoutStderr()
			_, err := Run(context.Background(), Options{
				AgentName: "test-agent",
				Stdout:    stdout,
				Stderr:    stderr,
				APIKey:    keyProbe,
				BaseURL:   baseURL,
				Verbose:   true,
				DryRun:    tt.dryRun,
			})
			if tt.name == "live-fail" {
				if err == nil {
					t.Fatal("Run() expected transport error, got nil")
				}
				if strings.Contains(err.Error(), keyProbe) {
					t.Error("Run() error contains Options.APIKey")
				}
			} else if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			for _, output := range []struct {
				name string
				text string
			}{
				{name: "stdout", text: stdout.String()},
				{name: "stderr", text: stderr.String()},
			} {
				if strings.Contains(output.text, keyProbe) {
					t.Errorf("%s contains Options.APIKey", output.name)
				}
				if strings.Contains(output.text, baseURL) {
					t.Errorf("%s contains Options.BaseURL", output.name)
				}
			}
		})
	}
}

func TestCredentialsOption_UnsetUsesLookup(t *testing.T) {
	setupTestEnv(t,
		`[providers.anthropic]
api_key = "config-key"
`,
		"test-agent", credentialsOptionAgentTOML,
	)
	t.Setenv("ANTHROPIC_API_KEY", "")
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{testutil.AnthropicResponse("ok")})
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	stdout, stderr := newMockStdoutStderr()
	_, err := Run(context.Background(), Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		APIKey:    "",
		BaseURL:   "",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := mock.RequestCount(); got != 1 {
		t.Fatalf("RequestCount = %d, want 1", got)
	}
	if mock.Requests[0].Headers.Get("x-api-key") != "config-key" {
		t.Error("x-api-key does not match config.toml api_key")
	}
}

func TestCredentialsOption_BedrockIgnoresOptions(t *testing.T) {
	setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "bedrock/claude-test-model"
`)
	t.Setenv("AWS_REGION", "us-west-2")
	var gotName, gotKey, gotBaseURL string
	newProvider := func(name, key, baseURL string) (provider.Provider, error) {
		gotName, gotKey, gotBaseURL = name, key, baseURL
		return nil, errors.New("provider creation sentinel")
	}

	stdout, stderr := newMockStdoutStderr()
	_, err := run(context.Background(), Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		APIKey:    "fake",
		BaseURL:   "http://localhost:1",
	}, newProvider)
	if !IsRuntimeError(err) {
		t.Fatalf("expected RuntimeError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "failed to create provider") {
		t.Errorf("error message missing 'failed to create provider': %v", err)
	}
	if gotName != "bedrock" {
		t.Errorf("provider = %q, want bedrock", gotName)
	}
	if gotKey != "us-west-2" {
		t.Error("provider key does not match AWS_REGION")
	}
	if gotBaseURL != "" {
		t.Error("provider base URL is not empty")
	}
}

func TestCredentialsOption_BedrockNoRegionIsConfigError(t *testing.T) {
	setupTestEnv(t, "", "test-agent", `name = "test-agent"
model = "bedrock/claude-test-model"
`)
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AXE_BEDROCK_REGION", "")
	newProvider := func(_, _, _ string) (provider.Provider, error) {
		t.Fatal("run() must not create provider without a bedrock region")
		return nil, nil
	}

	stdout, stderr := newMockStdoutStderr()
	_, err := run(context.Background(), Options{
		AgentName: "test-agent",
		Stdout:    stdout,
		Stderr:    stderr,
		APIKey:    "fake",
	}, newProvider)
	if !IsConfigError(err) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), `region for provider "bedrock" is not configured`) {
		t.Errorf("error message missing bedrock region configuration error: %v", err)
	}
}

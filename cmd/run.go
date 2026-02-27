package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/resolve"
	"github.com/jrswab/axe/internal/xdg"
	"github.com/spf13/cobra"
)

// defaultUserMessage is sent when no stdin content is piped.
const defaultUserMessage = "Execute the task described in your instructions."

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run an agent",
	Long: `Run an agent by loading its TOML configuration, resolving all runtime
context (working directory, file globs, skill, stdin), building a prompt,
calling the LLM provider, and printing the response.`,
	Args: cobra.ExactArgs(1),
	RunE: runAgent,
}

func init() {
	runCmd.Flags().String("skill", "", "Override the agent's default skill path")
	runCmd.Flags().String("workdir", "", "Override the working directory")
	runCmd.Flags().String("model", "", "Override the model (provider/model-name format)")
	runCmd.Flags().Int("timeout", 120, "Request timeout in seconds")
	runCmd.Flags().Bool("dry-run", false, "Show resolved context without calling the LLM")
	runCmd.Flags().BoolP("verbose", "v", false, "Print debug info to stderr")
	runCmd.Flags().Bool("json", false, "Wrap output in JSON with metadata")
	rootCmd.AddCommand(runCmd)
}

// parseModel splits a "provider/model-name" string into provider and model parts.
func parseModel(model string) (providerName, modelName string, err error) {
	idx := strings.Index(model, "/")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid model format %q: expected provider/model-name", model)
	}

	providerName = model[:idx]
	modelName = model[idx+1:]

	if providerName == "" {
		return "", "", fmt.Errorf("invalid model format %q: empty provider", model)
	}
	if modelName == "" {
		return "", "", fmt.Errorf("invalid model format %q: empty model name", model)
	}

	return providerName, modelName, nil
}

func runAgent(cmd *cobra.Command, args []string) error {
	agentName := args[0]

	// Step 1: Load agent config
	cfg, err := agent.Load(agentName)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	// Step 2-3: Apply flag overrides
	flagModel, _ := cmd.Flags().GetString("model")
	if flagModel != "" {
		cfg.Model = flagModel
	}

	flagSkill, _ := cmd.Flags().GetString("skill")
	if flagSkill != "" {
		cfg.Skill = flagSkill
	}

	// Step 4-5: Parse model and validate provider
	provName, modelName, err := parseModel(cfg.Model)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	if provName != "anthropic" {
		return &ExitError{Code: 1, Err: fmt.Errorf("unsupported provider %q: only \"anthropic\" is supported in this version", provName)}
	}

	// Step 6: Resolve working directory
	flagWorkdir, _ := cmd.Flags().GetString("workdir")
	workdir := resolve.Workdir(flagWorkdir, cfg.Workdir)

	// Step 7: Resolve file globs
	files, err := resolve.Files(cfg.Files, workdir)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	// Step 8: Load skill
	configDir, err := xdg.GetConfigDir()
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	skillPath := cfg.Skill
	skillContent, err := resolve.Skill(skillPath, configDir)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	// Step 9: Read stdin
	// If cmd.InOrStdin() was overridden (e.g. in tests), read from it directly.
	// Otherwise, use resolve.Stdin() which checks if os.Stdin is piped.
	var stdinContent string
	if cmdIn := cmd.InOrStdin(); cmdIn != os.Stdin {
		data, readErr := io.ReadAll(cmdIn)
		if readErr != nil {
			return &ExitError{Code: 1, Err: fmt.Errorf("failed to read stdin: %w", readErr)}
		}
		stdinContent = string(data)
	} else {
		stdinContent, err = resolve.Stdin()
		if err != nil {
			return &ExitError{Code: 1, Err: err}
		}
	}

	// Step 10: Build system prompt
	systemPrompt := resolve.BuildSystemPrompt(cfg.SystemPrompt, skillContent, files)

	// Flags
	timeout, _ := cmd.Flags().GetInt("timeout")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	verbose, _ := cmd.Flags().GetBool("verbose")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Step 11: Dry-run mode
	if dryRun {
		return printDryRun(cmd, cfg, provName, modelName, workdir, timeout, systemPrompt, skillContent, files, stdinContent)
	}

	// Step 12-13: Resolve API key
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return &ExitError{Code: 3, Err: fmt.Errorf("ANTHROPIC_API_KEY environment variable is not set")}
	}

	// Step 14: Create provider
	var opts []provider.AnthropicOption
	if baseURL := os.Getenv("AXE_ANTHROPIC_BASE_URL"); baseURL != "" {
		opts = append(opts, provider.WithBaseURL(baseURL))
	}
	prov, err := provider.NewAnthropic(apiKey, opts...)
	if err != nil {
		return &ExitError{Code: 3, Err: err}
	}

	// Step 15: Build user message
	userMessage := defaultUserMessage
	if strings.TrimSpace(stdinContent) != "" {
		userMessage = stdinContent
	}

	// Step 16: Build request
	req := &provider.Request{
		Model:       modelName,
		System:      systemPrompt,
		Messages:    []provider.Message{{Role: "user", Content: userMessage}},
		Temperature: cfg.Params.Temperature,
		MaxTokens:   cfg.Params.MaxTokens,
	}

	// Verbose: pre-call info
	if verbose {
		skillDisplay := skillPath
		if skillDisplay == "" {
			skillDisplay = "(none)"
		}
		stdinDisplay := "no"
		if strings.TrimSpace(stdinContent) != "" {
			stdinDisplay = "yes"
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Model:    %s/%s\n", provName, modelName)
		fmt.Fprintf(cmd.ErrOrStderr(), "Workdir:  %s\n", workdir)
		fmt.Fprintf(cmd.ErrOrStderr(), "Skill:    %s\n", skillDisplay)
		fmt.Fprintf(cmd.ErrOrStderr(), "Files:    %d file(s)\n", len(files))
		fmt.Fprintf(cmd.ErrOrStderr(), "Stdin:    %s\n", stdinDisplay)
		fmt.Fprintf(cmd.ErrOrStderr(), "Timeout:  %ds\n", timeout)
		fmt.Fprintf(cmd.ErrOrStderr(), "Params:   temperature=%g, max_tokens=%d\n", cfg.Params.Temperature, cfg.Params.MaxTokens)
	}

	// Step 17: Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Step 18: Call provider
	start := time.Now()
	resp, err := prov.Send(ctx, req)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		// Verbose: post-call on error
		if verbose {
			fmt.Fprintf(cmd.ErrOrStderr(), "Duration: %dms\n", durationMs)
		}
		return mapProviderError(err)
	}

	// Verbose: post-call info
	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Duration: %dms\n", durationMs)
		fmt.Fprintf(cmd.ErrOrStderr(), "Tokens:   %d input, %d output\n", resp.InputTokens, resp.OutputTokens)
		fmt.Fprintf(cmd.ErrOrStderr(), "Stop:     %s\n", resp.StopReason)
	}

	// Step 19: JSON output
	if jsonOutput {
		envelope := map[string]interface{}{
			"model":         resp.Model,
			"content":       resp.Content,
			"input_tokens":  resp.InputTokens,
			"output_tokens": resp.OutputTokens,
			"stop_reason":   resp.StopReason,
			"duration_ms":   durationMs,
		}
		data, err := json.Marshal(envelope)
		if err != nil {
			return &ExitError{Code: 1, Err: fmt.Errorf("failed to marshal JSON output: %w", err)}
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Step 20: Default output
	fmt.Fprint(cmd.OutOrStdout(), resp.Content)
	return nil
}

func printDryRun(cmd *cobra.Command, cfg *agent.AgentConfig, provName, modelName, workdir string, timeout int, systemPrompt, skillContent string, files []resolve.FileContent, stdinContent string) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "=== Dry Run ===")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Model:    %s/%s\n", provName, modelName)
	fmt.Fprintf(out, "Workdir:  %s\n", workdir)
	fmt.Fprintf(out, "Timeout:  %ds\n", timeout)
	fmt.Fprintf(out, "Params:   temperature=%g, max_tokens=%d\n", cfg.Params.Temperature, cfg.Params.MaxTokens)

	fmt.Fprintln(out)
	fmt.Fprintln(out, "--- System Prompt ---")
	fmt.Fprintln(out, systemPrompt)

	fmt.Fprintln(out)
	fmt.Fprintln(out, "--- Skill ---")
	if skillContent != "" {
		fmt.Fprintln(out, skillContent)
	} else {
		fmt.Fprintln(out, "(none)")
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "--- Files (%d) ---\n", len(files))
	if len(files) > 0 {
		for _, f := range files {
			fmt.Fprintln(out, f.Path)
		}
	} else {
		fmt.Fprintln(out, "(none)")
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "--- Stdin ---")
	if strings.TrimSpace(stdinContent) != "" {
		fmt.Fprintln(out, stdinContent)
	} else {
		fmt.Fprintln(out, "(none)")
	}

	return nil
}

// mapProviderError converts a provider error to an ExitError with the correct exit code.
func mapProviderError(err error) error {
	var provErr *provider.ProviderError
	if errors.As(err, &provErr) {
		switch provErr.Category {
		case provider.ErrCategoryAuth, provider.ErrCategoryRateLimit,
			provider.ErrCategoryTimeout, provider.ErrCategoryOverloaded,
			provider.ErrCategoryServer:
			return &ExitError{Code: 3, Err: provErr}
		case provider.ErrCategoryBadRequest:
			return &ExitError{Code: 1, Err: provErr}
		}
	}
	return &ExitError{Code: 1, Err: err}
}

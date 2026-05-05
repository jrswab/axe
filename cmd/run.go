package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/pkg/runner"
	"github.com/spf13/cobra"
)

// defaultUserMessage is sent when no stdin content is piped.
const defaultUserMessage = "Execute the task described in your instructions."

var runCmd = &cobra.Command{
	Use:   "run <agent>",
	Short: "Run an agent",
	Long: `Run an agent by loading its TOML configuration, resolving all runtime
context (working directory, file globs, skill, stdin), building a prompt,
calling the LLM provider, and printing the response.

The user message is resolved in this order:
  1. -p / --prompt flag (if non-empty and non-whitespace)
  2. Piped stdin
  3. Built-in default ("Execute the task described in your instructions.")`,
	Args: exactArgs(1),
	RunE: runAgent,
}

func init() {
	runCmd.Flags().String("skill", "", "Override the agent's default skill path")
	runCmd.Flags().String("workdir", "", "Override the working directory")
	runCmd.Flags().String("agents-dir", "", "Additional agents directory to search before global config")
	runCmd.Flags().String("model", "", "Override the model (provider/model-name format)")
	runCmd.Flags().Int("timeout", 120, "Request timeout in seconds")
	runCmd.Flags().Bool("dry-run", false, "Show resolved context without calling the LLM")
	runCmd.Flags().BoolP("verbose", "v", false, "Print debug info to stderr")
	runCmd.Flags().Bool("json", false, "Wrap output in JSON with metadata")
	runCmd.Flags().StringP("prompt", "p", "", "Inline prompt to use as the user message (takes precedence over stdin; empty or whitespace is treated as absent)")
	runCmd.Flags().Int("max-tokens", 0, "Maximum total tokens (input+output) for the entire run (0 = unlimited)")
	runCmd.Flags().String("artifact-dir", "", "Override or set the artifact directory (activates artifact system)")
	runCmd.Flags().Bool("keep-artifacts", false, "Preserve auto-generated artifact directories after the run")
	runCmd.Flags().Bool("stream", false, "Enable streaming responses from the LLM provider")
	rootCmd.AddCommand(runCmd)
}

func runAgent(cmd *cobra.Command, args []string) error {
	agentName := args[0]

	flagAgentsDir, _ := cmd.Flags().GetString("agents-dir")
	var agentsDirs []string
	if flagAgentsDir != "" {
		agentsDirs = append(agentsDirs, flagAgentsDir)
	}

	flagModel, _ := cmd.Flags().GetString("model")
	flagSkill, _ := cmd.Flags().GetString("skill")
	flagWorkdir, _ := cmd.Flags().GetString("workdir")
	flagPrompt, _ := cmd.Flags().GetString("prompt")
	timeout, _ := cmd.Flags().GetInt("timeout")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	verbose, _ := cmd.Flags().GetBool("verbose")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	maxTokens, _ := cmd.Flags().GetInt("max-tokens")
	artifactDir, _ := cmd.Flags().GetString("artifact-dir")
	keepArtifacts, _ := cmd.Flags().GetBool("keep-artifacts")
	stream, _ := cmd.Flags().GetBool("stream")

	// Handle --stream flag override: only override cfg.Stream when explicitly changed.
	streamChanged := cmd.Flags().Changed("stream")
	// Handle --timeout flag override: only override when explicitly changed.
	timeoutChanged := cmd.Flags().Changed("timeout")

	opts := runner.Options{
		AgentName:     agentName,
		AgentsDirs:    agentsDirs,
		Model:         flagModel,
		Skill:         flagSkill,
		Workdir:       flagWorkdir,
		Prompt:        flagPrompt,
		MaxTokens:     maxTokens,
		ArtifactDir:   artifactDir,
		KeepArtifacts: keepArtifacts,
		DryRun:        dryRun,
		Verbose:       verbose,
		JSON:          jsonOutput,
		Stdout:        cmd.OutOrStdout(),
		Stderr:        cmd.ErrOrStderr(),
	}

	if timeoutChanged {
		opts.Timeout = timeout
	}
	if streamChanged {
		opts.Stream = stream
	}
	// Allow tests to override stdin via Cobra's InOrStdin.
	if cmdIn := cmd.InOrStdin(); cmdIn != os.Stdin {
		opts.Stdin = cmdIn
	}

	result, err := runner.Run(cmd.Context(), opts)
	if err != nil {
		return mapRunnerError(err)
	}

	if result.DryRun && result.DryRunInfo != nil {
		return printDryRun(cmd.OutOrStdout(), result.DryRunInfo)
	}

	return nil
}

func printDryRun(out io.Writer, info *runner.DryRunInfo) error {
	_, _ = fmt.Fprintln(out, "=== Dry Run ===")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "Model:    %s\n", info.Model)
	_, _ = fmt.Fprintf(out, "Workdir:  %s\n", info.Workdir)
	_, _ = fmt.Fprintf(out, "Timeout:  %ds\n", info.Timeout)
	_, _ = fmt.Fprintf(out, "Params:   %s\n", info.Params)
	_, _ = fmt.Fprintf(out, "Budget:   %d tokens (0 = unlimited)\n", info.Budget)
	streamVal := "no"
	if info.Stream {
		streamVal = "yes"
	}
	_, _ = fmt.Fprintf(out, "Stream:   %s\n", streamVal)

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- System Prompt ---")
	_, _ = fmt.Fprintln(out, info.SystemPrompt)

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- Skill ---")
	if info.Skill != "" {
		_, _ = fmt.Fprintln(out, info.Skill)
	} else {
		_, _ = fmt.Fprintln(out, "(none)")
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "--- Files (%d) ---\n", len(info.Files))
	if len(info.Files) > 0 {
		for _, f := range info.Files {
			_, _ = fmt.Fprintln(out, f)
		}
	} else {
		_, _ = fmt.Fprintln(out, "(none)")
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- User Message ---")
	if info.UserMessage != defaultUserMessage {
		_, _ = fmt.Fprintln(out, info.UserMessage)
	} else {
		_, _ = fmt.Fprintln(out, "(default)")
	}

	if info.MemoryEnabled {
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "--- Memory ---")
		if info.Memory != "" {
			_, _ = fmt.Fprintln(out, info.Memory)
		} else {
			_, _ = fmt.Fprintln(out, "(none)")
		}
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- Tools ---")
	if len(info.Tools) > 0 {
		_, _ = fmt.Fprintln(out, strings.Join(info.Tools, ", "))
	} else {
		_, _ = fmt.Fprintln(out, "(none)")
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- MCP Servers ---")
	if len(info.MCPServers) > 0 {
		for _, s := range info.MCPServers {
			_, _ = fmt.Fprintln(out, s)
		}
	} else {
		_, _ = fmt.Fprintln(out, "(none)")
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- Sub-Agents ---")
	if len(info.SubAgents) > 0 {
		_, _ = fmt.Fprintln(out, strings.Join(info.SubAgents, ", "))
		_, _ = fmt.Fprintf(out, "Max Depth: %d\n", info.MaxDepth)
		parallelVal := "yes"
		if !info.Parallel {
			parallelVal = "no"
		}
		_, _ = fmt.Fprintf(out, "Parallel:  %s\n", parallelVal)
		_, _ = fmt.Fprintf(out, "Timeout:   %ds\n", info.SubAgentTimeout)
	} else {
		_, _ = fmt.Fprintln(out, "(none)")
	}

	return nil
}

// mapRunnerError converts a runner error to an ExitError with the correct exit code.
func mapRunnerError(err error) error {
	if runner.IsConfigError(err) {
		return &ExitError{Code: 2, Err: err}
	}
	if runner.IsBudgetExceededError(err) {
		return &ExitError{Code: 4, Err: err}
	}
	if cat, ok := runner.ProviderCategory(err); ok {
		switch cat {
		case provider.ErrCategoryAuth, provider.ErrCategoryRateLimit,
			provider.ErrCategoryTimeout, provider.ErrCategoryOverloaded,
			provider.ErrCategoryServer:
			return &ExitError{Code: 3, Err: err}
		}
	}
	return &ExitError{Code: 1, Err: err}
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

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
	"github.com/jrswab/axe/internal/config"
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/resolve"
	"github.com/jrswab/axe/internal/tool"
	"github.com/jrswab/axe/internal/xdg"
	"github.com/spf13/cobra"
)

// defaultUserMessage is sent when no stdin content is piped.
const defaultUserMessage = "Execute the task described in your instructions."

// maxConversationTurns is the safety limit for the conversation loop.
const maxConversationTurns = 50

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

	// Step 5b: Load global config
	globalCfg, err := config.Load()
	if err != nil {
		return &ExitError{Code: 2, Err: err}
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

	// Step 12-13: Resolve API key and validate
	apiKey := globalCfg.ResolveAPIKey(provName)
	baseURL := globalCfg.ResolveBaseURL(provName)

	// Check for missing API key only for supported providers that require one.
	// Unsupported providers fall through to provider.New() which returns a clear error.
	if provider.Supported(provName) && provName != "ollama" && apiKey == "" {
		envVar := config.APIKeyEnvVar(provName)
		return &ExitError{Code: 3, Err: fmt.Errorf("API key for provider %q is not configured (set %s or add to config.toml)", provName, envVar)}
	}

	// Step 14: Create provider
	prov, err := provider.New(provName, apiKey, baseURL)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
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

	// Step 16b: Inject tools if agent has sub_agents
	// Depth starts at 0 for top-level invocation
	depth := 0
	effectiveMaxDepth := 3 // system default
	if cfg.SubAgentsConf.MaxDepth > 0 && cfg.SubAgentsConf.MaxDepth <= 5 {
		effectiveMaxDepth = cfg.SubAgentsConf.MaxDepth
	}
	if len(cfg.SubAgents) > 0 && depth < effectiveMaxDepth {
		req.Tools = []provider.Tool{tool.CallAgentTool(cfg.SubAgents)}
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

	// Step 18: Call provider (conversation loop when tools are present)
	start := time.Now()

	// Determine parallel execution setting.
	// Default is true (per spec). Only false if explicitly set via TOML.
	// Using *bool allows distinguishing "not set" (nil) from "set to false".
	parallel := true
	if cfg.SubAgentsConf.Parallel != nil {
		parallel = *cfg.SubAgentsConf.Parallel
	}

	var resp *provider.Response
	var totalInputTokens int
	var totalOutputTokens int
	var totalToolCalls int

	if len(req.Tools) == 0 {
		// Single-shot: no tools, no conversation loop (identical to M4)
		resp, err = prov.Send(ctx, req)
		if err != nil {
			durationMs := time.Since(start).Milliseconds()
			if verbose {
				fmt.Fprintf(cmd.ErrOrStderr(), "Duration: %dms\n", durationMs)
			}
			return mapProviderError(err)
		}
		totalInputTokens = resp.InputTokens
		totalOutputTokens = resp.OutputTokens

		if verbose {
			durationMs := time.Since(start).Milliseconds()
			fmt.Fprintf(cmd.ErrOrStderr(), "Duration: %dms\n", durationMs)
			fmt.Fprintf(cmd.ErrOrStderr(), "Tokens:   %d input, %d output\n", resp.InputTokens, resp.OutputTokens)
			fmt.Fprintf(cmd.ErrOrStderr(), "Stop:     %s\n", resp.StopReason)
		}
	} else {
		// Conversation loop: handle tool calls
		for turn := 0; turn < maxConversationTurns; turn++ {
			if verbose {
				pendingToolCalls := 0
				for _, m := range req.Messages {
					if m.Role == "tool" {
						pendingToolCalls += len(m.ToolResults)
					}
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "[turn %d] Sending request (%d messages, %d tool calls pending)\n", turn+1, len(req.Messages), pendingToolCalls)
			}

			resp, err = prov.Send(ctx, req)
			if err != nil {
				durationMs := time.Since(start).Milliseconds()
				if verbose {
					fmt.Fprintf(cmd.ErrOrStderr(), "Duration: %dms\n", durationMs)
				}
				return mapProviderError(err)
			}

			totalInputTokens += resp.InputTokens
			totalOutputTokens += resp.OutputTokens

			if verbose {
				fmt.Fprintf(cmd.ErrOrStderr(), "[turn %d] Received response: %s (%d tool calls)\n", turn+1, resp.StopReason, len(resp.ToolCalls))
			}

			// No tool calls: conversation is done
			if len(resp.ToolCalls) == 0 {
				break
			}

			// Append assistant message with tool calls
			assistantMsg := provider.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			}
			req.Messages = append(req.Messages, assistantMsg)

			// Execute tool calls
			results := executeToolCalls(ctx, resp.ToolCalls, cfg, globalCfg, depth, effectiveMaxDepth, parallel, verbose, cmd.ErrOrStderr())
			totalToolCalls += len(resp.ToolCalls)

			// Append tool result message
			toolMsg := provider.Message{
				Role:        "tool",
				ToolResults: results,
			}
			req.Messages = append(req.Messages, toolMsg)
		}

		// Check if we exhausted turns
		if resp != nil && len(resp.ToolCalls) > 0 {
			return &ExitError{Code: 1, Err: fmt.Errorf("agent exceeded maximum conversation turns (%d)", maxConversationTurns)}
		}

		if verbose {
			durationMs := time.Since(start).Milliseconds()
			fmt.Fprintf(cmd.ErrOrStderr(), "Duration: %dms\n", durationMs)
			fmt.Fprintf(cmd.ErrOrStderr(), "Tokens:   %d input, %d output (cumulative)\n", totalInputTokens, totalOutputTokens)
			fmt.Fprintf(cmd.ErrOrStderr(), "Stop:     %s\n", resp.StopReason)
		}
	}

	durationMs := time.Since(start).Milliseconds()

	// Step 19: JSON output
	if jsonOutput {
		envelope := map[string]interface{}{
			"model":         resp.Model,
			"content":       resp.Content,
			"input_tokens":  totalInputTokens,
			"output_tokens": totalOutputTokens,
			"stop_reason":   resp.StopReason,
			"duration_ms":   durationMs,
			"tool_calls":    totalToolCalls,
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

	fmt.Fprintln(out)
	fmt.Fprintln(out, "--- Sub-Agents ---")
	if len(cfg.SubAgents) > 0 {
		fmt.Fprintln(out, strings.Join(cfg.SubAgents, ", "))
		effectiveMaxDepth := 3
		if cfg.SubAgentsConf.MaxDepth > 0 && cfg.SubAgentsConf.MaxDepth <= 5 {
			effectiveMaxDepth = cfg.SubAgentsConf.MaxDepth
		}
		parallelVal := "yes"
		if cfg.SubAgentsConf.Parallel != nil && !*cfg.SubAgentsConf.Parallel {
			parallelVal = "no"
		}
		timeoutVal := cfg.SubAgentsConf.Timeout
		fmt.Fprintf(out, "Max Depth: %d\n", effectiveMaxDepth)
		fmt.Fprintf(out, "Parallel:  %s\n", parallelVal)
		fmt.Fprintf(out, "Timeout:   %ds\n", timeoutVal)
	} else {
		fmt.Fprintln(out, "(none)")
	}

	return nil
}

// executeToolCalls dispatches tool calls and returns results.
// When parallel is true and there are multiple calls, they run concurrently.
func executeToolCalls(ctx context.Context, toolCalls []provider.ToolCall, cfg *agent.AgentConfig, globalCfg *config.GlobalConfig, depth, maxDepth int, parallel, verbose bool, stderr io.Writer) []provider.ToolResult {
	results := make([]provider.ToolResult, len(toolCalls))

	execOpts := tool.ExecuteOptions{
		AllowedAgents: cfg.SubAgents,
		ParentModel:   cfg.Model,
		Depth:         depth,
		MaxDepth:      maxDepth,
		Timeout:       cfg.SubAgentsConf.Timeout,
		GlobalConfig:  globalCfg,
		Verbose:       verbose,
		Stderr:        stderr,
	}

	if len(toolCalls) == 1 || !parallel {
		// Sequential execution (also used for single call)
		for i, tc := range toolCalls {
			if tc.Name == tool.CallAgentToolName {
				results[i] = tool.ExecuteCallAgent(ctx, tc, execOpts)
			} else {
				results[i] = provider.ToolResult{
					CallID:  tc.ID,
					Content: fmt.Sprintf("Unknown tool: %q", tc.Name),
					IsError: true,
				}
			}
		}
	} else {
		// Parallel execution
		type indexedResult struct {
			index  int
			result provider.ToolResult
		}
		ch := make(chan indexedResult, len(toolCalls))
		for i, tc := range toolCalls {
			go func(idx int, call provider.ToolCall) {
				var res provider.ToolResult
				if call.Name == tool.CallAgentToolName {
					res = tool.ExecuteCallAgent(ctx, call, execOpts)
				} else {
					res = provider.ToolResult{
						CallID:  call.ID,
						Content: fmt.Sprintf("Unknown tool: %q", call.Name),
						IsError: true,
					}
				}
				ch <- indexedResult{index: idx, result: res}
			}(i, tc)
		}
		for range toolCalls {
			ir := <-ch
			results[ir.index] = ir.result
		}
	}

	return results
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

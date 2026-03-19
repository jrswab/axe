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
	"unicode/utf8"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/budget"
	"github.com/jrswab/axe/internal/config"
	"github.com/jrswab/axe/internal/mcpclient"
	"github.com/jrswab/axe/internal/memory"
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/refusal"
	"github.com/jrswab/axe/internal/resolve"
	"github.com/jrswab/axe/internal/tool"
	"github.com/jrswab/axe/internal/xdg"
	"github.com/spf13/cobra"
)

// defaultUserMessage is sent when no stdin content is piped.
const defaultUserMessage = "Execute the task described in your instructions."

// maxConversationTurns is the safety limit for the conversation loop.
const maxConversationTurns = 50

const maxToolOutputBytes = 1024

type toolCallDetail struct {
	Name    string            `json:"name"`
	Input   map[string]string `json:"input"`
	Output  string            `json:"output"`
	IsError bool              `json:"is_error"`
}

var runCmd = &cobra.Command{
	Use:   "run <agent>",
	Short: "Run an agent",
	Long: `Run an agent by loading its TOML configuration, resolving all runtime
context (working directory, file globs, skill, stdin), building a prompt,
calling the LLM provider, and printing the response.`,
	Args: exactArgs(1),
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
	runCmd.Flags().Int("max-tokens", 0, "Maximum total tokens (input+output) for the entire run (0 = unlimited)")
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

func truncateOutput(s string) string {
	if len(s) <= maxToolOutputBytes {
		return s
	}

	// Backtrack from the byte limit to avoid splitting a multi-byte UTF-8 rune.
	i := maxToolOutputBytes
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}

	return s[:i] + "... (truncated)"
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
	workdir, err := resolve.Workdir(flagWorkdir, cfg.Workdir)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

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

	// Step 10b: Memory — load entries into system prompt
	var memoryEntries string
	var memoryPath string
	var memoryCount int
	if cfg.Memory.Enabled {
		var memErr error
		memoryPath, memErr = memory.FilePath(agentName, cfg.Memory.Path)
		if memErr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to load memory for %q: %v\n", agentName, memErr)
		} else {
			memoryEntries, memErr = memory.LoadEntries(memoryPath, cfg.Memory.LastN)
			if memErr != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to load memory for %q: %v\n", agentName, memErr)
			} else if memoryEntries != "" {
				systemPrompt += "\n\n---\n\n## Memory\n\n" + memoryEntries
			}

			memoryCount, memErr = memory.CountEntries(memoryPath)
			if memErr != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to load memory for %q: %v\n", agentName, memErr)
			} else if cfg.Memory.MaxEntries > 0 && memoryCount >= cfg.Memory.MaxEntries {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: agent %q memory has %d entries (max_entries: %d). Run 'axe gc %s' to trim.\n", agentName, memoryCount, cfg.Memory.MaxEntries, agentName)
			}
		}
	}

	// Flags
	timeout, _ := cmd.Flags().GetInt("timeout")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	verbose, _ := cmd.Flags().GetBool("verbose")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Resolve effective budget
	flagMaxTokens, _ := cmd.Flags().GetInt("max-tokens")
	effectiveMaxTokens := cfg.Budget.MaxTokens
	if flagMaxTokens > 0 {
		effectiveMaxTokens = flagMaxTokens
	}
	tracker := budget.New(effectiveMaxTokens)

	// Step 11: Dry-run mode
	if dryRun {
		return printDryRun(cmd, cfg, provName, modelName, workdir, timeout, systemPrompt, skillContent, files, stdinContent, memoryEntries, effectiveMaxTokens)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Step 12-13: Resolve API key and validate
	apiKey := globalCfg.ResolveAPIKey(provName)
	baseURL := globalCfg.ResolveBaseURL(provName)
	region := globalCfg.ResolveRegion(provName)

	// For bedrock, use region as apiKey parameter and clear baseURL
	if provName == "bedrock" {
		if region == "" {
			return &ExitError{Code: 2, Err: fmt.Errorf("region for provider %q is not configured (set AWS_REGION or add to config.toml)", provName)}
		}
		apiKey = region
		baseURL = "" // Don't pass baseURL to bedrock
	}

	// Check for missing API key only for supported providers that require one.
	// Unsupported providers fall through to provider.New() which returns a clear error.
	if provider.Supported(provName) && provName != "ollama" && provName != "bedrock" && apiKey == "" {
		envVar := config.APIKeyEnvVar(provName)
		return &ExitError{Code: 2, Err: fmt.Errorf("API key for provider %q is not configured (set %s or add to config.toml)", provName, envVar)}
	}

	// Step 14: Create provider
	prov, err := provider.New(provName, apiKey, baseURL)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	// Step 14b: Wrap provider with retry decorator
	retryProv := provider.NewRetry(prov, provider.RetryConfig{
		MaxRetries:     cfg.Retry.MaxRetries,
		Backoff:        cfg.Retry.Backoff,
		InitialDelayMs: cfg.Retry.InitialDelayMs,
		MaxDelayMs:     cfg.Retry.MaxDelayMs,
		Verbose:        verbose,
		Stderr:         cmd.ErrOrStderr(),
	})
	prov = retryProv

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

	// Step 16b: Create tool registry and resolve configured tools
	registry := tool.NewRegistry()
	tool.RegisterAll(registry)
	depth := 0
	effectiveMaxDepth := 3 // system default
	if cfg.SubAgentsConf.MaxDepth > 0 && cfg.SubAgentsConf.MaxDepth <= 5 {
		effectiveMaxDepth = cfg.SubAgentsConf.MaxDepth
	}

	// Inject configured tools first (from cfg.Tools)
	if len(cfg.Tools) > 0 {
		resolvedTools, resolveErr := registry.Resolve(cfg.Tools)
		if resolveErr != nil {
			return &ExitError{Code: 1, Err: fmt.Errorf("failed to resolve tools: %w", resolveErr)}
		}
		req.Tools = append(req.Tools, resolvedTools...)
	}

	// Then inject call_agent if agent has sub_agents
	if len(cfg.SubAgents) > 0 && depth < effectiveMaxDepth {
		req.Tools = append(req.Tools, tool.CallAgentTool(cfg.SubAgents))
	}

	var mcpRouter *mcpclient.Router
	if len(cfg.MCPServers) > 0 {
		mcpRouter = mcpclient.NewRouter()
		defer func() { _ = mcpRouter.Close() }()

		builtinNames := make(map[string]bool, len(req.Tools))
		for _, t := range req.Tools {
			builtinNames[t.Name] = true
		}

		for _, serverCfg := range cfg.MCPServers {
			if verbose {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "[mcp] Connecting to %q at %s (%s)\n", serverCfg.Name, serverCfg.URL, serverCfg.Transport)
			}

			client, connErr := mcpclient.Connect(ctx, serverCfg)
			if connErr != nil {
				code := 3
				if strings.Contains(connErr.Error(), "environment variable") || strings.Contains(connErr.Error(), "unsupported MCP transport") {
					code = 2
				}
				return &ExitError{Code: code, Err: fmt.Errorf("failed to connect MCP server %q: %w", serverCfg.Name, connErr)}
			}

			mcpTools, listErr := client.ListTools(ctx)
			if listErr != nil {
				_ = client.Close()
				return &ExitError{Code: 3, Err: fmt.Errorf("failed to list tools from MCP server %q: %w", serverCfg.Name, listErr)}
			}

			filtered, registerErr := mcpRouter.Register(client, mcpTools, builtinNames)
			if registerErr != nil {
				_ = client.Close()
				return &ExitError{Code: 2, Err: fmt.Errorf("failed to register MCP tools from %q: %w", serverCfg.Name, registerErr)}
			}

			if verbose {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "[mcp] %q discovered %d tool(s), registered %d\n", serverCfg.Name, len(mcpTools), len(filtered))
				for _, discovered := range mcpTools {
					if builtinNames[discovered.Name] {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "[mcp] Skipping %q from %q (conflicts with built-in tool)\n", discovered.Name, serverCfg.Name)
					}
				}
			}

			req.Tools = append(req.Tools, filtered...)
		}
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
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Model:    %s/%s\n", provName, modelName)
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Workdir:  %s\n", workdir)
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Skill:    %s\n", skillDisplay)
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Files:    %d file(s)\n", len(files))
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Stdin:    %s\n", stdinDisplay)
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Timeout:  %ds\n", timeout)
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Params:   temperature=%g, max_tokens=%d\n", cfg.Params.Temperature, cfg.Params.MaxTokens)
		if cfg.Memory.Enabled {
			if memoryCount > 0 {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Memory:   %d entries loaded from %s\n", memoryCount, memoryPath)
			} else {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Memory:   0 entries (no memory file)\n")
			}
		}
	}

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
	var allToolCallDetails []toolCallDetail
	var budgetExceeded bool

	if len(req.Tools) == 0 {
		// Single-shot: no tools, no conversation loop (identical to M4)
		resp, err = prov.Send(ctx, req)
		if err != nil {
			durationMs := time.Since(start).Milliseconds()
			if verbose {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Duration: %dms\n", durationMs)
			}
			return mapProviderError(err)
		}
		totalInputTokens = resp.InputTokens
		totalOutputTokens = resp.OutputTokens
		tracker.Add(resp.InputTokens, resp.OutputTokens)

		if tracker.Exceeded() {
			budgetExceeded = true
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "budget exceeded: used %d of %d tokens\n", tracker.Used(), tracker.Max())
		}

		if verbose {
			durationMs := time.Since(start).Milliseconds()
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Duration: %dms\n", durationMs)
			if tracker.Max() > 0 {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Tokens:   %d input, %d output (cumulative, budget: %d/%d)\n", resp.InputTokens, resp.OutputTokens, tracker.Used(), tracker.Max())
			} else {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Tokens:   %d input, %d output\n", resp.InputTokens, resp.OutputTokens)
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Stop:     %s\n", resp.StopReason)
		}
	} else {
		// Conversation loop: handle tool calls
		for turn := 0; turn < maxConversationTurns; turn++ {
			// Check budget before making LLM call
			if tracker.Exceeded() {
				break
			}

			if verbose {
				pendingToolCalls := 0
				for _, m := range req.Messages {
					if m.Role == "tool" {
						pendingToolCalls += len(m.ToolResults)
					}
				}
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "[turn %d] Sending request (%d messages, %d tool calls pending)\n", turn+1, len(req.Messages), pendingToolCalls)
			}

			resp, err = prov.Send(ctx, req)
			if err != nil {
				durationMs := time.Since(start).Milliseconds()
				if verbose {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Duration: %dms\n", durationMs)
				}
				return mapProviderError(err)
			}

			totalInputTokens += resp.InputTokens
			totalOutputTokens += resp.OutputTokens
			tracker.Add(resp.InputTokens, resp.OutputTokens)

			if verbose {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "[turn %d] Received response: %s (%d tool calls)\n", turn+1, resp.StopReason, len(resp.ToolCalls))
			}

			// No tool calls: conversation is done
			if len(resp.ToolCalls) == 0 {
				break
			}

			// Stop before executing tools if budget is exceeded
			if tracker.Exceeded() {
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
			results := executeToolCalls(ctx, resp.ToolCalls, cfg, globalCfg, registry, mcpRouter, depth, effectiveMaxDepth, parallel, verbose, cmd.ErrOrStderr(), workdir, tracker)
			totalToolCalls += len(resp.ToolCalls)

			if jsonOutput {
				for i, tc := range resp.ToolCalls {
					input := tc.Arguments
					if input == nil {
						input = map[string]string{}
					}

					allToolCallDetails = append(allToolCallDetails, toolCallDetail{
						Name:    tc.Name,
						Input:   input,
						Output:  truncateOutput(results[i].Content),
						IsError: results[i].IsError,
					})
				}
			}

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

		// Check if budget was exceeded
		if tracker.Exceeded() {
			budgetExceeded = true
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "budget exceeded: used %d of %d tokens\n", tracker.Used(), tracker.Max())
		}

		if verbose {
			durationMs := time.Since(start).Milliseconds()
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Duration: %dms\n", durationMs)
			if tracker.Max() > 0 {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Tokens:   %d input, %d output (cumulative, budget: %d/%d)\n", totalInputTokens, totalOutputTokens, tracker.Used(), tracker.Max())
			} else {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Tokens:   %d input, %d output (cumulative)\n", totalInputTokens, totalOutputTokens)
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Stop:     %s\n", resp.StopReason)
		}
	}

	durationMs := time.Since(start).Milliseconds()

	// Step 19: JSON output
	if jsonOutput {
		if allToolCallDetails == nil {
			allToolCallDetails = make([]toolCallDetail, 0)
		}

		envelope := map[string]interface{}{
			"model":             resp.Model,
			"content":           resp.Content,
			"input_tokens":      totalInputTokens,
			"output_tokens":     totalOutputTokens,
			"stop_reason":       resp.StopReason,
			"duration_ms":       durationMs,
			"tool_calls":        totalToolCalls,
			"tool_call_details": allToolCallDetails,
			"refused":           refusal.Detect(resp.Content),
			"retry_attempts":    retryProv.Attempts(),
		}
		if tracker.Max() > 0 {
			envelope["budget_max_tokens"] = tracker.Max()
			envelope["budget_used_tokens"] = tracker.Used()
			envelope["budget_exceeded"] = tracker.Exceeded()
		}
		data, err := json.Marshal(envelope)
		if err != nil {
			return &ExitError{Code: 1, Err: fmt.Errorf("failed to marshal JSON output: %w", err)}
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		// Step 20: Default output
		_, _ = fmt.Fprint(cmd.OutOrStdout(), resp.Content)
	}

	// Return exit code 4 if budget was exceeded (before memory append)
	if budgetExceeded {
		return &ExitError{Code: 4, Err: fmt.Errorf("budget exceeded: used %d of %d tokens", tracker.Used(), tracker.Max())}
	}

	// Step 21: Append memory entry after successful response
	if cfg.Memory.Enabled {
		appendPath, appendErr := memory.FilePath(agentName, cfg.Memory.Path)
		if appendErr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save memory for %q: %v\n", agentName, appendErr)
		} else {
			if appendErr = memory.AppendEntry(appendPath, userMessage, resp.Content); appendErr != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save memory for %q: %v\n", agentName, appendErr)
			}
		}
	}

	return nil
}

func printDryRun(cmd *cobra.Command, cfg *agent.AgentConfig, provName, modelName, workdir string, timeout int, systemPrompt, skillContent string, files []resolve.FileContent, stdinContent string, memoryEntries string, maxTokens int) error {
	out := cmd.OutOrStdout()

	_, _ = fmt.Fprintln(out, "=== Dry Run ===")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "Model:    %s/%s\n", provName, modelName)
	_, _ = fmt.Fprintf(out, "Workdir:  %s\n", workdir)
	_, _ = fmt.Fprintf(out, "Timeout:  %ds\n", timeout)
	_, _ = fmt.Fprintf(out, "Params:   temperature=%g, max_tokens=%d\n", cfg.Params.Temperature, cfg.Params.MaxTokens)
	_, _ = fmt.Fprintf(out, "Budget:   %d tokens (0 = unlimited)\n", maxTokens)

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- System Prompt ---")
	_, _ = fmt.Fprintln(out, systemPrompt)

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- Skill ---")
	if skillContent != "" {
		_, _ = fmt.Fprintln(out, skillContent)
	} else {
		_, _ = fmt.Fprintln(out, "(none)")
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "--- Files (%d) ---\n", len(files))
	if len(files) > 0 {
		for _, f := range files {
			_, _ = fmt.Fprintln(out, f.Path)
		}
	} else {
		_, _ = fmt.Fprintln(out, "(none)")
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- Stdin ---")
	if strings.TrimSpace(stdinContent) != "" {
		_, _ = fmt.Fprintln(out, stdinContent)
	} else {
		_, _ = fmt.Fprintln(out, "(none)")
	}

	if cfg.Memory.Enabled {
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "--- Memory ---")
		if memoryEntries != "" {
			_, _ = fmt.Fprintln(out, memoryEntries)
		} else {
			_, _ = fmt.Fprintln(out, "(none)")
		}
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- Tools ---")
	if len(cfg.Tools) > 0 {
		_, _ = fmt.Fprintln(out, strings.Join(cfg.Tools, ", "))
	} else {
		_, _ = fmt.Fprintln(out, "(none)")
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- MCP Servers ---")
	if len(cfg.MCPServers) > 0 {
		for _, srv := range cfg.MCPServers {
			_, _ = fmt.Fprintf(out, "%s: %s (%s)\n", srv.Name, srv.URL, srv.Transport)
		}
	} else {
		_, _ = fmt.Fprintln(out, "(none)")
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "--- Sub-Agents ---")
	if len(cfg.SubAgents) > 0 {
		_, _ = fmt.Fprintln(out, strings.Join(cfg.SubAgents, ", "))
		effectiveMaxDepth := 3
		if cfg.SubAgentsConf.MaxDepth > 0 && cfg.SubAgentsConf.MaxDepth <= 5 {
			effectiveMaxDepth = cfg.SubAgentsConf.MaxDepth
		}
		parallelVal := "yes"
		if cfg.SubAgentsConf.Parallel != nil && !*cfg.SubAgentsConf.Parallel {
			parallelVal = "no"
		}
		timeoutVal := cfg.SubAgentsConf.Timeout
		_, _ = fmt.Fprintf(out, "Max Depth: %d\n", effectiveMaxDepth)
		_, _ = fmt.Fprintf(out, "Parallel:  %s\n", parallelVal)
		_, _ = fmt.Fprintf(out, "Timeout:   %ds\n", timeoutVal)
	} else {
		_, _ = fmt.Fprintln(out, "(none)")
	}

	return nil
}

// executeToolCalls dispatches tool calls and returns results.
// When parallel is true and there are multiple calls, they run concurrently.
func executeToolCalls(ctx context.Context, toolCalls []provider.ToolCall, cfg *agent.AgentConfig, globalCfg *config.GlobalConfig, registry *tool.Registry, mcpRouter *mcpclient.Router, depth, maxDepth int, parallel, verbose bool, stderr io.Writer, workdir string, budgetTracker *budget.BudgetTracker) []provider.ToolResult {
	results := make([]provider.ToolResult, len(toolCalls))

	execOpts := tool.ExecuteOptions{
		AllowedAgents: cfg.SubAgents,
		ParentModel:   cfg.Model,
		Depth:         depth,
		MaxDepth:      maxDepth,
		Timeout:       cfg.SubAgentsConf.Timeout,
		GlobalConfig:  globalCfg,
		MCPRouter:     mcpRouter,
		Verbose:       verbose,
		Stderr:        stderr,
		BudgetTracker: budgetTracker,
	}

	if len(toolCalls) == 1 || !parallel {
		// Sequential execution (also used for single call)
		for i, tc := range toolCalls {
			if mcpRouter != nil && mcpRouter.Has(tc.Name) {
				results[i] = dispatchToolCall(ctx, tc, registry, mcpRouter, verbose, stderr, workdir)
			} else if tc.Name == tool.CallAgentToolName {
				results[i] = tool.ExecuteCallAgent(ctx, tc, execOpts)
			} else {
				results[i] = dispatchToolCall(ctx, tc, registry, mcpRouter, verbose, stderr, workdir)
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
				if mcpRouter != nil && mcpRouter.Has(call.Name) {
					res = dispatchToolCall(ctx, call, registry, mcpRouter, verbose, stderr, workdir)
				} else if call.Name == tool.CallAgentToolName {
					res = tool.ExecuteCallAgent(ctx, call, execOpts)
				} else {
					res = dispatchToolCall(ctx, call, registry, mcpRouter, verbose, stderr, workdir)
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

func dispatchToolCall(ctx context.Context, tc provider.ToolCall, registry *tool.Registry, mcpRouter *mcpclient.Router, verbose bool, stderr io.Writer, workdir string) provider.ToolResult {
	if mcpRouter != nil && mcpRouter.Has(tc.Name) {
		if verbose && stderr != nil {
			if serverName, ok := mcpRouter.ServerName(tc.Name); ok {
				_, _ = fmt.Fprintf(stderr, "[mcp] Routing tool %q to server %q\n", tc.Name, serverName)
			}
		}
		result, err := mcpRouter.Dispatch(ctx, tc)
		if err != nil {
			return provider.ToolResult{CallID: tc.ID, Content: err.Error(), IsError: true}
		}
		return result
	}

	result, dispatchErr := registry.Dispatch(ctx, tc, tool.ExecContext{Workdir: workdir, Stderr: stderr, Verbose: verbose})
	if dispatchErr != nil {
		return provider.ToolResult{CallID: tc.ID, Content: dispatchErr.Error(), IsError: true}
	}
	return result
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

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/config"
	"github.com/jrswab/axe/internal/memory"
	"github.com/jrswab/axe/internal/provider"
	"github.com/spf13/cobra"
)

// gcPatternPrompt is the hard-coded system prompt for pattern detection (Req 4.1, 4.2).
const gcPatternPrompt = `You are a memory analyst for an AI agent. You will receive a log of the agent's past tasks and results. Analyze the entries and provide a structured report.

Your report MUST contain exactly these three sections with these exact headings:

## Patterns Found
Identify recurring themes, common task types, or behavioral patterns across the entries. If no patterns exist, state "No clear patterns detected."

## Repeated Work
Identify any tasks that appear to be duplicated or that the agent has done multiple times with the same or similar inputs. If no repetition is found, state "No repeated work detected."

## Recommendations
Based on the patterns and repetitions found, suggest concrete actions the user could take to improve the agent's configuration, skill, or workflow. If no recommendations apply, state "No specific recommendations."

Be concise. Reference specific entries by their timestamps when relevant.`

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Analyze and trim agent memory",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGC,
}

func init() {
	gcCmd.Flags().Bool("dry-run", false, "Analyze and print suggestions without trimming the memory file")
	gcCmd.Flags().Bool("all", false, "Run GC on all agents that have memory.enabled = true")
	gcCmd.Flags().String("model", "", "Override the model used for pattern detection (provider/model-name format)")
	rootCmd.AddCommand(gcCmd)
}

func runGC(cmd *cobra.Command, args []string) error {
	allFlag, _ := cmd.Flags().GetBool("all")

	// Argument validation
	if allFlag && len(args) > 0 {
		return &ExitError{Code: 1, Err: fmt.Errorf("cannot specify both --all and an agent name")}
	}
	if !allFlag && len(args) == 0 {
		return &ExitError{Code: 1, Err: fmt.Errorf("agent name is required (or use --all)")}
	}

	if allFlag {
		return runAllAgentsGC(cmd)
	}

	// Single-agent GC flow
	agentName := args[0]
	return runSingleAgentGC(cmd, agentName)
}

func runSingleAgentGC(cmd *cobra.Command, agentName string) error {
	// Step 1: Load agent config (Req 3.1)
	cfg, err := agent.Load(agentName)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	// Step 2: Check if memory is enabled (Req 3.2)
	if !cfg.Memory.Enabled {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: agent %q does not have memory enabled. Skipping.\n", agentName)
		return nil
	}

	// Step 3: Resolve memory file path (Req 3.4)
	memPath, err := memory.FilePath(agentName, cfg.Memory.Path)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	// Step 4: Load all entries (Req 3.5)
	entries, err := memory.LoadEntries(memPath, 0)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	if entries == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "No memory entries for agent %q. Nothing to do.\n", agentName)
		return nil
	}

	// Step 5: Count and display entries (Req 3.6)
	count, err := memory.CountEntries(memPath)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Agent: %s\nEntries: %d\n", agentName, count)

	// Step 6: Determine model (Req 3.3)
	modelFlag, _ := cmd.Flags().GetString("model")
	modelStr := cfg.Model
	if modelFlag != "" {
		modelStr = modelFlag
	}

	provName, modelName, err := parseModel(modelStr)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	// Step 7: Load global config and resolve API key (Req 3.8)
	globalCfg, err := config.Load()
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	apiKey := globalCfg.ResolveAPIKey(provName)
	baseURL := globalCfg.ResolveBaseURL(provName)

	if provider.Supported(provName) && provName != "ollama" && apiKey == "" {
		envVar := config.APIKeyEnvVar(provName)
		return &ExitError{Code: 3, Err: fmt.Errorf("API key for provider %q is not configured (set %s or add to config.toml)", provName, envVar)}
	}

	// Step 8: Create provider (Req 3.7)
	prov, err := provider.New(provName, apiKey, baseURL)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	// Step 9: Build and send LLM request (Req 3.7)
	req := &provider.Request{
		Model:       modelName,
		System:      gcPatternPrompt,
		Messages:    []provider.Message{{Role: "user", Content: entries}},
		Temperature: 0.3,
		MaxTokens:   4096,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := prov.Send(ctx, req)
	if err != nil {
		return mapProviderError(err)
	}

	// Step 10: Print analysis (Req 3.9)
	fmt.Fprintf(cmd.OutOrStdout(), "--- Analysis ---\n%s\n", resp.Content)

	// Step 11: Dry-run check (Req 3.10)
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Dry run: no entries trimmed.\n")
		return nil
	}

	// Step 12: Determine trim target (Req 3.11)
	var trimTarget int
	if cfg.Memory.LastN > 0 {
		trimTarget = cfg.Memory.LastN
	} else if cfg.Memory.MaxEntries > 0 {
		trimTarget = cfg.Memory.MaxEntries
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "No trim target configured (last_n and max_entries are both 0). Skipping trim.\n")
		return nil
	}

	// Step 13: Trim entries (Req 3.12, 3.13)
	removed, err := memory.TrimEntries(memPath, trimTarget)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return &ExitError{Code: 1, Err: err}
	}

	if removed == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No trimming needed: %d entries within limit (%d).\n", count, trimTarget)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Trimmed: %d entries removed, %d entries kept.\n", removed, trimTarget)
	}

	return nil
}

func runAllAgentsGC(cmd *cobra.Command) error {
	// Step 1: List all agents (Req 5.1)
	agents, err := agent.List()
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	// Step 2: Filter to memory-enabled agents (Req 5.2)
	var memoryAgents []agent.AgentConfig
	for _, cfg := range agents {
		if cfg.Memory.Enabled {
			memoryAgents = append(memoryAgents, cfg)
		}
	}

	if len(memoryAgents) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No agents with memory enabled.\n")
		return nil
	}

	// Step 3: Process each agent sequentially (Req 5.3, 5.4)
	failCount := 0
	for _, cfg := range memoryAgents {
		fmt.Fprintf(cmd.OutOrStdout(), "=== GC: %s ===\n", cfg.Name)

		if err := runSingleAgentGC(cmd, cfg.Name); err != nil {
			// Per-agent failure: print error, continue (Req 5.5)
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: gc failed for agent %q: %v\n", cfg.Name, err)
			failCount++
		}
	}

	// Step 4: Report summary (Req 5.6, 5.7)
	if failCount > 0 {
		return &ExitError{
			Code: 1,
			Err:  fmt.Errorf("gc completed with errors: %d of %d agents failed", failCount, len(memoryAgents)),
		}
	}

	return nil
}

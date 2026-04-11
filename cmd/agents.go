package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/xdg"
	"github.com/spf13/cobra"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage agent configurations",
	Long:  "Subcommands for managing agent TOML configuration files. Use these commands to list, inspect, create, and edit agent configurations.",
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agent configurations",
	RunE: func(cmd *cobra.Command, args []string) error {
		flagAgentsDir, _ := cmd.Flags().GetString("agents-dir")

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		searchDirs := agent.BuildSearchDirs(flagAgentsDir, cwd)
		agents, err := agent.List(searchDirs)
		if err != nil {
			return err
		}

		sort.Slice(agents, func(i, j int) bool {
			return agents[i].Name < agents[j].Name
		})

		for _, a := range agents {
			if a.Description != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s - %s\n", a.Name, a.Description)
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), a.Name)
			}
		}

		return nil
	},
}

var agentsShowCmd = &cobra.Command{
	Use:   "show <agent>",
	Short: "Show agent configuration details",
	Args:  exactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		flagAgentsDir, _ := cmd.Flags().GetString("agents-dir")

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		searchDirs := agent.BuildSearchDirs(flagAgentsDir, cwd)
		cfg, err := agent.Load(args[0], searchDirs)
		if err != nil {
			return err
		}

		w := cmd.OutOrStdout()

		// Always print required fields
		_, _ = fmt.Fprintf(w, "%-16s%s\n", "Name:", cfg.Name)
		if cfg.Description != "" {
			_, _ = fmt.Fprintf(w, "%-16s%s\n", "Description:", cfg.Description)
		}
		_, _ = fmt.Fprintf(w, "%-16s%s\n", "Model:", cfg.Model)
		if cfg.Format != nil {
			if fstr, ok := cfg.Format.(string); ok {
				_, _ = fmt.Fprintf(w, "%-16s%s\n", "Format:", fstr)
			} else {
				_, _ = fmt.Fprintf(w, "%-16s%s\n", "Format:", "JSON Schema")
			}
		}
		if cfg.SystemPrompt != "" {
			_, _ = fmt.Fprintf(w, "%-16s%s\n", "System Prompt:", cfg.SystemPrompt)
		}
		if cfg.Skill != "" {
			_, _ = fmt.Fprintf(w, "%-16s%s\n", "Skill:", cfg.Skill)
		}
		if len(cfg.Files) > 0 {
			_, _ = fmt.Fprintf(w, "%-16s%s\n", "Files:", strings.Join(cfg.Files, ", "))
		}
		if cfg.Workdir != "" {
			_, _ = fmt.Fprintf(w, "%-16s%s\n", "Workdir:", cfg.Workdir)
		}
		if cfg.Timeout > 0 {
			_, _ = fmt.Fprintf(w, "%-16s%d\n", "Timeout:", cfg.Timeout)
		}
		if len(cfg.Tools) > 0 {
			_, _ = fmt.Fprintf(w, "%-16s%s\n", "Tools:", strings.Join(cfg.Tools, ", "))
		}
		if len(cfg.SubAgents) > 0 {
			_, _ = fmt.Fprintf(w, "%-16s%s\n", "Sub-Agents:", strings.Join(cfg.SubAgents, ", "))
			if cfg.SubAgentsConf.MaxDepth != 0 {
				_, _ = fmt.Fprintf(w, "%-16s%d\n", "Max Depth:", cfg.SubAgentsConf.MaxDepth)
			}
			if cfg.SubAgentsConf.Parallel != nil {
				_, _ = fmt.Fprintf(w, "%-16s%v\n", "Parallel:", *cfg.SubAgentsConf.Parallel)
			}
			if cfg.SubAgentsConf.Timeout != 0 {
				_, _ = fmt.Fprintf(w, "%-16s%d\n", "Sub-Agent Timeout:", cfg.SubAgentsConf.Timeout)
			}
		}
		if cfg.Memory.Enabled {
			_, _ = fmt.Fprintf(w, "%-16s%v\n", "Memory Enabled:", cfg.Memory.Enabled)
		}
		if cfg.Memory.Path != "" {
			_, _ = fmt.Fprintf(w, "%-16s%s\n", "Memory Path:", cfg.Memory.Path)
		}
		if cfg.Memory.LastN != 0 {
			_, _ = fmt.Fprintf(w, "%-16s%d\n", "Memory LastN:", cfg.Memory.LastN)
		}
		if cfg.Memory.MaxEntries != 0 {
			_, _ = fmt.Fprintf(w, "%-16s%d\n", "Memory MaxEntries:", cfg.Memory.MaxEntries)
		}
		if cfg.Params.Temperature != 0 {
			_, _ = fmt.Fprintf(w, "%-16s%g\n", "Temperature:", cfg.Params.Temperature)
		}
		if cfg.Params.MaxTokens != 0 {
			_, _ = fmt.Fprintf(w, "%-16s%d\n", "Max Tokens:", cfg.Params.MaxTokens)
		}

		return nil
	},
}

var agentsInitCmd = &cobra.Command{
	Use:   "init <agent>",
	Short: "Create a new agent configuration file",
	Args:  exactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		flagAgentsDir, _ := cmd.Flags().GetString("agents-dir")

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		configDir, err := xdg.GetConfigDir()
		if err != nil {
			return err
		}

		var targetDir string
		if flagAgentsDir != "" {
			// Use the flag-provided directory
			targetDir = flagAgentsDir
		} else {
			// Check if local axe/agents directory exists
			localDir := filepath.Join(cwd, "axe", "agents")
			if _, err := os.Stat(localDir); err == nil {
				targetDir = localDir
			} else {
				// Fall back to global config dir
				targetDir = filepath.Join(configDir, "agents")
			}
		}

		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create agents directory: %w", err)
		}

		path := filepath.Join(targetDir, name+".toml")

		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("agent config already exists: %s", path)
		}

		content, err := agent.Scaffold(name)
		if err != nil {
			return err
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write agent config: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), path)
		return nil
	},
}

var agentsEditCmd = &cobra.Command{
	Use:   "edit <agent>",
	Short: "Edit an agent configuration file",
	Args:  exactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			return errors.New("$EDITOR environment variable is not set")
		}

		name := args[0]

		flagAgentsDir, _ := cmd.Flags().GetString("agents-dir")

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		searchDirs := agent.BuildSearchDirs(flagAgentsDir, cwd)

		configDir, err := xdg.GetConfigDir()
		if err != nil {
			return err
		}

		// Check searchDirs first, then fall back to global config dir
		allDirs := append(searchDirs, filepath.Join(configDir, "agents"))
		var path string
		for _, dir := range allDirs {
			candidate := filepath.Join(dir, name+".toml")
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}

		if path == "" {
			return fmt.Errorf("agent config not found: %s", name)
		}

		// Execute the editor as a child process with connected stdio
		editorCmd := exec.Command(editor, path)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr

		return editorCmd.Run()
	},
}

func init() {
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsShowCmd)
	agentsCmd.AddCommand(agentsInitCmd)
	agentsCmd.AddCommand(agentsEditCmd)
	rootCmd.AddCommand(agentsCmd)

	agentsCmd.PersistentFlags().String("agents-dir", "", "Additional agents directory to search before global config")
}

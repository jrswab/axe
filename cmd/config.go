package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jrswab/axe/internal/xdg"
	"github.com/spf13/cobra"
)

var skillsFS fs.FS

// SetSkillsFS sets the embedded filesystem containing skill templates.
func SetSkillsFS(fsys fs.FS) {
	skillsFS = fsys
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage axe configuration",
	Long:  "Commands for managing the axe configuration directory and files.",
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the configuration directory path",
	Long:  "Print the full absolute path to the axe configuration directory.",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir, err := xdg.GetConfigDir()
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), configDir)
		return nil
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the configuration directory",
	Long:  "Create the axe configuration directory structure and copy default skill templates.",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir, err := xdg.GetConfigDir()
		if err != nil {
			return err
		}

		// Create agents/ directory
		agentsDir := filepath.Join(configDir, "agents")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			return fmt.Errorf("failed to create agents directory: %w", err)
		}

		// Create skills/sample/ directory
		skillsSampleDir := filepath.Join(configDir, "skills", "sample")
		if err := os.MkdirAll(skillsSampleDir, 0755); err != nil {
			return fmt.Errorf("failed to create skills/sample directory: %w", err)
		}

		// Copy SKILL.md if it doesn't exist (idempotent)
		skillDest := filepath.Join(skillsSampleDir, "SKILL.md")
		if _, err := os.Stat(skillDest); os.IsNotExist(err) {
			data, err := fs.ReadFile(skillsFS, "skills/sample/SKILL.md")
			if err != nil {
				return fmt.Errorf("failed to read embedded SKILL.md: %w", err)
			}

			if err := os.WriteFile(skillDest, data, 0644); err != nil {
				return fmt.Errorf("failed to write SKILL.md: %w", err)
			}
		}

		// Scaffold config.toml if it doesn't exist
		configTOMLPath := filepath.Join(configDir, "config.toml")
		if _, err := os.Stat(configTOMLPath); os.IsNotExist(err) {
			configTOMLContent := `# Axe global configuration
# API keys and base URL overrides for LLM providers.
# Environment variables take precedence over values set here.
#
# Env var convention:
#   API key:  <PROVIDER_UPPER>_API_KEY  (e.g. ANTHROPIC_API_KEY)
#   Base URL: AXE_<PROVIDER_UPPER>_BASE_URL  (e.g. AXE_ANTHROPIC_BASE_URL)

# [providers.anthropic]
# api_key = ""
# base_url = ""

# [providers.openai]
# api_key = ""
# base_url = ""

# [providers.ollama]
# base_url = "http://localhost:11434"
`
			if err := os.WriteFile(configTOMLPath, []byte(configTOMLContent), 0600); err != nil {
				return fmt.Errorf("failed to write config.toml: %w", err)
			}
		}

		fmt.Fprintln(cmd.OutOrStdout(), configDir)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configInitCmd)
	rootCmd.AddCommand(configCmd)
}

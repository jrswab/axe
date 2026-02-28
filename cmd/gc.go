package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

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

	// TODO: implement single-agent and --all flows in later phases
	return nil
}

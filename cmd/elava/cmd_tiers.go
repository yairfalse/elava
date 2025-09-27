package main

import (
	"github.com/spf13/cobra"
)

// tiersCmd represents the tiers command
var tiersCmd = &cobra.Command{
	Use:   "tiers",
	Short: "Show tiered scanning status and configuration",
	Long: `Display the current tiered scanning status, showing which resource
types are assigned to which scanning tiers and their scan intervals.

Tiers:
- Critical: Essential production resources (5min intervals)
- Production: Active production resources (15min intervals)
- Standard: Regular resources (1hr intervals)
- Archive: Rarely accessed resources (6hr intervals)`,
	RunE: runTiers,
}

func init() {
	rootCmd.AddCommand(tiersCmd)
}

func runTiers(cmd *cobra.Command, args []string) error {
	// Create scan command with tier status enabled
	scanCommand := &ScanCommand{
		ShowTierStatus: true,
		Region:         "us-east-1", // Default region for tier status
	}
	return scanCommand.Run()
}

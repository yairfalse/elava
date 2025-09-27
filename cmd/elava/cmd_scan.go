package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	scanRegion         string
	scanOutput         string
	scanFilter         string
	scanRiskOnly       bool
	scanTiers          string
	scanShowTierStatus bool
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan cloud resources for untracked items",
	Long: `Scan your cloud infrastructure to find untracked resources,
orphaned items, and potential waste.

Supports tiered scanning for efficient resource discovery:
- Critical: Essential production resources (5min intervals)
- Production: Active production resources (15min intervals)
- Standard: Regular resources (1hr intervals)
- Archive: Rarely accessed resources (6hr intervals)`,
	Example: `  elava scan                              # Scan current region
  elava scan --region us-west-2           # Scan specific region
  elava scan --filter ec2                 # Only scan EC2 instances
  elava scan --filter s3                  # Only scan S3 buckets
  elava scan --risk-only                  # Only high-risk resources
  elava scan --tiers critical,production  # Scan specific tiers`,
	RunE: runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringVarP(&scanRegion, "region", "r", "us-east-1", "AWS region to scan")
	scanCmd.Flags().StringVarP(&scanOutput, "output", "o", "table", "Output format: table, json, csv")
	scanCmd.Flags().StringVarP(&scanFilter, "filter", "f", "", "Filter by resource type (ec2,rds,elb,s3,lambda,ebs,elastic_ip,nat_gateway,snapshot,ami)")
	scanCmd.Flags().BoolVar(&scanRiskOnly, "risk-only", false, "Show only high-risk untracked resources")
	scanCmd.Flags().StringVarP(&scanTiers, "tiers", "t", "", "Comma-separated list of tiers to scan")
	scanCmd.Flags().BoolVar(&scanShowTierStatus, "show-tier-status", false, "Show tiered scanning status")
}

func runScan(cmd *cobra.Command, args []string) error {
	// Build scan command from flags
	scanCommand := &ScanCommand{
		Region:         scanRegion,
		Output:         scanOutput,
		Filter:         scanFilter,
		RiskOnly:       scanRiskOnly,
		Tiers:          scanTiers,
		ShowTierStatus: scanShowTierStatus,
	}

	// Validate output format
	validOutputs := []string{"table", "json", "csv"}
	if !contains(validOutputs, scanCommand.Output) {
		return fmt.Errorf("invalid output format: %s (must be one of: %s)",
			scanCommand.Output, strings.Join(validOutputs, ", "))
	}

	// Execute scan
	return scanCommand.Run()
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

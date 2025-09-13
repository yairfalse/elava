package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/yairfalse/ovi/providers"
	_ "github.com/yairfalse/ovi/providers/aws" // Register AWS provider
	"github.com/yairfalse/ovi/scanner"
	"github.com/yairfalse/ovi/storage"
	"github.com/yairfalse/ovi/types"
	"github.com/yairfalse/ovi/wal"
)

// ScanCommand implements the 'ovi scan' command
type ScanCommand struct {
	Region   string `help:"AWS region to scan" default:"us-east-1"`
	Output   string `help:"Output format: table, json, csv" default:"table"`
	Filter   string `help:"Filter by resource type (ec2, rds, elb)"`
	RiskOnly bool   `help:"Show only high-risk untracked resources" default:"false"`
}

// Run executes the scan command
func (cmd *ScanCommand) Run() error {
	ctx := context.Background()

	fmt.Printf("🔍 Scanning AWS region %s for untracked resources...\n\n", cmd.Region)

	// Initialize storage and WAL
	tmpDir := os.TempDir() + "/ovi-scan"
	_ = os.MkdirAll(tmpDir, 0750)

	storage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer func() { _ = storage.Close() }()

	walInstance, err := wal.Open(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to create WAL: %w", err)
	}
	defer func() { _ = walInstance.Close() }()

	// Create AWS provider
	providerConfig := providers.ProviderConfig{
		Type:   "aws",
		Region: cmd.Region,
	}
	provider, err := providers.GetProvider(ctx, "aws", providerConfig)
	if err != nil {
		return fmt.Errorf("failed to create AWS provider: %w", err)
	}

	// Build filter
	filter := types.ResourceFilter{}
	if cmd.Filter != "" {
		filter.Type = cmd.Filter
	}

	// Scan resources
	resources, err := provider.ListResources(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to scan resources: %w", err)
	}

	// Record observation in WAL
	err = walInstance.Append(wal.EntryObserved, "", ScanOperation{
		Timestamp:     time.Now(),
		Region:        cmd.Region,
		ResourceCount: len(resources),
		Filter:        filter,
	})
	if err != nil {
		fmt.Printf("Warning: failed to log scan operation: %v\n", err)
	}

	// Find untracked resources
	untracked := scanner.ScanForUntracked(ctx, resources)

	// Apply risk filter if requested
	if cmd.RiskOnly {
		untracked = filterHighRisk(untracked)
	}

	// Display results
	switch cmd.Output {
	case "json":
		return cmd.outputJSON(untracked)
	case "csv":
		return cmd.outputCSV(untracked)
	default:
		return cmd.outputTable(resources, untracked)
	}
}

// outputTable displays results in a nice table format
func (cmd *ScanCommand) outputTable(allResources []types.Resource, untracked []scanner.UntrackedResource) error {
	totalResources := len(allResources)
	untrackedCount := len(untracked)
	trackedCount := totalResources - untrackedCount

	// Summary
	fmt.Printf("📊 Scan Summary:\n")
	fmt.Printf("   Total resources: %d\n", totalResources)
	fmt.Printf("   Tracked: %d (%.1f%%)\n", trackedCount, float64(trackedCount)/float64(totalResources)*100)
	fmt.Printf("   Untracked: %d (%.1f%%)\n", untrackedCount, float64(untrackedCount)/float64(totalResources)*100)
	fmt.Printf("\n")

	if len(untracked) == 0 {
		fmt.Println("🎉 All resources are properly tracked!")
		return nil
	}

	// Sort by risk (high first)
	sort.Slice(untracked, func(i, j int) bool {
		riskOrder := map[string]int{"high": 3, "medium": 2, "low": 1}
		return riskOrder[untracked[i].Risk] > riskOrder[untracked[j].Risk]
	})

	fmt.Printf("🚨 Untracked Resources:\n")

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "RESOURCE\tTYPE\tSTATUS\tRISK\tISSUES\tACTION")
	_, _ = fmt.Fprintln(w, "--------\t----\t------\t----\t------\t------")

	for _, item := range untracked {
		resource := item.Resource
		resourceID := truncate(resource.ID, 20)
		issues := strings.Join(item.Issues, ", ")
		issues = truncate(issues, 40)

		// Add emoji for risk level
		riskDisplay := item.Risk
		switch item.Risk {
		case "high":
			riskDisplay = "🔴 " + item.Risk
		case "medium":
			riskDisplay = "🟡 " + item.Risk
		case "low":
			riskDisplay = "🟢 " + item.Risk
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			resourceID,
			resource.Type,
			resource.Status,
			riskDisplay,
			issues,
			item.Action,
		)
	}

	_ = w.Flush()
	fmt.Printf("\n")

	// Action recommendations
	cmd.printActionSummary(untracked)

	return nil
}

// printActionSummary shows recommended next steps
func (cmd *ScanCommand) printActionSummary(untracked []scanner.UntrackedResource) {
	actions := make(map[string]int)
	for _, item := range untracked {
		actions[item.Action]++
	}

	fmt.Printf("💡 Recommended Actions:\n")
	for action, count := range actions {
		switch action {
		case "cleanup":
			fmt.Printf("   • Clean up %d stopped/dead resources\n", count)
		case "tag_owner":
			fmt.Printf("   • Add owner tags to %d resources\n", count)
		case "verify_management":
			fmt.Printf("   • Verify IaC management for %d resources\n", count)
		case "investigate":
			fmt.Printf("   • Investigate %d resources\n", count)
		}
	}

	fmt.Printf("\nNext steps:\n")
	fmt.Printf("   ovi cleanup --dry-run    # Preview cleanup actions (SAFE: read-only)\n")
	fmt.Printf("   ovi tag --interactive    # Tag resources interactively\n")
	fmt.Printf("   ovi report --team        # Generate team ownership report\n")
	fmt.Printf("\n🔒 Safety: Ovi NEVER deletes resources. We only detect and recommend.\n")
}

// outputJSON outputs results as JSON
func (cmd *ScanCommand) outputJSON(untracked []scanner.UntrackedResource) error {
	// Implementation for JSON output
	fmt.Println("JSON output not implemented yet")
	return nil
}

// outputCSV outputs results as CSV
func (cmd *ScanCommand) outputCSV(untracked []scanner.UntrackedResource) error {
	// Implementation for CSV output
	fmt.Println("CSV output not implemented yet")
	return nil
}

// Helper functions

func filterHighRisk(untracked []scanner.UntrackedResource) []scanner.UntrackedResource {
	var highRisk []scanner.UntrackedResource
	for _, item := range untracked {
		if item.Risk == "high" {
			highRisk = append(highRisk, item)
		}
	}
	return highRisk
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ScanOperation represents a scan operation for WAL logging
type ScanOperation struct {
	Timestamp     time.Time            `json:"timestamp"`
	Region        string               `json:"region"`
	ResourceCount int                  `json:"resource_count"`
	Filter        types.ResourceFilter `json:"filter"`
}

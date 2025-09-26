package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yairfalse/elava/config"
	"github.com/yairfalse/elava/orchestrator"
	"github.com/yairfalse/elava/providers"
	"github.com/yairfalse/elava/providers/aws"
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

var (
	dryRun     bool
	filterStr  string
	configPath string
	verbose    bool
)

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Run reconciliation cycle",
	Long: `Run a reconciliation cycle to scan resources, evaluate policies, and enforce decisions.

This command will:
1. Scan cloud resources
2. Evaluate policies against each resource
3. Enforce decisions (tag, notify)
4. Store results in audit trail

Examples:
  # Run full reconciliation
  elava reconcile

  # Preview what would happen (dry-run)
  elava reconcile --dry-run

  # Only reconcile EC2 resources
  elava reconcile --filter type=ec2

  # Use specific config file
  elava reconcile --config ./config.yaml`,
	RunE: runReconcile,
}

func init() {
	rootCmd.AddCommand(reconcileCmd)

	reconcileCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without enforcing")
	reconcileCmd.Flags().StringVar(&filterStr, "filter", "", "Resource filter (e.g., type=ec2,region=us-east-1)")
	reconcileCmd.Flags().StringVar(&configPath, "config", "", "Config file path")
	reconcileCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
}

func runReconcile(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration
	cfg, err := loadReconcileConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize storage (use default path for now)
	storagePath := "/tmp/elava-storage"
	// TODO: Could read from config if we had storage config
	store, err := storage.NewMVCCStorage(storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Create provider
	provider, err := createProvider(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create scanner
	scanner := &ProviderScanner{
		provider: provider,
		filter:   parseFilter(filterStr),
	}

	// Create orchestrator
	orch := orchestrator.NewOrchestrator(store)
	orch = orch.WithScanner(scanner)

	// If dry-run, use enforcer without provider
	if dryRun {
		fmt.Println("🔍 Running in DRY-RUN mode - no changes will be made")
	}

	// Run reconciliation cycle
	fmt.Println("🔄 Starting reconciliation cycle...")
	result, err := orch.RunCycle(ctx)
	if err != nil {
		return fmt.Errorf("reconciliation failed: %w", err)
	}

	// Display results
	displayReconcileResults(result)

	return nil
}

func loadReconcileConfig() (*config.Config, error) {
	if configPath != "" {
		return config.LoadFromPath(configPath)
	}
	return config.LoadDefault(), nil
}

func createProvider(ctx context.Context, cfg *config.Config) (providers.CloudProvider, error) {
	// For now, only AWS
	region := cfg.Region
	if region == "" {
		region = "us-east-1" // Default region
	}

	return aws.NewRealAWSProvider(ctx, region)
}

func parseFilter(filterStr string) types.ResourceFilter {
	// Simple filter parsing for now
	// Format: type=ec2,region=us-east-1
	filter := types.ResourceFilter{}

	// TODO: Implement proper filter parsing
	// For now, return empty filter (all resources)

	return filter
}

func displayReconcileResults(result *orchestrator.CycleResult) {
	fmt.Println("\n✅ Reconciliation Complete")
	fmt.Printf("  📊 Resources Scanned: %d\n", result.ResourcesScanned)
	fmt.Printf("  📋 Policies Evaluated: %d\n", result.PoliciesEvaluated)
	fmt.Printf("  🎯 Actions Taken: %d\n", result.EnforcementActions)
	fmt.Printf("  ⏱️  Duration: %s\n", result.Duration)

	if len(result.Errors) > 0 {
		fmt.Printf("\n⚠️  Errors encountered:\n")
		for _, err := range result.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	if result.Success {
		fmt.Println("\n✨ All operations completed successfully")
	} else {
		fmt.Println("\n❌ Some operations failed - check errors above")
		os.Exit(1)
	}
}

// ProviderScanner wraps a provider to implement Scanner interface
type ProviderScanner struct {
	provider providers.CloudProvider
	filter   types.ResourceFilter
}

func (ps *ProviderScanner) Scan(ctx context.Context) ([]types.Resource, error) {
	return ps.provider.ListResources(ctx, ps.filter)
}

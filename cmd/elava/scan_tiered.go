package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/yairfalse/elava/config"
	"github.com/yairfalse/elava/scanner"
	"github.com/yairfalse/elava/types"
)

// TieredScanInfra holds tiered scanning infrastructure
type TieredScanInfra struct {
	config        *config.Config
	tieredScanner *scanner.TieredScanner
}

// initTieredInfrastructure sets up tiered scanning components
func (cmd *ScanCommand) initTieredInfrastructure() (*TieredScanInfra, error) {
	// Load configuration
	cfg, err := config.LoadFromPath("")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create tiered scanner
	tieredScanner := scanner.NewTieredScanner(cfg)

	return &TieredScanInfra{
		config:        cfg,
		tieredScanner: tieredScanner,
	}, nil
}

// runTieredScan executes a tiered scan workflow
func (cmd *ScanCommand) runTieredScan(ctx context.Context, infra *scanInfra) ([]types.Resource, error) {
	// Initialize tiered components
	tieredInfra, err := cmd.initTieredInfrastructure()
	if err != nil {
		return nil, err
	}

	// Handle tier status display
	if cmd.ShowTierStatus {
		return nil, cmd.displayTierStatus(tieredInfra.tieredScanner)
	}

	// Determine which tiers to scan
	tierNames := cmd.determineTiersToScan(tieredInfra.tieredScanner)

	fmt.Printf("Scanning AWS region %s for untracked resources...\n", cmd.Region)
	if len(tierNames) > 0 {
		fmt.Printf("Scanning tiers: %v\n\n", tierNames)
	} else {
		fmt.Printf("Scanning all resources (no tier-based filtering)\n\n")
	}

	// Scan all resources first
	resources, err := cmd.scanResources(ctx, infra.provider)
	if err != nil {
		return nil, err
	}

	// Filter resources by tiers if specified
	if len(tierNames) > 0 {
		resources = cmd.filterResourcesByTiers(tieredInfra.tieredScanner, resources, tierNames)
		// Mark tiers as scanned
		cmd.markTiersScanned(tieredInfra.tieredScanner, resources, tierNames)
	}

	return resources, nil
}

// Tiered scanning helper methods

// displayTierStatus shows the current tiered scanning status
func (cmd *ScanCommand) displayTierStatus(tieredScanner *scanner.TieredScanner) error {
	summary := tieredScanner.GetScanSummary()
	fmt.Print(summary.String())
	return nil
}

// determineTiersToScan decides which tiers to scan based on flags and schedule
func (cmd *ScanCommand) determineTiersToScan(tieredScanner *scanner.TieredScanner) []string {
	if cmd.Tiers != "" {
		// User specified specific tiers
		tiers := strings.Split(strings.TrimSpace(cmd.Tiers), ",")
		var validTiers []string
		for _, tier := range tiers {
			tier = strings.TrimSpace(tier)
			if tier != "" {
				validTiers = append(validTiers, tier)
			}
		}
		return validTiers
	}

	// Use intelligent tier scheduling - only scan tiers that are due
	return tieredScanner.GetTiersDueForScan()
}

// filterResourcesByTiers filters resources to only those in specified tiers
func (cmd *ScanCommand) filterResourcesByTiers(tieredScanner *scanner.TieredScanner, resources []types.Resource, tierNames []string) []types.Resource {
	tierMap := tieredScanner.FilterResourcesByTier(resources, tierNames)

	var filtered []types.Resource
	for _, tierResources := range tierMap {
		filtered = append(filtered, tierResources...)
	}

	return filtered
}

// markTiersScanned records that tiers were scanned
func (cmd *ScanCommand) markTiersScanned(tieredScanner *scanner.TieredScanner, resources []types.Resource, tierNames []string) {
	tierMap := tieredScanner.FilterResourcesByTier(resources, tierNames)

	for tierName, tierResources := range tierMap {
		tieredScanner.MarkTierScanned(tierName, tierResources)
	}
}

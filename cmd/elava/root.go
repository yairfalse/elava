package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
	rootCmd = &cobra.Command{
		Use:   "elava",
		Short: "Living Infrastructure Engine",
		Long: `Elava - Living Infrastructure Engine

Elava is a living infrastructure reconciliation engine that manages
cloud resources without state files. Your cloud IS the state.

Find and manage untracked resources, get cleanup recommendations,
and maintain infrastructure hygiene with continuous reconciliation.`,
		Version: version,
	}
)

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// init sets up the root command
func init() {
	rootCmd.SetVersionTemplate(`Elava {{.Version}} - Living Infrastructure Engine
`)
}

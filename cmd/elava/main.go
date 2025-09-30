package main

import (
	"fmt"
	"os"
)

func main() {
	Execute()
}

// Execute runs the CLI application
func Execute() {
	// Simple placeholder - just run scan for now
	cmd := &ScanCommand{
		Region: "us-east-1",
		Output: "table",
	}

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"
	"flag"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "version":
		fmt.Printf("Ovi %s - Day 2 Operations Companion\n", version)
	case "scan":
		if err := runScanCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "cleanup":
		fmt.Println("Cleanup recommendations... (coming soon)")
	case "tag":
		fmt.Println("Interactive tagging... (coming soon)")
	case "report":
		fmt.Println("Generate reports... (coming soon)")
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runScanCommand() error {
	// Parse scan-specific flags
	scanFlags := flag.NewFlagSet("scan", flag.ExitOnError)
	region := scanFlags.String("region", "us-east-1", "AWS region to scan")
	output := scanFlags.String("output", "table", "Output format: table, json, csv")
	filter := scanFlags.String("filter", "", "Filter by resource type (ec2, rds, elb)")
	riskOnly := scanFlags.Bool("risk-only", false, "Show only high-risk untracked resources")
	
	// Parse remaining args
	scanFlags.Parse(os.Args[2:])
	
	// Create and run scan command
	scanCmd := &ScanCommand{
		Region:   *region,
		Output:   *output,
		Filter:   *filter,
		RiskOnly: *riskOnly,
	}
	
	return scanCmd.Run()
}

func printUsage() {
	fmt.Println("Ovi - Day 2 Operations Companion")
	fmt.Println("Find and clean up untracked resources in your cloud")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ovi <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  scan      Find untracked resources")
	fmt.Println("  cleanup   Get cleanup recommendations")
	fmt.Println("  tag       Tag resources interactively")
	fmt.Println("  report    Generate ownership reports")
	fmt.Println("  version   Show version")
	fmt.Println("  help      Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ovi scan                    # Scan current region")
	fmt.Println("  ovi scan --region us-west-2 # Scan specific region")
	fmt.Println("  ovi scan --filter ec2       # Only scan EC2 instances")
	fmt.Println("  ovi scan --risk-only        # Only high-risk resources")
}

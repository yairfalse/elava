package main

import (
	"fmt"
	"os"
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
		fmt.Printf("Ovi %s\n", version)
	case "scan":
		fmt.Println("Scanning cloud resources... (not implemented)")
	case "apply":
		fmt.Println("Applying configuration... (not implemented)")
	case "guardian":
		fmt.Println("Starting guardian mode... (not implemented)")
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Ovi - Living Infrastructure Engine")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ovi <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  scan      Scan cloud resources")
	fmt.Println("  apply     Apply configuration")
	fmt.Println("  guardian  Start continuous reconciliation")
	fmt.Println("  version   Show version")
	fmt.Println("  help      Show this help")
}

package main

import (
	"fmt"
	"os"

	"github.com/brynnjknight/proxer/internal/cmd"
)

// Version information (set during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func main() {
	// Set version info for the CLI
	cmd.SetVersionInfo(Version, GitCommit, BuildDate)

	// Execute the root command
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

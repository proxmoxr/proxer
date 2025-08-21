package cmd

import (
	"path/filepath"

	"github.com/brynnjknight/proxer/internal/models"
)

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// getProjectNameFromPath derives a project name from a stack file path
func getProjectNameFromPath(stackFile string) string {
	// Use directory name as project name
	dir := filepath.Dir(stackFile)
	if dir == "." {
		// Use current directory name
		if wd, err := filepath.Abs("."); err == nil {
			return filepath.Base(wd)
		}
	}
	return filepath.Base(dir)
}

// getNetworkNames returns a slice of network names from a stack
func getNetworkNames(stack *models.LXCStack) []string {
	names := make([]string, 0, len(stack.Networks))
	for name := range stack.Networks {
		names = append(names, name)
	}
	return names
}

// getVolumeNames returns a slice of volume names from a stack
func getVolumeNames(stack *models.LXCStack) []string {
	names := make([]string, 0, len(stack.Volumes))
	for name := range stack.Volumes {
		names = append(names, name)
	}
	return names
}

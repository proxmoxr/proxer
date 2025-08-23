package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/brynnjknight/proxer/pkg/proxmox"
)

var (
	showAll    bool
	showQuiet  bool
	format     string
	noTrunc    bool
	filterTags []string
)

// psCmd represents the ps command
var psCmd = &cobra.Command{
	Use:   "ps [OPTIONS]",
	Short: "List LXC containers",
	Long: `List LXC containers managed by pxc.

By default, only shows containers managed by pxc (tagged with 'pxc').
Use --all to show all LXC containers on the system.

OUTPUT INFORMATION:
  Default table format shows:
  • VMID: Proxmox container ID
  • NAME: Container name (from metadata or auto-generated)
  • STATUS: Current container state (running, stopped, etc.)
  • UPTIME: Time since container started
  • MEMORY: Current memory usage / memory limit
  • CPU: Current CPU usage percentage

FILTERING:
  Use --filter to narrow results:
  • tag=value: Filter by container labels/tags
  • name=pattern: Filter by name pattern
  • status=state: Filter by container status

FORMAT OPTIONS:
  --format supports Go template syntax with these fields:
  • {{.VMID}} - Container ID
  • {{.Name}} - Container name
  • {{.Status}} - Container status
  • {{.Uptime}} - Container uptime
  • {{.Memory}} - Memory usage
  • {{.CPU}} - CPU usage
  • {{.Template}} - Source template
  • {{.Node}} - Proxmox node

STATUS VALUES:
  • running: Container is active and operational
  • stopped: Container is shut down
  • paused: Container is paused/suspended
  • template: Container template (not runnable)
  • error: Container in error state

RESOURCE MONITORING:
  The ps command shows real-time resource usage:
  • Memory: Shows current/limit (e.g., "512MB/1024MB")
  • CPU: Shows current usage percentage
  • Uptime: Shows time since last start

INTEGRATION:
  Output can be processed by other tools:
  • Use --quiet for container IDs only
  • Use --format for custom output
  • Combine with shell tools: pxc ps --quiet | xargs pct stop

TROUBLESHOOTING:
  Common Issues:
  • "No containers found"
    → Use --all to see all containers
    → Check if containers have pxc tags
    → Verify Proxmox node connectivity
  
  • "Permission denied"
    → Ensure user has Proxmox container privileges
    → Check if 'pct' command is accessible
    → Verify Proxmox authentication

AUTOMATION:
  Useful for scripts and monitoring:
  • Check if services are running: pxc ps --filter status=running
  • Get container IDs for batch operations: pxc ps --quiet
  • Monitor resource usage in scripts`,
	Example: `  # List pxc-managed containers
  pxc ps

  # List all containers on system
  pxc ps --all

  # Show only container IDs (useful for scripts)
  pxc ps --quiet

  # Filter by status
  pxc ps --filter status=running
  pxc ps --filter status=stopped

  # Filter by tags/labels
  pxc ps --filter tag=webapp
  pxc ps --filter tag=production

  # Custom table format
  pxc ps --format "table {{.VMID}}\t{{.Name}}\t{{.Status}}\t{{.Memory}}"

  # JSON-like output for automation
  pxc ps --format "{{.VMID}},{{.Name}},{{.Status}}"

  # Show full information without truncation
  pxc ps --no-trunc

  # Monitor specific project containers
  pxc ps --filter tag=myproject --format "table {{.Name}}\t{{.Status}}\t{{.Uptime}}"`,
	RunE: runPS,
}

func init() {
	rootCmd.AddCommand(psCmd)

	// PS-specific flags
	psCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all containers (not just pxc-managed)")
	psCmd.Flags().BoolVarP(&showQuiet, "quiet", "q", false, "Only display container IDs")
	psCmd.Flags().StringVar(&format, "format", "", "Format output using a custom template")
	psCmd.Flags().BoolVar(&noTrunc, "no-trunc", false, "Don't truncate output")
	psCmd.Flags().StringSliceVar(&filterTags, "filter", []string{}, "Filter containers (e.g., tag=webapp)")
}

func runPS(cmd *cobra.Command, args []string) error {
	// Create Proxmox client
	client := proxmox.NewClient("", IsVerbose(), IsDryRun())

	// Get all containers
	containers, err := client.ListContainers()
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Filter containers if not showing all
	if !showAll {
		containers = client.FilterPXCContainers(containers)
	}

	// Apply additional filters
	containers = applyFilters(containers, filterTags)

	// Handle quiet mode
	if showQuiet {
		for _, container := range containers {
			fmt.Println(container.VMID)
		}
		return nil
	}

	// Handle custom format
	if format != "" {
		return printCustomFormat(containers, format)
	}

	// Default table format
	return printContainerTable(containers)
}

// applyFilters applies tag and other filters to the container list
func applyFilters(containers []proxmox.ContainerInfo, filters []string) []proxmox.ContainerInfo {
	if len(filters) == 0 {
		return containers
	}

	var filtered []proxmox.ContainerInfo

	for _, container := range containers {
		include := true

		for _, filter := range filters {
			parts := strings.SplitN(filter, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := parts[0]
			value := parts[1]

			switch key {
			case "tag":
				if !strings.Contains(container.Tags, value) {
					include = false
				}
			case "status":
				if container.Status != value {
					include = false
				}
			case "name":
				if !strings.Contains(container.Name, value) {
					include = false
				}
			}
		}

		if include {
			filtered = append(filtered, container)
		}
	}

	return filtered
}

// printContainerTable prints containers in a table format
func printContainerTable(containers []proxmox.ContainerInfo) error {
	if len(containers) == 0 {
		if !showAll {
			PrintInfo("No pxc-managed containers found. Use --all to see all containers.")
		} else {
			PrintInfo("No containers found.")
		}
		return nil
	}

	// Create tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	defer w.Flush()

	// Print header
	fmt.Fprintln(w, "CONTAINER ID\tNAME\tSTATUS\tCPUS\tMEMORY\tUPTIME\tTAGS")

	// Print containers
	for _, container := range containers {
		status := formatStatus(container.Status)
		cpus := formatCPUs(container.CPUs)
		memory := formatMemory(container.Memory)
		uptime := formatUptime(container.Uptime)
		tags := formatTags(container.Tags)

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			container.VMID,
			truncateStringPS(container.Name, 20),
			status,
			cpus,
			memory,
			uptime,
			tags,
		)
	}

	return nil
}

// printCustomFormat prints containers using a custom format template
func printCustomFormat(containers []proxmox.ContainerInfo, format string) error {
	// Simple template replacement - could be enhanced with proper templating
	for _, container := range containers {
		output := format
		output = strings.ReplaceAll(output, "{{.VMID}}", strconv.Itoa(container.VMID))
		output = strings.ReplaceAll(output, "{{.Name}}", container.Name)
		output = strings.ReplaceAll(output, "{{.Status}}", container.Status)
		output = strings.ReplaceAll(output, "{{.CPUs}}", formatCPUs(container.CPUs))
		output = strings.ReplaceAll(output, "{{.Memory}}", formatMemory(container.Memory))
		output = strings.ReplaceAll(output, "{{.Uptime}}", formatUptime(container.Uptime))
		output = strings.ReplaceAll(output, "{{.Tags}}", container.Tags)
		output = strings.ReplaceAll(output, "\\t", "\t")
		output = strings.ReplaceAll(output, "\\n", "\n")

		// Handle table prefix
		output = strings.TrimPrefix(output, "table ")

		fmt.Println(output)
	}
	return nil
}

// formatStatus returns a colored status string
func formatStatus(status string) string {
	switch status {
	case "running":
		return color.GreenString("running")
	case "stopped":
		return color.RedString("stopped")
	case "template":
		return color.BlueString("template")
	default:
		return color.YellowString(status)
	}
}

// formatCPUs formats CPU information
func formatCPUs(cpus float64) string {
	if cpus == 0 {
		return "-"
	}
	if cpus == float64(int(cpus)) {
		return fmt.Sprintf("%.0f", cpus)
	}
	return fmt.Sprintf("%.1f", cpus)
}

// formatMemory formats memory information in human-readable format
func formatMemory(memory int64) string {
	if memory == 0 {
		return "-"
	}

	// Convert from bytes to appropriate unit
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	if memory >= GB {
		return fmt.Sprintf("%.1fGB", float64(memory)/GB)
	} else if memory >= MB {
		return fmt.Sprintf("%.0fMB", float64(memory)/MB)
	} else if memory >= KB {
		return fmt.Sprintf("%.0fKB", float64(memory)/KB)
	} else {
		return fmt.Sprintf("%dB", memory)
	}
}

// formatUptime formats uptime information
func formatUptime(uptime int64) string {
	if uptime == 0 {
		return "-"
	}

	duration := time.Duration(uptime) * time.Second

	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	} else if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// formatTags formats and truncates tags
func formatTags(tags string) string {
	if tags == "" {
		return "-"
	}

	if !noTrunc && len(tags) > 20 {
		return tags[:17] + "..."
	}

	return tags
}

// truncateStringPS truncates a string to maxLen characters (ps-specific version)
func truncateStringPS(s string, maxLen int) string {
	if noTrunc || len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

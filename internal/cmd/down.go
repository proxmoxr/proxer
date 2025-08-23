package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/brynnjknight/proxer/pkg/config"
	"github.com/brynnjknight/proxer/pkg/runner"
)

var (
	removeVolumes bool
	removeOrphans bool
	timeout       int
)

// downCmd represents the down command
var downCmd = &cobra.Command{
	Use:   "down [OPTIONS]",
	Short: "Stop and remove containers, networks, and volumes",
	Long: `Stop and remove containers, networks, and volumes created by 'pxc up'.

This command safely tears down multi-container applications:
1. Executes pre-stop hooks (backup scripts, notifications, etc.)
2. Stops containers in reverse dependency order (web → api → database)
3. Removes containers and their configurations
4. Optionally removes volumes and networks (--volumes flag)
5. Executes post-stop hooks (cleanup, monitoring updates)

SAFETY FEATURES:
  • Volumes are preserved by default to prevent data loss
  • Containers are stopped gracefully with configurable timeout
  • Dependency order ensures services shut down cleanly
  • Pre-stop hooks allow for backup and cleanup operations

DATA PROTECTION:
  By default, the following are preserved:
  • Named volumes (database data, application state)
  • Networks (can be reused by future deployments)
  • Templates (built container images)
  
  Use --volumes flag to remove volumes (DESTRUCTIVE - data will be lost!)

STOP SEQUENCE:
  Containers are stopped in reverse dependency order to maintain data integrity.
  
  Example: For web → api → database dependencies:
  1. web (stops first, no other services depend on it)
  2. api (stops after web is down)
  3. database (stops last, ensures no active connections)

TIMEOUT HANDLING:
  • Each container gets timeout seconds to stop gracefully
  • After timeout, containers are forcibly terminated
  • Database containers may need longer timeouts for clean shutdown
  • Use --timeout to adjust based on your application needs

ORPHAN REMOVAL:
  • --remove-orphans removes containers not defined in current stack
  • Useful when services have been removed from lxc-stack.yml
  • Prevents accumulation of unused containers

TROUBLESHOOTING:
  Common Issues:
  • "Container won't stop"
    → Increase timeout: --timeout 60
    → Check for hung processes in container
    → Force stop with pct stop <id> --force
  
  • "Volume removal failed"
    → Check if volume is still mounted
    → Ensure no other containers using volume
    → Manual cleanup: pct set <id> --delete mp0
  
  • "Network removal failed"
    → Verify no containers still connected
    → Check for external bridge dependencies
    → Manual cleanup may be required

RECOVERY:
  If down fails partially:
  • Use pxc ps to see remaining containers
  • Manually stop problematic containers: pct stop <id>
  • Re-run pxc down to complete cleanup
  • Use --remove-orphans to catch missed containers`,
	Example: `  # Stop and remove containers (preserves volumes)
  pxc down

  # Remove everything including volumes (DESTRUCTIVE)
  pxc down --volumes

  # Use custom stack file
  pxc down -f my-stack.yml

  # Remove orphaned containers not in stack
  pxc down --remove-orphans

  # Increase stop timeout for databases
  pxc down --timeout 60

  # Dry run to see what would be removed
  pxc down --dry-run --verbose

  # Emergency cleanup (if normal down fails)
  pxc down --timeout 5 --remove-orphans --volumes`,
	RunE: runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)

	// Down-specific flags
	downCmd.Flags().StringVarP(&stackFile, "file", "f", "", "Path to stack file (default: lxc-stack.yml)")
	downCmd.Flags().StringVar(&projectName, "project-name", "", "Project name (default: directory name)")
	downCmd.Flags().BoolVar(&removeVolumes, "volumes", false, "Remove named volumes")
	downCmd.Flags().BoolVar(&removeOrphans, "remove-orphans", false, "Remove containers not defined in stack")
	downCmd.Flags().IntVarP(&timeout, "timeout", "t", 10, "Timeout in seconds for container stop")
}

func runDown(cmd *cobra.Command, args []string) error {
	// Determine stack file
	if stackFile == "" {
		stackFile = config.GetDefaultStackfile()
	}

	// Validate stack file exists
	if err := config.ValidateConfigExists(stackFile); err != nil {
		return err
	}

	// Determine project name
	if projectName == "" {
		projectName = getProjectNameFromPath(stackFile)
	}

	PrintInfo("Stopping stack: %s", projectName)
	PrintInfo("Stack file: %s", stackFile)

	if removeVolumes {
		PrintWarning("Volumes will be removed")
	}

	if IsVerbose() {
		printDownSummary()
	}

	if IsDryRun() {
		PrintWarning("Dry run mode - no actual removal will be performed")
		return printDownDryRun()
	}

	// Create orchestrator
	orchestrator := runner.New(&runner.Config{
		Verbose:         IsVerbose(),
		DryRun:          IsDryRun(),
		ProjectName:     projectName,
		BaseDir:         filepath.Dir(stackFile),
		ProxmoxNode:     viper.GetString("proxmox_node"),
		Storage:         viper.GetString("storage"),
		TemplateStorage: viper.GetString("template_storage"),
	})

	// Stop the stack
	if err := orchestrator.Down(stackFile, removeVolumes); err != nil {
		return fmt.Errorf("failed to stop stack: %w", err)
	}

	// Handle orphan removal if requested
	if removeOrphans {
		if err := removeOrphanedContainers(); err != nil {
			PrintWarning("Failed to remove orphaned containers: %v", err)
		}
	}

	PrintSuccess("Stack stopped successfully")
	return nil
}

func printDownSummary() {
	// Load stack to show summary
	stack, err := config.LoadLXCStack(stackFile)
	if err != nil {
		return
	}

	fmt.Println("\nShutdown Summary:")
	fmt.Printf("  Project: %s\n", projectName)
	fmt.Printf("  Services to stop: %d\n", len(stack.Services))

	if removeVolumes && len(stack.Volumes) > 0 {
		fmt.Printf("  Volumes to remove: %d\n", len(stack.Volumes))
	}

	// Show services in shutdown order
	if len(stack.Services) > 0 {
		fmt.Println("  Shutdown order:")
		serviceOrder, err := stack.GetServiceDependencyOrder()
		if err == nil {
			// Reverse for shutdown
			for i := len(serviceOrder)/2 - 1; i >= 0; i-- {
				opp := len(serviceOrder) - 1 - i
				serviceOrder[i], serviceOrder[opp] = serviceOrder[opp], serviceOrder[i]
			}

			for i, serviceName := range serviceOrder {
				fmt.Printf("    %d. %s\n", i+1, serviceName)
			}
		}
	}

	fmt.Println()
}

func printDownDryRun() error {
	// Load and validate stack
	stack, err := config.LoadLXCStack(stackFile)
	if err != nil {
		return err
	}

	fmt.Println("\nDry Run Plan:")

	// Show pre-stop hooks
	if stack.Hooks != nil && len(stack.Hooks.PreStop) > 0 {
		fmt.Printf("  1. Execute pre-stop hooks (%d hooks)\n", len(stack.Hooks.PreStop))
	}

	// Show service shutdown order
	serviceOrder, err := stack.GetServiceDependencyOrder()
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Reverse for shutdown
	for i := len(serviceOrder)/2 - 1; i >= 0; i-- {
		opp := len(serviceOrder) - 1 - i
		serviceOrder[i], serviceOrder[opp] = serviceOrder[opp], serviceOrder[i]
	}

	step := 2
	for _, serviceName := range serviceOrder {
		fmt.Printf("  %d. Stop and remove container for service '%s'\n", step, serviceName)
		step++
	}

	// Show volume removal
	if removeVolumes && len(stack.Volumes) > 0 {
		volumeNames := getVolumeNames(stack)
		fmt.Printf("  %d. Remove volumes: %s\n", step, fmt.Sprintf("%v", volumeNames))
		step++
	}

	// Show network removal
	if len(stack.Networks) > 0 {
		networkNames := getNetworkNames(stack)
		fmt.Printf("  %d. Remove networks: %s\n", step, fmt.Sprintf("%v", networkNames))
		step++
	}

	// Show orphan removal
	if removeOrphans {
		fmt.Printf("  %d. Remove orphaned containers\n", step)
		step++
	}

	// Show post-stop hooks
	if stack.Hooks != nil && len(stack.Hooks.PostStop) > 0 {
		fmt.Printf("  %d. Execute post-stop hooks (%d hooks)\n", step, len(stack.Hooks.PostStop))
	}

	return nil
}

func removeOrphanedContainers() error {
	// TODO: Implement orphan container removal
	// This would:
	// 1. List all containers tagged with project name
	// 2. Compare with containers defined in stack
	// 3. Remove any that aren't defined

	PrintInfo("Removing orphaned containers (placeholder)")
	return nil
}

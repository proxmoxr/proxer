package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/brynnjknight/proxer/pkg/config"
	"github.com/brynnjknight/proxer/pkg/runner"
)

var (
	stackFile     string
	projectName   string
	detach        bool
	buildArgs     map[string]string
	buildServices []string
)

// upCmd represents the up command
var upCmd = &cobra.Command{
	Use:   "up [OPTIONS] [SERVICE...]",
	Short: "Create and start containers from lxc-stack.yml",
	Long: `Create and start containers for a multi-container application defined in lxc-stack.yml.

This command orchestrates the complete deployment process:
1. Validates stack configuration and resolves service dependencies
2. Builds any missing container templates from LXCfile definitions
3. Creates custom networks and named volumes as defined
4. Creates containers with proper resource allocation and configuration
5. Starts containers in dependency order (respecting depends_on)
6. Waits for health checks to pass on all services
7. Executes post-start hooks for additional setup

By default, looks for lxc-stack.yml in the current directory.

DEPENDENCY RESOLUTION:
  Services are started in topological order based on depends_on declarations.
  Circular dependencies are detected and reported as errors.
  
  Example startup order for: web → api → database
  1. database (no dependencies)
  2. api (depends on database) 
  3. web (depends on api)

NETWORKING:
  • Default network is created automatically if no custom networks defined
  • Services can communicate using service names as hostnames
  • Internal networks isolate backend services from external access
  • Port mappings expose services to host system

VOLUMES AND DATA:
  • Named volumes are created and mounted as specified
  • Host bind mounts are validated and created if needed
  • Volume drivers (local, zfs, nfs) are configured per volume definition
  • Backup settings are applied to persistent volumes

CONFIGURATION PRECEDENCE:
  1. Command-line flags (--build-arg, --project-name, etc.)
  2. lxc-stack.yml service-level settings
  3. lxc-stack.yml global defaults (settings section)
  4. Global configuration file (.pxc.yaml)
  5. Built-in defaults

TROUBLESHOOTING:
  Common Issues:
  • "Circular dependency detected"
    → Review depends_on relationships in services
    → Use health checks instead of startup dependencies when possible
  
  • "Service build failed" 
    → Check individual LXCfile.yml syntax: pxc build -f path/LXCfile.yml --dry-run
    → Verify all build contexts and files exist
  
  • "Network creation failed"
    → Check Proxmox bridge availability (vmbr0, vmbr1, etc.)
    → Ensure subnet ranges don't conflict with existing networks
  
  • "Container start failed"
    → Check resource availability: pxc ps --all
    → Verify storage space and container ID conflicts
    → Review container logs for startup errors

SCALING AND LOAD BALANCING:
  • Use 'scale' parameter to run multiple instances of stateless services
  • Scaled instances are named service-1, service-2, etc.
  • Load balancing requires external proxy (nginx, haproxy, etc.)

DEVELOPMENT MODE:
  • Development overrides automatically applied when detected
  • Extra services (debug, docs) started alongside main services
  • Volume mounts enable live code reloading`,
	Example: `  # Start all services
  pxc up

  # Use custom stack file
  pxc up -f my-stack.yml

  # Start specific services only
  pxc up web database

  # Set project name for isolation
  pxc up --project-name myapp-prod

  # Build and start with arguments
  pxc up --build --build-arg VERSION=1.2.3

  # Detached mode (background)
  pxc up --detach

  # Dry run to validate configuration
  pxc up --dry-run --verbose

  # Force rebuild specific services
  pxc up --build web --build-arg NODE_ENV=development`,
	RunE: runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)

	// Up-specific flags
	upCmd.Flags().StringVarP(&stackFile, "file", "f", "", "Path to stack file (default: lxc-stack.yml)")
	upCmd.Flags().StringVar(&projectName, "project-name", "", "Project name (default: directory name)")
	upCmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run containers in background")
	upCmd.Flags().StringToStringVar(&buildArgs, "build-arg", map[string]string{}, "Set build-time variables")
	upCmd.Flags().StringSliceVar(&buildServices, "build", []string{}, "Build only specified services")
}

func runUp(cmd *cobra.Command, args []string) error {
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

	PrintInfo("Starting stack: %s", projectName)
	PrintInfo("Stack file: %s", stackFile)

	if IsVerbose() {
		printUpSummary()
	}

	if IsDryRun() {
		PrintWarning("Dry run mode - no actual deployment will be performed")
		return printUpDryRun()
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

	// Deploy the stack
	result, err := orchestrator.Up(stackFile)
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	// Print results
	printDeploymentResults(result)

	if !detach {
		PrintInfo("Use 'pxc ps' to view running containers")
		PrintInfo("Use 'pxc down' to stop and remove containers")
	}

	return nil
}

func printUpSummary() {
	// Load stack to show summary
	stack, err := config.LoadLXCStack(stackFile)
	if err != nil {
		return
	}

	fmt.Println("\nDeployment Summary:")
	fmt.Printf("  Project: %s\n", projectName)
	fmt.Printf("  Services: %d\n", len(stack.Services))

	if len(stack.Networks) > 0 {
		fmt.Printf("  Networks: %d\n", len(stack.Networks))
	}
	if len(stack.Volumes) > 0 {
		fmt.Printf("  Volumes: %d\n", len(stack.Volumes))
	}

	// Show services
	if len(stack.Services) > 0 {
		fmt.Println("  Service list:")
		serviceOrder, err := stack.GetServiceDependencyOrder()
		if err == nil {
			for i, serviceName := range serviceOrder {
				service := stack.Services[serviceName]
				buildInfo := ""
				if service.HasBuild() {
					buildInfo = " (build)"
				} else if service.Template != "" {
					buildInfo = fmt.Sprintf(" (template: %s)", service.Template)
				}

				fmt.Printf("    %d. %s%s\n", i+1, serviceName, buildInfo)
			}
		}
	}

	fmt.Println()
}

func printUpDryRun() error {
	// Load and validate stack
	stack, err := config.LoadLXCStack(stackFile)
	if err != nil {
		return err
	}

	if err := stack.Validate(); err != nil {
		return fmt.Errorf("invalid stack: %w", err)
	}

	fmt.Println("\nDry Run Plan:")

	// Show network creation
	if len(stack.Networks) > 0 {
		fmt.Printf("  1. Create networks: %s\n", strings.Join(getNetworkNames(stack), ", "))
	}

	// Show volume creation
	if len(stack.Volumes) > 0 {
		fmt.Printf("  2. Create volumes: %s\n", strings.Join(getVolumeNames(stack), ", "))
	}

	// Show service deployment order
	serviceOrder, err := stack.GetServiceDependencyOrder()
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	step := 3
	for _, serviceName := range serviceOrder {
		service := stack.Services[serviceName]

		if service.HasBuild() {
			fmt.Printf("  %d. Build template for service '%s'\n", step, serviceName)
			step++
		}

		fmt.Printf("  %d. Create and start container for service '%s'\n", step, serviceName)
		step++
	}

	// Show hooks
	if stack.Hooks != nil && len(stack.Hooks.PostStart) > 0 {
		fmt.Printf("  %d. Execute post-start hooks (%d hooks)\n", step, len(stack.Hooks.PostStart))
	}

	return nil
}

func printDeploymentResults(result *runner.DeploymentResult) {
	PrintSuccess("Stack deployed successfully in %v", result.DeploymentTime)

	if len(result.Services) > 0 {
		fmt.Println("\nServices:")
		for _, service := range result.Services {
			if service.Error != nil {
				PrintError("  %s: Failed - %v", service.Name, service.Error)
			} else {
				fmt.Printf("  ✓ %s: Container %d (%s)\n",
					service.Name, service.ContainerID, service.Status)
				if IsVerbose() && service.BuildTime > 0 {
					fmt.Printf("    Build time: %v\n", service.BuildTime)
				}
				if IsVerbose() && service.StartTime > 0 {
					fmt.Printf("    Start time: %v\n", service.StartTime)
				}
			}
		}
	}

	if len(result.Networks) > 0 {
		fmt.Println("\nNetworks:")
		for _, network := range result.Networks {
			if network.Error != nil {
				PrintError("  %s: Failed - %v", network.Name, network.Error)
			} else {
				fmt.Printf("  ✓ %s: %s\n", network.Name, network.Status)
			}
		}
	}

	if len(result.Volumes) > 0 {
		fmt.Println("\nVolumes:")
		for _, volume := range result.Volumes {
			if volume.Error != nil {
				PrintError("  %s: Failed - %v", volume.Name, volume.Error)
			} else {
				fmt.Printf("  ✓ %s: %s\n", volume.Name, volume.Status)
			}
		}
	}
}

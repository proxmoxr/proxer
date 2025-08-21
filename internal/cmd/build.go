package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brynnjknight/proxer/internal/models"
	"github.com/brynnjknight/proxer/pkg/builder"
	"github.com/brynnjknight/proxer/pkg/config"
	"github.com/spf13/cobra"
)

var (
	buildFile    string
	tag          string
	buildArgsBld map[string]string
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build [OPTIONS]",
	Short: "Build an LXC container template from an LXCfile",
	Long: `Build creates an LXC container template from an LXCfile.yml configuration.

The build process:
1. Creates a temporary LXC container from the base template
2. Executes all setup steps defined in the LXCfile
3. Applies configuration settings (resources, security, etc.)
4. Exports the configured container as a reusable template
5. Cleans up the temporary container

The resulting template can be used to create new containers or referenced
in lxc-stack.yml files for multi-container applications.`,
	Example: `  # Build from default LXCfile.yml
  pxc build

  # Build from specific file with custom tag
  pxc build -f ./web/LXCfile.yml -t webapp:1.0

  # Build with build arguments
  pxc build --build-arg NODE_ENV=production --build-arg VERSION=1.2.3

  # Dry run to see what would happen
  pxc build --dry-run`,
	RunE: runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)

	// Build-specific flags
	buildCmd.Flags().StringVarP(&buildFile, "file", "f", "LXCfile.yml", "Path to LXCfile")
	buildCmd.Flags().StringVarP(&tag, "tag", "t", "", "Template name and optionally tag (name:tag)")
	buildCmd.Flags().StringToStringVar(&buildArgsBld, "build-arg", map[string]string{}, "Set build-time variables")

	// Add examples for help
	buildCmd.SetUsageTemplate(buildCmd.UsageTemplate() + `
Examples:
  pxc build                                    # Build from LXCfile.yml
  pxc build -f custom.yml -t myapp:1.0        # Custom file and tag
  pxc build --build-arg NODE_ENV=production   # With build arguments
`)
}

func runBuild(cmd *cobra.Command, args []string) error {
	// Validate that the LXCfile exists
	if _, err := os.Stat(buildFile); os.IsNotExist(err) {
		return fmt.Errorf("LXCfile not found: %s", buildFile)
	}

	// Load and validate the LXCfile
	PrintInfo("Loading LXCfile: %s", buildFile)
	lxcfile, err := config.LoadLXCfile(buildFile)
	if err != nil {
		return fmt.Errorf("failed to load LXCfile: %w", err)
	}

	// Validate the configuration
	if err := lxcfile.Validate(); err != nil {
		return fmt.Errorf("invalid LXCfile: %w", err)
	}

	// Determine the template name
	templateName := tag
	if templateName == "" {
		templateName = lxcfile.GetTemplateName()
	}

	PrintInfo("Building template: %s", templateName)
	PrintInfo("Base template: %s", lxcfile.From)

	if IsVerbose() {
		printBuildSummary(lxcfile)
	}

	if IsDryRun() {
		PrintWarning("Dry run mode - no actual build will be performed")
		return printDryRunPlan(lxcfile, templateName)
	}

	// Create builder instance
	bldr := builder.New(&builder.Config{
		Verbose: IsVerbose(),
		DryRun:  IsDryRun(),
	})

	// Execute the build
	result, err := bldr.BuildTemplate(lxcfile, templateName, buildArgsBld)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Report success
	PrintSuccess("Template built successfully: %s", result.TemplateName)
	PrintInfo("Template path: %s", result.TemplatePath)
	PrintInfo("Build time: %v", result.BuildDuration)

	if IsVerbose() {
		PrintInfo("Container ID used: %d", result.ContainerID)
		PrintInfo("Steps executed: %d", len(result.ExecutedSteps))
	}

	return nil
}

func printBuildSummary(lxcfile *models.LXCfile) {
	fmt.Println("\nBuild Summary:")
	fmt.Printf("  Base: %s\n", lxcfile.From)
	
	if lxcfile.Metadata != nil {
		if lxcfile.Metadata.Description != "" {
			fmt.Printf("  Description: %s\n", lxcfile.Metadata.Description)
		}
		if lxcfile.Metadata.Author != "" {
			fmt.Printf("  Author: %s\n", lxcfile.Metadata.Author)
		}
	}

	fmt.Printf("  Setup steps: %d\n", len(lxcfile.Setup))
	
	if len(lxcfile.Cleanup) > 0 {
		fmt.Printf("  Cleanup steps: %d\n", len(lxcfile.Cleanup))
	}

	if lxcfile.Resources != nil {
		fmt.Printf("  Resources: %.1f cores, %d MB RAM\n", 
			lxcfile.Resources.Cores, lxcfile.Resources.Memory)
	}

	if len(lxcfile.Ports) > 0 {
		fmt.Printf("  Exposed ports: %d\n", len(lxcfile.Ports))
	}

	if len(lxcfile.Mounts) > 0 {
		fmt.Printf("  Mount points: %d\n", len(lxcfile.Mounts))
	}

	fmt.Println()
}

func printDryRunPlan(lxcfile *models.LXCfile, templateName string) error {
	fmt.Println("\nDry Run Plan:")
	fmt.Printf("  1. Create temporary container from base: %s\n", lxcfile.From)
	
	for i, step := range lxcfile.Setup {
		if step.Run != "" {
			fmt.Printf("  %d. Execute: %s\n", i+2, truncateString(step.Run, 60))
		} else if step.Copy != nil {
			fmt.Printf("  %d. Copy: %s -> %s\n", i+2, step.Copy.Source, step.Copy.Dest)
		} else if step.Env != nil {
			fmt.Printf("  %d. Set environment variables (%d vars)\n", i+2, len(step.Env))
		}
	}

	if lxcfile.Resources != nil {
		fmt.Printf("  %d. Configure resources\n", len(lxcfile.Setup)+2)
	}

	if len(lxcfile.Cleanup) > 0 {
		fmt.Printf("  %d. Run cleanup steps (%d steps)\n", len(lxcfile.Setup)+3, len(lxcfile.Cleanup))
	}

	fmt.Printf("  %d. Export template as: %s\n", len(lxcfile.Setup)+4, templateName)
	fmt.Printf("  %d. Clean up temporary container\n", len(lxcfile.Setup)+5)

	return nil
}

// getBuildFileDir returns the directory containing the LXCfile for resolving relative paths
func getBuildFileDir() string {
	return filepath.Dir(buildFile)
}
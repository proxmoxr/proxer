package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
	dryRun  bool

	// Version information
	version   string
	gitCommit string
	buildDate string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pxc",
	Short: "Docker-like developer experience for Proxmox LXC containers",
	Long: `pxc (Proxmox Container eXecutor) provides a Docker-like developer experience 
for managing LXC containers in Proxmox Virtual Environment.

Build and orchestrate LXC containers using familiar YAML configuration files,
while leveraging Proxmox's native features like snapshots, backups, and clustering.

CONFIGURATION:
  pxc looks for configuration files in the following order:
  1. --config flag value
  2. ./.pxc.yaml (current directory)
  3. $HOME/.pxc.yaml (home directory)

  Key configuration options:
    storage: "local-lvm"          # Container storage backend
    template_storage: "local"     # Template storage location
    proxmox_node: "pve"          # Target Proxmox node

ENVIRONMENT VARIABLES:
  PXC_STORAGE              Override default storage backend
  PXC_TEMPLATE_STORAGE     Override template storage location
  PXC_PROXMOX_NODE         Override target Proxmox node
  PXC_CONFIG               Override config file location

DOCUMENTATION:
  For comprehensive configuration documentation:
    docs/LXCfile-reference.md      # Container build configuration
    docs/lxc-stack-reference.md    # Multi-container orchestration
    docs/configuration-guide.md    # Best practices and patterns

TROUBLESHOOTING:
  • Use --dry-run to preview actions without execution
  • Use --verbose for detailed operation logging
  • Ensure 'pct' command is available and functional
  • Check Proxmox storage and template availability
  • Verify container ID availability (pxc ps --all)

EXIT CODES:
  0   Success
  1   General error (configuration, validation, etc.)
  2   Command execution failed
  3   Resource allocation failed`,
	Example: `  # Build a container template from LXCfile.yml
  pxc build -f LXCfile.yml -t myapp:1.0

  # Start a multi-container application
  pxc up -f lxc-stack.yml

  # List running containers
  pxc ps

  # Stop and remove containers
  pxc down -f lxc-stack.yml

  # Test configuration without execution
  pxc build --dry-run --verbose

  # Use custom storage configuration
  PXC_STORAGE=fast-ssd pxc build -t myapp:1.0`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// SetVersionInfo sets the version information for the CLI
func SetVersionInfo(v, commit, date string) {
	version = v
	gitCommit = commit
	buildDate = date
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pxc.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be done without executing")

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pxc version %s\n", version)
			if verbose {
				fmt.Printf("Git commit: %s\n", gitCommit)
				fmt.Printf("Build date: %s\n", buildDate)
			}
		},
	})
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".pxc" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".pxc")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil && verbose {
		fmt.Fprintln(os.Stderr, color.GreenString("Using config file: %s", viper.ConfigFileUsed()))
	}
}

// Utility functions for consistent output
func PrintSuccess(format string, args ...interface{}) {
	fmt.Printf(color.GreenString("✓ ")+format+"\n", args...)
}

func PrintWarning(format string, args ...interface{}) {
	fmt.Printf(color.YellowString("⚠ ")+format+"\n", args...)
}

func PrintError(format string, args ...interface{}) {
	fmt.Printf(color.RedString("✗ ")+format+"\n", args...)
}

func PrintInfo(format string, args ...interface{}) {
	fmt.Printf(color.BlueString("ℹ ")+format+"\n", args...)
}

// IsVerbose returns true if verbose mode is enabled
func IsVerbose() bool {
	return verbose
}

// IsDryRun returns true if dry-run mode is enabled
func IsDryRun() bool {
	return dryRun
}

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
while leveraging Proxmox's native features like snapshots, backups, and clustering.`,
	Example: `  # Build a container template from LXCfile.yml
  pxc build -f LXCfile.yml -t myapp:1.0

  # Start a multi-container application
  pxc up -f lxc-stack.yml

  # List running containers
  pxc ps

  # Stop and remove containers
  pxc down -f lxc-stack.yml`,
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
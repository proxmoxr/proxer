package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	
	"github.com/brynnjknight/proxer/internal/models"
)

func TestBuildCommand(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pxc-build-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save original directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Change to temp directory
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	tests := []struct {
		name        string
		setupFiles  map[string]string
		args        []string
		wantErr     bool
		errorContains string
		expectFiles []string
	}{
		{
			name: "missing LXCfile",
			args: []string{},
			wantErr: true,
			errorContains: "LXCfile not found",
		},
		{
			name: "invalid LXCfile syntax",
			setupFiles: map[string]string{
				"LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"
  invalid_syntax_here`,
			},
			args: []string{},
			wantErr: true,
			errorContains: "failed to load LXCfile",
		},
		{
			name: "valid LXCfile with dry run",
			setupFiles: map[string]string{
				"LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update && apt-get install -y curl"
  - copy:
      source: "./app"
      dest: "/opt/app"
      owner: "root:root"
      mode: "755"
metadata:
  name: "test-app"
  version: "1.0"
resources:
  cores: 2
  memory: 1024
ports:
  - container: 3000
    host: 8080
    protocol: "tcp"`,
				"app/index.js": "console.log('Hello World');",
			},
			args: []string{"--dry-run"},
			wantErr: false,
		},
		{
			name: "custom file and tag",
			setupFiles: map[string]string{
				"custom.yml": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"
metadata:
  name: "custom-app"`,
			},
			args: []string{"-f", "custom.yml", "-t", "myapp:2.0", "--dry-run"},
			wantErr: false,
		},
		{
			name: "build with build args",
			setupFiles: map[string]string{
				"LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "export NODE_ENV=${NODE_ENV:-development} && echo $NODE_ENV"
  - run: "echo Version: ${VERSION}"`,
			},
			args: []string{"--build-arg", "NODE_ENV=production", "--build-arg", "VERSION=1.2.3", "--dry-run"},
			wantErr: false,
		},
		{
			name: "invalid from field",
			setupFiles: map[string]string{
				"LXCfile.yml": `setup:
  - run: "apt-get update"`,
			},
			args: []string{"--dry-run"},
			wantErr: true,
			errorContains: "invalid LXCfile",
		},
		{
			name: "copy source doesn't exist",
			setupFiles: map[string]string{
				"LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - copy:
      source: "./nonexistent"
      dest: "/opt/app"`,
			},
			args: []string{"--dry-run"},
			wantErr: false, // Dry run should not validate file existence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing files
			os.Remove("LXCfile.yml")
			os.Remove("custom.yml")
			os.RemoveAll("app")

			// Setup test files
			for path, content := range tt.setupFiles {
				dir := filepath.Dir(path)
				if dir != "." {
					err := os.MkdirAll(dir, 0755)
					if err != nil {
						t.Fatalf("Failed to create directory %s: %v", dir, err)
					}
				}
				err := os.WriteFile(path, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create file %s: %v", path, err)
				}
			}

			// Reset viper for clean test
			viper.Reset()
			
			// Set default values
			viper.Set("proxmox_node", "localhost")
			viper.Set("storage", "local-lvm")
			viper.Set("template_storage", "local")

			// Create fresh command instance
			cmd := &cobra.Command{
				Use: "build",
				RunE: runBuild,
			}
			
			// Reset flags
			buildFile = ""
			tag = ""
			buildArgsBld = make(map[string]string)
			
			// Add flags
			cmd.Flags().StringVarP(&buildFile, "file", "f", "LXCfile.yml", "Path to LXCfile")
			cmd.Flags().StringVarP(&tag, "tag", "t", "", "Template name and optionally tag")
			cmd.Flags().StringToStringVar(&buildArgsBld, "build-arg", map[string]string{}, "Set build-time variables")
			cmd.Flags().Bool("dry-run", false, "Show what would be done without executing")
			cmd.Flags().Bool("verbose", false, "Enable verbose output")

			// Set args
			cmd.SetArgs(tt.args)

			// Execute command
			err := cmd.Execute()

			// Check results
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			// Check expected files
			for _, expectedFile := range tt.expectFiles {
				if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
					t.Errorf("Expected file %s to exist but it doesn't", expectedFile)
				}
			}
		})
	}
}

func TestBuildCommandFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantFile string
		wantTag  string
		wantArgs map[string]string
	}{
		{
			name:     "default flags",
			args:     []string{},
			wantFile: "LXCfile.yml",
			wantTag:  "",
			wantArgs: map[string]string{},
		},
		{
			name:     "custom file flag short",
			args:     []string{"-f", "custom.yml"},
			wantFile: "custom.yml",
			wantTag:  "",
		},
		{
			name:     "custom file flag long",
			args:     []string{"--file", "custom.yml"},
			wantFile: "custom.yml",
			wantTag:  "",
		},
		{
			name:     "tag flag short",
			args:     []string{"-t", "myapp:1.0"},
			wantFile: "LXCfile.yml",
			wantTag:  "myapp:1.0",
		},
		{
			name:     "tag flag long",
			args:     []string{"--tag", "myapp:1.0"},
			wantFile: "LXCfile.yml",
			wantTag:  "myapp:1.0",
		},
		{
			name:     "build args single",
			args:     []string{"--build-arg", "NODE_ENV=production"},
			wantFile: "LXCfile.yml",
			wantArgs: map[string]string{"NODE_ENV": "production"},
		},
		{
			name:     "build args multiple",
			args:     []string{"--build-arg", "NODE_ENV=production", "--build-arg", "VERSION=1.0"},
			wantFile: "LXCfile.yml",
			wantArgs: map[string]string{"NODE_ENV": "production", "VERSION": "1.0"},
		},
		{
			name:     "combined flags",
			args:     []string{"-f", "webapp.yml", "-t", "webapp:2.0", "--build-arg", "ENV=prod"},
			wantFile: "webapp.yml",
			wantTag:  "webapp:2.0",
			wantArgs: map[string]string{"ENV": "prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables
			buildFile = ""
			tag = ""
			buildArgsBld = make(map[string]string)

			// Create command
			cmd := &cobra.Command{
				Use: "build",
			}
			
			cmd.Flags().StringVarP(&buildFile, "file", "f", "LXCfile.yml", "Path to LXCfile")
			cmd.Flags().StringVarP(&tag, "tag", "t", "", "Template name and optionally tag")
			cmd.Flags().StringToStringVar(&buildArgsBld, "build-arg", map[string]string{}, "Set build-time variables")

			// Parse flags
			cmd.SetArgs(tt.args)
			err := cmd.ParseFlags(tt.args)
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			// Check results
			if buildFile != tt.wantFile {
				t.Errorf("buildFile = %v, want %v", buildFile, tt.wantFile)
			}
			if tt.wantTag != "" && tag != tt.wantTag {
				t.Errorf("tag = %v, want %v", tag, tt.wantTag)
			}
			if tt.wantArgs != nil {
				for k, v := range tt.wantArgs {
					if buildArgsBld[k] != v {
						t.Errorf("buildArgs[%s] = %v, want %v", k, buildArgsBld[k], v)
					}
				}
			}
		})
	}
}

func TestPrintBuildSummary(t *testing.T) {
	// This test captures output to verify the summary format
	// In a real implementation, you'd want to capture stdout
	lxcfile := createTestLXCfile()
	
	// Just ensure it doesn't panic
	printBuildSummary(lxcfile)
}

func TestPrintDryRunPlan(t *testing.T) {
	lxcfile := createTestLXCfile()
	
	err := printDryRunPlan(lxcfile, "test:1.0")
	if err != nil {
		t.Errorf("printDryRunPlan returned unexpected error: %v", err)
	}
}

// Helper function to create test LXCfile
func createTestLXCfile() *models.LXCfile {
	return &models.LXCfile{
		From: "ubuntu:22.04",
		Metadata: &models.Metadata{
			Name:        "test-app",
			Description: "Test application",
			Author:      "test@example.com",
		},
		Setup: []models.SetupStep{
			{Run: "apt-get update"},
			{Copy: &models.CopyStep{
				Source: "./app",
				Dest:   "/opt/app",
			}},
			{Env: map[string]string{
				"NODE_ENV": "production",
			}},
		},
		Cleanup: []models.SetupStep{
			{Run: "apt-get clean"},
		},
		Resources: &models.Resources{
			Cores:  2,
			Memory: 1024,
		},
		Ports: []models.Port{
			{Container: 3000, Host: 8080, Protocol: "tcp"},
		},
		Mounts: []models.Mount{
			{Source: "/host/data", Target: "/container/data", Type: "bind"},
		},
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "1234567890",
			maxLen:   10,
			expected: "1234567890",
		},
		{
			name:     "long string",
			input:    "this is a very long command that should be truncated",
			maxLen:   20,
			expected: "this is a very lo...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}
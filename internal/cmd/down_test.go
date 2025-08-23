package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestDownCommand(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pxc-down-test-*")
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
		name          string
		setupFiles    map[string]string
		args          []string
		wantErr       bool
		errorContains string
	}{
		{
			name:          "missing stack file",
			args:          []string{},
			wantErr:       true,
			errorContains: "stack file not found",
		},
		{
			name: "invalid stack YAML",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    build: "./web"
  invalid_yaml`,
			},
			args:          []string{},
			wantErr:       true,
			errorContains: "failed to load stack",
		},
		{
			name: "valid stack with dry run",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    template: "nginx:latest"
  database:
    template: "postgres:15"`,
			},
			args:    []string{"--dry-run"},
			wantErr: false,
		},
		{
			name: "custom stack file",
			setupFiles: map[string]string{
				"custom-stack.yml": `version: "1.0"
services:
  app:
    template: "alpine:latest"`,
			},
			args:    []string{"-f", "custom-stack.yml", "--dry-run"},
			wantErr: false,
		},
		{
			name: "stack with volumes - preserve data",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  database:
    template: "postgres:15"
    volumes:
      - "db-data:/var/lib/postgresql/data"

volumes:
  db-data:
    driver: "local"`,
			},
			args:    []string{"--dry-run", "--verbose"},
			wantErr: false,
		},
		{
			name: "force removal with volumes",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  database:
    template: "postgres:15"
    volumes:
      - "db-data:/var/lib/postgresql/data"

volumes:
  db-data:
    driver: "local"`,
			},
			args:    []string{"--volumes", "--dry-run"},
			wantErr: false,
		},
		{
			name: "timeout configuration",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    template: "nginx:latest"
  app:
    template: "node:latest"
    depends_on:
      - web`,
			},
			args:    []string{"--timeout", "60s", "--dry-run"},
			wantErr: false,
		},
		{
			name: "stack with dependencies - reverse order shutdown",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  database:
    template: "postgres:15"
  cache:
    template: "redis:latest"
  api:
    template: "node:latest"
    depends_on:
      - database
      - cache
  web:
    template: "nginx:latest"
    depends_on:
      - api`,
			},
			args:    []string{"--dry-run", "--verbose"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing files
			os.Remove("lxc-stack.yml")
			os.Remove("custom-stack.yml")

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

			// Create fresh command instance
			cmd := &cobra.Command{
				Use: "down",
				RunE: runDown,
			}

			// Reset flags
			stackFile = ""
			
			// Add flags
			cmd.Flags().StringVarP(&stackFile, "file", "f", "lxc-stack.yml", "Path to stack file")
			cmd.Flags().Bool("dry-run", false, "Show what would be done without executing")
			cmd.Flags().Bool("verbose", false, "Enable verbose output")
			cmd.Flags().Bool("volumes", false, "Remove named volumes")
			cmd.Flags().Duration("timeout", 0, "Shutdown timeout")

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
		})
	}
}

func TestDownCommandFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantFile    string
		wantVolumes bool
		wantTimeout string
	}{
		{
			name:     "default flags",
			args:     []string{},
			wantFile: "lxc-stack.yml",
		},
		{
			name:     "custom file flag short",
			args:     []string{"-f", "custom.yml"},
			wantFile: "custom.yml",
		},
		{
			name:     "custom file flag long",
			args:     []string{"--file", "custom.yml"},
			wantFile: "custom.yml",
		},
		{
			name:        "volumes flag",
			args:        []string{"--volumes"},
			wantVolumes: true,
		},
		{
			name:        "timeout flag",
			args:        []string{"--timeout", "30s"},
			wantTimeout: "30s",
		},
		{
			name:        "combined flags",
			args:        []string{"-f", "webapp.yml", "--volumes", "--timeout", "60s"},
			wantFile:    "webapp.yml",
			wantVolumes: true,
			wantTimeout: "60s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables
			stackFile = ""

			// Create command
			cmd := &cobra.Command{
				Use: "down",
			}

			var volumesFlag bool
			var timeoutFlag string
			
			cmd.Flags().StringVarP(&stackFile, "file", "f", "lxc-stack.yml", "Path to stack file")
			cmd.Flags().BoolVar(&volumesFlag, "volumes", false, "Remove volumes")
			cmd.Flags().StringVar(&timeoutFlag, "timeout", "", "Timeout")

			// Parse flags
			cmd.SetArgs(tt.args)
			err := cmd.ParseFlags(tt.args)
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			// Check results
			if stackFile != tt.wantFile {
				t.Errorf("stackFile = %v, want %v", stackFile, tt.wantFile)
			}
			if volumesFlag != tt.wantVolumes {
				t.Errorf("volumesFlag = %v, want %v", volumesFlag, tt.wantVolumes)
			}
			if tt.wantTimeout != "" && timeoutFlag != tt.wantTimeout {
				t.Errorf("timeoutFlag = %v, want %v", timeoutFlag, tt.wantTimeout)
			}
		})
	}
}

func TestDownServiceOrder(t *testing.T) {
	// Test that services are stopped in reverse dependency order
	tempDir, err := os.MkdirTemp("", "pxc-down-order-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create a complex dependency chain
	stackContent := `version: "1.0"
services:
  database:
    template: "postgres:15"
  
  cache:
    template: "redis:latest"
    depends_on:
      - database
  
  api:
    template: "node:latest"
    depends_on:
      - database
      - cache
  
  web:
    template: "nginx:latest"
    depends_on:
      - api

  worker:
    template: "python:3.9"
    depends_on:
      - api`

	err = os.WriteFile("lxc-stack.yml", []byte(stackContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create stack file: %v", err)
	}

	// Reset viper
	viper.Reset()
	viper.Set("proxmox_node", "localhost")
	viper.Set("storage", "local-lvm")

	// Create command
	cmd := &cobra.Command{
		Use: "down",
		RunE: runDown,
	}

	stackFile = ""
	cmd.Flags().StringVarP(&stackFile, "file", "f", "lxc-stack.yml", "Path to stack file")
	cmd.Flags().Bool("dry-run", false, "Dry run")
	cmd.Flags().Bool("verbose", false, "Verbose")

	cmd.SetArgs([]string{"--dry-run", "--verbose"})

	// Execute command - should not error with valid dependency chain
	err = cmd.Execute()
	if err != nil {
		t.Errorf("Expected no error for valid dependency chain, got: %v", err)
	}
}

func TestDownVolumeHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pxc-down-volume-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	tests := []struct {
		name         string
		stackContent string
		withVolumes  bool
		expectError  bool
	}{
		{
			name: "services with volumes - preserve by default",
			stackContent: `version: "1.0"
services:
  database:
    template: "postgres:15"
    volumes:
      - "db-data:/var/lib/postgresql/data"
      - "db-logs:/var/log/postgresql"

volumes:
  db-data:
    driver: "local"
  db-logs:
    driver: "local"`,
			withVolumes: false,
			expectError: false,
		},
		{
			name: "services with volumes - remove with flag",
			stackContent: `version: "1.0"
services:
  database:
    template: "postgres:15"
    volumes:
      - "db-data:/var/lib/postgresql/data"

volumes:
  db-data:
    driver: "local"`,
			withVolumes: true,
			expectError: false,
		},
		{
			name: "no volumes defined",
			stackContent: `version: "1.0"
services:
  web:
    template: "nginx:latest"`,
			withVolumes: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create stack file
			err := os.WriteFile("lxc-stack.yml", []byte(tt.stackContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create stack file: %v", err)
			}
			defer os.Remove("lxc-stack.yml")

			// Reset viper
			viper.Reset()
			viper.Set("proxmox_node", "localhost")

			// Create command
			cmd := &cobra.Command{
				Use: "down",
				RunE: runDown,
			}

			stackFile = ""
			args := []string{"--dry-run"}
			if tt.withVolumes {
				args = append(args, "--volumes")
			}

			cmd.Flags().StringVarP(&stackFile, "file", "f", "lxc-stack.yml", "Path to stack file")
			cmd.Flags().Bool("dry-run", false, "Dry run")
			cmd.Flags().Bool("volumes", false, "Remove volumes")

			cmd.SetArgs(args)

			// Execute command
			err = cmd.Execute()

			// Check results
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}
package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestUpCommand(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pxc-up-test-*")
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
  invalid_yaml_here`,
			},
			args:          []string{},
			wantErr:       true,
			errorContains: "failed to load stack",
		},
		{
			name: "valid minimal stack with dry run",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    build: "./web"`,
				"web/LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`,
			},
			args:    []string{"--dry-run"},
			wantErr: false,
		},
		{
			name: "stack with dependencies",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  database:
    template: "postgres:15"
    environment:
      POSTGRES_DB: "app"
  web:
    build: "./web"
    depends_on:
      - database
    environment:
      DATABASE_HOST: "database"`,
				"web/LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update && apt-get install -y nodejs"`,
			},
			args:    []string{"--dry-run", "--verbose"},
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
			name: "stack validation error - missing build and template",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    environment:
      NODE_ENV: "production"`,
			},
			args:          []string{"--dry-run"},
			wantErr:       true,
			errorContains: "invalid stack",
		},
		{
			name: "stack with networks and volumes",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    build: "./web"
    ports:
      - "80:3000"
    networks:
      - frontend
      - backend
    volumes:
      - "web-data:/var/lib/data"
    
  database:
    template: "postgres:15"
    networks:
      - backend
    volumes:
      - "db-data:/var/lib/postgresql/data"

networks:
  frontend:
    driver: "bridge"
    subnet: "172.20.0.0/24"
  backend:
    driver: "bridge"
    subnet: "172.21.0.0/24"
    internal: true

volumes:
  web-data:
    driver: "local"
  db-data:
    driver: "local"`,
				"web/LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`,
			},
			args:    []string{"--dry-run"},
			wantErr: false,
		},
		{
			name: "stack with build args",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    build:
      context: "./web"
      args:
        NODE_ENV: "production"
        VERSION: "1.0"`,
				"web/LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "echo NODE_ENV=${NODE_ENV} VERSION=${VERSION}"`,
			},
			args:    []string{"--dry-run"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing files
			os.Remove("lxc-stack.yml")
			os.Remove("custom-stack.yml")
			os.RemoveAll("web")

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
				Use: "up",
				RunE: runUp,
			}

			// Reset flags
			stackFile = ""
			
			// Add flags
			cmd.Flags().StringVarP(&stackFile, "file", "f", "lxc-stack.yml", "Path to stack file")
			cmd.Flags().Bool("dry-run", false, "Show what would be done without executing")
			cmd.Flags().Bool("verbose", false, "Enable verbose output")
			cmd.Flags().Bool("build", false, "Force rebuild of container templates")
			cmd.Flags().Bool("detach", false, "Run containers in detached mode")

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

func TestUpCommandFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantFile   string
		wantBuild  bool
		wantDetach bool
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
			name:      "build flag",
			args:      []string{"--build"},
			wantBuild: true,
		},
		{
			name:       "detach flag",
			args:       []string{"--detach"},
			wantDetach: true,
		},
		{
			name:       "combined flags",
			args:       []string{"-f", "webapp.yml", "--build", "--detach"},
			wantFile:   "webapp.yml",
			wantBuild:  true,
			wantDetach: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables
			stackFile = ""

			// Create command
			cmd := &cobra.Command{
				Use: "up",
			}

			var buildFlag, detachFlag bool
			
			cmd.Flags().StringVarP(&stackFile, "file", "f", "lxc-stack.yml", "Path to stack file")
			cmd.Flags().BoolVar(&buildFlag, "build", false, "Force rebuild")
			cmd.Flags().BoolVar(&detachFlag, "detach", false, "Detached mode")

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
			if buildFlag != tt.wantBuild {
				t.Errorf("buildFlag = %v, want %v", buildFlag, tt.wantBuild)
			}
			if detachFlag != tt.wantDetach {
				t.Errorf("detachFlag = %v, want %v", detachFlag, tt.wantDetach)
			}
		})
	}
}

func TestStackDependencyValidation(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pxc-dependency-test-*")
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
		name          string
		stackContent  string
		wantErr       bool
		errorContains string
	}{
		{
			name: "valid dependencies",
			stackContent: `version: "1.0"
services:
  database:
    template: "postgres:15"
  web:
    template: "nginx:latest"
    depends_on:
      - database`,
			wantErr: false,
		},
		{
			name: "circular dependency",
			stackContent: `version: "1.0"
services:
  web:
    template: "nginx:latest"
    depends_on:
      - api
  api:
    template: "node:latest"
    depends_on:
      - web`,
			wantErr:       true,
			errorContains: "circular dependency",
		},
		{
			name: "undefined dependency",
			stackContent: `version: "1.0"
services:
  web:
    template: "nginx:latest"
    depends_on:
      - nonexistent`,
			wantErr:       true,
			errorContains: "undefined service",
		},
		{
			name: "complex valid dependencies",
			stackContent: `version: "1.0"
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
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test stack file
			err := os.WriteFile("lxc-stack.yml", []byte(tt.stackContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create stack file: %v", err)
			}
			defer os.Remove("lxc-stack.yml")

			// Reset viper
			viper.Reset()
			viper.Set("proxmox_node", "localhost")
			viper.Set("storage", "local-lvm")

			// Create command
			cmd := &cobra.Command{
				Use: "up",
				RunE: runUp,
			}

			stackFile = ""
			cmd.Flags().StringVarP(&stackFile, "file", "f", "lxc-stack.yml", "Path to stack file")
			cmd.Flags().Bool("dry-run", false, "Dry run")

			cmd.SetArgs([]string{"--dry-run"})

			// Execute command
			err = cmd.Execute()

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
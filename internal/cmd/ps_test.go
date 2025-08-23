package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestPsCommand(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pxc-ps-test-*")
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
			name: "valid stack - default format",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    template: "nginx:latest"
    ports:
      - "80:3000"
  database:
    template: "postgres:15"
    environment:
      POSTGRES_DB: "app"`,
			},
			args:    []string{},
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
			args:    []string{"-f", "custom-stack.yml"},
			wantErr: false,
		},
		{
			name: "json output format",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    template: "nginx:latest"`,
			},
			args:    []string{"--format", "json"},
			wantErr: false,
		},
		{
			name: "wide output format",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    template: "nginx:latest"
    ports:
      - "80:3000"
  database:
    template: "postgres:15"`,
			},
			args:    []string{"--format", "wide"},
			wantErr: false,
		},
		{
			name: "quiet output format",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    template: "nginx:latest"`,
			},
			args:    []string{"-q"},
			wantErr: false,
		},
		{
			name: "filter by service",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    template: "nginx:latest"
  database:
    template: "postgres:15"
  cache:
    template: "redis:latest"`,
			},
			args:    []string{"--filter", "service=web"},
			wantErr: false,
		},
		{
			name: "filter by status",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    template: "nginx:latest"
  database:
    template: "postgres:15"`,
			},
			args:    []string{"--filter", "status=running"},
			wantErr: false,
		},
		{
			name: "all containers flag",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    template: "nginx:latest"`,
			},
			args:    []string{"-a"},
			wantErr: false,
		},
		{
			name: "complex stack with all options",
			setupFiles: map[string]string{
				"lxc-stack.yml": `version: "1.0"
services:
  web:
    template: "nginx:latest"
    ports:
      - "80:3000"
    networks:
      - frontend
    volumes:
      - "web-data:/var/www"
    restart: "always"
    scale: 2
    
  api:
    template: "node:latest"
    depends_on:
      - database
    environment:
      NODE_ENV: "production"
    networks:
      - frontend
      - backend
      
  database:
    template: "postgres:15"
    environment:
      POSTGRES_DB: "app"
    volumes:
      - "db-data:/var/lib/postgresql/data"
    networks:
      - backend

networks:
  frontend:
    driver: "bridge"
  backend:
    driver: "bridge"
    internal: true

volumes:
  web-data:
    driver: "local"
  db-data:
    driver: "local"`,
			},
			args:    []string{"--format", "wide", "-a"},
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
				Use: "ps",
				RunE: runPS,
			}

			// Reset flags
			stackFile = ""
			
			// Add flags
			cmd.Flags().StringVarP(&stackFile, "file", "f", "lxc-stack.yml", "Path to stack file")
			cmd.Flags().String("format", "table", "Output format")
			cmd.Flags().BoolP("quiet", "q", false, "Only show container IDs")
			cmd.Flags().BoolP("all", "a", false, "Show all containers")
			cmd.Flags().StringSlice("filter", []string{}, "Filter containers")

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

func TestPsCommandFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantFile   string
		wantFormat string
		wantQuiet  bool
		wantAll    bool
		wantFilter []string
	}{
		{
			name:       "default flags",
			args:       []string{},
			wantFile:   "lxc-stack.yml",
			wantFormat: "table",
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
			name:       "json format",
			args:       []string{"--format", "json"},
			wantFormat: "json",
		},
		{
			name:       "wide format",
			args:       []string{"--format", "wide"},
			wantFormat: "wide",
		},
		{
			name:      "quiet flag short",
			args:      []string{"-q"},
			wantQuiet: true,
		},
		{
			name:      "quiet flag long",
			args:      []string{"--quiet"},
			wantQuiet: true,
		},
		{
			name:    "all flag short",
			args:    []string{"-a"},
			wantAll: true,
		},
		{
			name:    "all flag long",
			args:    []string{"--all"},
			wantAll: true,
		},
		{
			name:       "single filter",
			args:       []string{"--filter", "service=web"},
			wantFilter: []string{"service=web"},
		},
		{
			name:       "multiple filters",
			args:       []string{"--filter", "service=web", "--filter", "status=running"},
			wantFilter: []string{"service=web", "status=running"},
		},
		{
			name:       "combined flags",
			args:       []string{"-f", "app.yml", "--format", "json", "-a", "-q", "--filter", "service=api"},
			wantFile:   "app.yml",
			wantFormat: "json",
			wantAll:    true,
			wantQuiet:  true,
			wantFilter: []string{"service=api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables
			stackFile = ""

			// Create command
			cmd := &cobra.Command{
				Use: "ps",
			}

			var formatFlag string
			var quietFlag, allFlag bool
			var filterFlag []string
			
			cmd.Flags().StringVarP(&stackFile, "file", "f", "lxc-stack.yml", "Path to stack file")
			cmd.Flags().StringVar(&formatFlag, "format", "table", "Output format")
			cmd.Flags().BoolVarP(&quietFlag, "quiet", "q", false, "Quiet mode")
			cmd.Flags().BoolVarP(&allFlag, "all", "a", false, "All containers")
			cmd.Flags().StringSliceVar(&filterFlag, "filter", []string{}, "Filters")

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
			if tt.wantFormat != "" && formatFlag != tt.wantFormat {
				t.Errorf("formatFlag = %v, want %v", formatFlag, tt.wantFormat)
			}
			if quietFlag != tt.wantQuiet {
				t.Errorf("quietFlag = %v, want %v", quietFlag, tt.wantQuiet)
			}
			if allFlag != tt.wantAll {
				t.Errorf("allFlag = %v, want %v", allFlag, tt.wantAll)
			}
			if tt.wantFilter != nil {
				if len(filterFlag) != len(tt.wantFilter) {
					t.Errorf("filterFlag length = %d, want %d", len(filterFlag), len(tt.wantFilter))
				} else {
					for i, filter := range tt.wantFilter {
						if filterFlag[i] != filter {
							t.Errorf("filterFlag[%d] = %v, want %v", i, filterFlag[i], filter)
						}
					}
				}
			}
		})
	}
}

func TestPsOutputFormats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pxc-ps-format-test-*")
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

	// Create test stack
	stackContent := `version: "1.0"
services:
  web:
    template: "nginx:latest"
    ports:
      - "80:3000"
  database:
    template: "postgres:15"`

	err = os.WriteFile("lxc-stack.yml", []byte(stackContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create stack file: %v", err)
	}

	tests := []struct {
		name   string
		format string
	}{
		{"table format", "table"},
		{"json format", "json"},
		{"wide format", "wide"},
		{"yaml format", "yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper
			viper.Reset()
			viper.Set("proxmox_node", "localhost")

			// Create command
			cmd := &cobra.Command{
				Use: "ps",
				RunE: runPS,
			}

			stackFile = ""
			cmd.Flags().StringVarP(&stackFile, "file", "f", "lxc-stack.yml", "Path to stack file")
			cmd.Flags().String("format", "table", "Output format")

			cmd.SetArgs([]string{"--format", tt.format})

			// Execute command - should not error
			err = cmd.Execute()
			if err != nil {
				t.Errorf("Format %s failed with error: %v", tt.format, err)
			}
		})
	}
}

func TestPsFiltering(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pxc-ps-filter-test-*")
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

	// Create complex stack for filtering tests
	stackContent := `version: "1.0"
services:
  web:
    template: "nginx:latest"
    labels:
      environment: "production"
      tier: "frontend"
  api:
    template: "node:latest"
    labels:
      environment: "production"
      tier: "backend"
  test-db:
    template: "postgres:15"
    labels:
      environment: "testing"
      tier: "database"`

	err = os.WriteFile("lxc-stack.yml", []byte(stackContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create stack file: %v", err)
	}

	tests := []struct {
		name    string
		filters []string
		wantErr bool
	}{
		{
			name:    "filter by service name",
			filters: []string{"service=web"},
		},
		{
			name:    "filter by status",
			filters: []string{"status=running"},
		},
		{
			name:    "filter by label",
			filters: []string{"label=environment=production"},
		},
		{
			name:    "filter by multiple labels",
			filters: []string{"label=environment=production", "label=tier=frontend"},
		},
		{
			name:    "multiple different filters",
			filters: []string{"service=web", "status=running"},
		},
		{
			name:    "invalid filter format",
			filters: []string{"invalid-filter"},
			wantErr: false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper
			viper.Reset()
			viper.Set("proxmox_node", "localhost")

			// Create command
			cmd := &cobra.Command{
				Use: "ps",
				RunE: runPS,
			}

			stackFile = ""
			cmd.Flags().StringVarP(&stackFile, "file", "f", "lxc-stack.yml", "Path to stack file")
			cmd.Flags().StringSlice("filter", []string{}, "Filters")

			args := []string{}
			for _, filter := range tt.filters {
				args = append(args, "--filter", filter)
			}

			cmd.SetArgs(args)

			// Execute command
			err = cmd.Execute()

			// Check results
			if tt.wantErr {
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
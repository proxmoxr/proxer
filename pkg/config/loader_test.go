package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brynnjknight/proxer/internal/models"
)

func TestLoadLXCfile(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pxc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name       string
		content    string
		wantErr    bool
		errorMsg   string
		validate   func(*testing.T, *models.LXCfile)
	}{
		{
			name: "valid minimal LXCfile",
			content: `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`,
			wantErr: false,
			validate: func(t *testing.T, lxc *models.LXCfile) {
				if lxc.From != "ubuntu:22.04" {
					t.Errorf("From = %v, want ubuntu:22.04", lxc.From)
				}
				if len(lxc.Setup) != 1 {
					t.Errorf("Setup length = %v, want 1", len(lxc.Setup))
				}
				if lxc.Setup[0].Run != "apt-get update" {
					t.Errorf("Setup[0].Run = %v, want apt-get update", lxc.Setup[0].Run)
				}
			},
		},
		{
			name: "complete LXCfile with all fields",
			content: `from: "ubuntu:22.04"

metadata:
  name: "webapp"
  description: "Web application container"
  version: "1.0.0"
  author: "developer@example.com"

features:
  unprivileged: true
  nesting: false
  keyctl: false
  fuse: false

resources:
  cores: 2
  memory: 1024
  swap: 512

security:
  isolation: "strict"
  apparmor: true
  seccomp: true

setup:
  - run: "apt-get update && apt-get install -y nodejs npm"
  - copy:
      source: "./app"
      dest: "/opt/app"
      owner: "www-data:www-data"
      mode: "755"
  - env:
      NODE_ENV: "production"
      PORT: "3000"

ports:
  - container: 3000
    host: 8080
    protocol: "tcp"

health:
  test: "curl -f http://localhost:3000 || exit 1"
  interval: "30s"
  timeout: "5s"
  retries: 3

labels:
  environment: "production"
  version: "1.0.0"`,
			wantErr: false,
			validate: func(t *testing.T, lxc *models.LXCfile) {
				if lxc.Metadata == nil || lxc.Metadata.Name != "webapp" {
					t.Errorf("Metadata.Name = %v, want webapp", lxc.Metadata.Name)
				}
				if lxc.Features == nil || !lxc.Features.Unprivileged {
					t.Errorf("Features.Unprivileged = %v, want true", lxc.Features.Unprivileged)
				}
				if lxc.Resources == nil || lxc.Resources.Cores != 2 {
					t.Errorf("Resources.Cores = %v, want 2", lxc.Resources.Cores)
				}
				if len(lxc.Setup) != 3 {
					t.Errorf("Setup length = %v, want 3", len(lxc.Setup))
				}
				if len(lxc.Ports) != 1 {
					t.Errorf("Ports length = %v, want 1", len(lxc.Ports))
				}
			},
		},
		{
			name: "invalid YAML syntax",
			content: `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"
  invalid_indentation`,
			wantErr:  true,
			errorMsg: "failed to parse LXCfile YAML",
		},
		{
			name: "missing required field",
			content: `setup:
  - run: "apt-get update"`,
			wantErr:  false, // Loader doesn't validate, just parses YAML
		},
		{
			name: "copy step with relative paths",
			content: `from: "ubuntu:22.04"
setup:
  - copy:
      source: "./app"
      dest: "/opt/app"`,
			wantErr: false,
			validate: func(t *testing.T, lxc *models.LXCfile) {
				if len(lxc.Setup) != 1 {
					t.Fatalf("Setup length = %v, want 1", len(lxc.Setup))
				}
				if lxc.Setup[0].Copy == nil {
					t.Fatalf("Copy step is nil")
				}
				// Path should be resolved relative to the LXCfile location
				expectedPath := filepath.Join(tempDir, "app")
				if lxc.Setup[0].Copy.Source != expectedPath {
					t.Errorf("Copy.Source = %v, want %v", lxc.Setup[0].Copy.Source, expectedPath)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tempDir, "LXCfile.yml")
			err := os.WriteFile(testFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Create test app directory for copy tests
			if tt.name == "copy step with relative paths" {
				appDir := filepath.Join(tempDir, "app")
				err := os.MkdirAll(appDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create app directory: %v", err)
				}
			}

			// Load the LXCfile
			lxcfile, err := LoadLXCfile(testFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadLXCfile() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errorMsg != "" {
					if !containsString(err.Error(), tt.errorMsg) {
						t.Errorf("LoadLXCfile() error = %v, want error containing %v", err.Error(), tt.errorMsg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("LoadLXCfile() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if tt.validate != nil {
					tt.validate(t, lxcfile)
				}
			}
		})
	}
}

func TestLoadLXCStack(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pxc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name       string
		content    string
		wantErr    bool
		errorMsg   string
		validate   func(*testing.T, *models.LXCStack)
	}{
		{
			name: "valid minimal stack",
			content: `version: "1.0"
services:
  web:
    build: "./web"`,
			wantErr: false,
			validate: func(t *testing.T, stack *models.LXCStack) {
				if stack.Version != "1.0" {
					t.Errorf("Version = %v, want 1.0", stack.Version)
				}
				if len(stack.Services) != 1 {
					t.Errorf("Services length = %v, want 1", len(stack.Services))
				}
				if _, ok := stack.Services["web"]; !ok {
					t.Errorf("Service 'web' not found")
				}
			},
		},
		{
			name: "complete stack with all sections",
			content: `version: "1.0"

metadata:
  name: "webapp-stack"
  description: "Complete web application stack"

services:
  web:
    build: "./web"
    ports:
      - "80:3000"
    depends_on:
      - database
    environment:
      NODE_ENV: "production"
      DATABASE_HOST: "database"
    networks:
      - frontend
      - backend

  database:
    template: "postgres:15"
    environment:
      POSTGRES_DB: "webapp"
      POSTGRES_USER: "webapp"
    volumes:
      - "db-data:/var/lib/postgresql/data"
    networks:
      - backend

volumes:
  db-data:
    driver: "local"

networks:
  frontend:
    driver: "bridge"
    subnet: "172.20.0.0/24"
  backend:
    driver: "bridge"
    subnet: "172.21.0.0/24"
    internal: true`,
			wantErr: false,
			validate: func(t *testing.T, stack *models.LXCStack) {
				if len(stack.Services) != 2 {
					t.Errorf("Services length = %v, want 2", len(stack.Services))
				}
				if len(stack.Networks) != 2 {
					t.Errorf("Networks length = %v, want 2", len(stack.Networks))
				}
				if len(stack.Volumes) != 1 {
					t.Errorf("Volumes length = %v, want 1", len(stack.Volumes))
				}
			},
		},
		{
			name: "invalid YAML",
			content: `version: "1.0"
services:
  web:
    build: "./web"
  invalid_yaml_structure`,
			wantErr:  true,
			errorMsg: "failed to parse lxc-stack YAML",
		},
		{
			name: "stack validation error",
			content: `version: "1.0"
services:
  web:
    # Missing both build and template
    environment:
      NODE_ENV: "production"`,
			wantErr:  false, // Loader doesn't validate, just parses YAML
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tempDir, "lxc-stack.yml")
			err := os.WriteFile(testFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Load the stack
			stack, err := LoadLXCStack(testFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadLXCStack() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errorMsg != "" {
					if !containsString(err.Error(), tt.errorMsg) {
						t.Errorf("LoadLXCStack() error = %v, want error containing %v", err.Error(), tt.errorMsg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("LoadLXCStack() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if tt.validate != nil {
					tt.validate(t, stack)
				}
			}
		})
	}
}

func TestGetDefaultLXCfile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "pxc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save current directory
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
		createFiles []string
		expected    string
	}{
		{
			name:        "LXCfile.yml exists",
			createFiles: []string{"LXCfile.yml"},
			expected:    "LXCfile.yml",
		},
		{
			name:        "LXCfile.yaml exists",
			createFiles: []string{"LXCfile.yaml"},
			expected:    "LXCfile.yaml",
		},
		{
			name:        "lxcfile.yml exists",
			createFiles: []string{"lxcfile.yml"},
			expected:    "lxcfile.yml",
		},
		{
			name:        "multiple files exist - priority order",
			createFiles: []string{"lxcfile.yml", "LXCfile.yaml", "LXCfile.yml"},
			expected:    "LXCfile.yml", // Should prefer this one
		},
		{
			name:        "no files exist",
			createFiles: []string{},
			expected:    "LXCfile.yml", // Default fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing files
			for _, filename := range []string{"LXCfile.yml", "LXCfile.yaml", "lxcfile.yml"} {
				os.Remove(filename)
			}

			// Create test files
			for _, filename := range tt.createFiles {
				err := os.WriteFile(filename, []byte("test"), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}

			result := GetDefaultLXCfile()
			if result != tt.expected {
				t.Errorf("GetDefaultLXCfile() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetDefaultStackfile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "pxc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save current directory
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
		createFiles []string
		expected    string
	}{
		{
			name:        "lxc-stack.yml exists",
			createFiles: []string{"lxc-stack.yml"},
			expected:    "lxc-stack.yml",
		},
		{
			name:        "lxc-stack.yaml exists",
			createFiles: []string{"lxc-stack.yaml"},
			expected:    "lxc-stack.yaml",
		},
		{
			name:        "docker-compose.yml exists",
			createFiles: []string{"docker-compose.yml"},
			expected:    "docker-compose.yml",
		},
		{
			name:        "multiple files exist - priority order",
			createFiles: []string{"docker-compose.yml", "lxc-stack.yaml", "lxc-stack.yml"},
			expected:    "lxc-stack.yml", // Should prefer this one
		},
		{
			name:        "no files exist",
			createFiles: []string{},
			expected:    "lxc-stack.yml", // Default fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing files
			for _, filename := range []string{"lxc-stack.yml", "lxc-stack.yaml", "docker-compose.yml"} {
				os.Remove(filename)
			}

			// Create test files
			for _, filename := range tt.createFiles {
				err := os.WriteFile(filename, []byte("test"), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}

			result := GetDefaultStackfile()
			if result != tt.expected {
				t.Errorf("GetDefaultStackfile() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLoadLXCfileNonExistentFile(t *testing.T) {
	_, err := LoadLXCfile("nonexistent-file.yml")
	if err == nil {
		t.Errorf("LoadLXCfile() with nonexistent file should return error")
	}
	if !containsString(err.Error(), "failed to read LXCfile") {
		t.Errorf("LoadLXCfile() error should mention failed to read LXCfile, got: %v", err)
	}
}

func TestLoadLXCStackNonExistentFile(t *testing.T) {
	_, err := LoadLXCStack("nonexistent-stack.yml")
	if err == nil {
		t.Errorf("LoadLXCStack() with nonexistent file should return error")
	}
	if !containsString(err.Error(), "failed to read lxc-stack file") {
		t.Errorf("LoadLXCStack() error should mention failed to read lxc-stack file, got: %v", err)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}
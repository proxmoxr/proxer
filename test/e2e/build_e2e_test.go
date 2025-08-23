package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// MockProxmoxEnvironment sets up mock pct commands for testing
type MockProxmoxEnvironment struct {
	tempDir       string
	mockPctPath   string
	commands      []string
	shouldFail    map[string]bool
	containerData map[int]ContainerInfo
}

type ContainerInfo struct {
	ID       int
	Status   string
	Template string
	Name     string
	Created  time.Time
}

func NewMockProxmoxEnvironment(t *testing.T) *MockProxmoxEnvironment {
	tempDir, err := os.MkdirTemp("", "pxc-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	mockPctPath := filepath.Join(tempDir, "pct")
	env := &MockProxmoxEnvironment{
		tempDir:       tempDir,
		mockPctPath:   mockPctPath,
		commands:      []string{},
		shouldFail:    make(map[string]bool),
		containerData: make(map[int]ContainerInfo),
	}

	// Create mock pct script
	env.createMockPctScript(t)

	return env
}

func (e *MockProxmoxEnvironment) createMockPctScript(t *testing.T) {
	mockScript := fmt.Sprintf(`#!/bin/bash
# Mock pct command for testing
echo "$@" >> %s/pct_commands.log

COMMAND="$1"
CONTAINER_ID="$2"

case "$COMMAND" in
    "create")
        if [ -f "%s/fail_create" ]; then
            echo "Error: failed to create container" >&2
            exit 1
        fi
        echo "Creating container $CONTAINER_ID"
        echo "created:$CONTAINER_ID" >> %s/container_state.log
        ;;
    "start")
        if [ -f "%s/fail_start" ]; then
            echo "Error: failed to start container" >&2
            exit 1
        fi
        echo "Starting container $CONTAINER_ID"
        echo "started:$CONTAINER_ID" >> %s/container_state.log
        ;;
    "stop")
        echo "Stopping container $CONTAINER_ID"
        echo "stopped:$CONTAINER_ID" >> %s/container_state.log
        ;;
    "exec")
        if [ "$3" = "--" ] && [ "$4" = "echo" ] && [ "$5" = "ready" ]; then
            # Container readiness check
            if [ -f "%s/container_not_ready" ]; then
                exit 1
            fi
            echo "ready"
            exit 0
        fi
        # Regular exec command
        if [ -f "%s/fail_exec" ]; then
            echo "Error: failed to execute command" >&2
            exit 1
        fi
        echo "Executing: ${@:3}"
        ;;
    "push")
        if [ -f "%s/fail_push" ]; then
            echo "Error: failed to push files" >&2
            exit 1
        fi
        echo "Pushing files to container $CONTAINER_ID"
        ;;
    "set")
        echo "Setting configuration for container $CONTAINER_ID"
        ;;
    "template")
        if [ -f "%s/fail_template" ]; then
            echo "Error: failed to create template" >&2
            exit 1
        fi
        echo "Converting container $CONTAINER_ID to template"
        echo "templated:$CONTAINER_ID" >> %s/container_state.log
        ;;
    "destroy")
        echo "Destroying container $CONTAINER_ID"
        echo "destroyed:$CONTAINER_ID" >> %s/container_state.log
        ;;
    "list")
        # Mock container list output
        cat << EOF
VMID  Status   Lock  Name
100   running        test-web-1
101   stopped        test-db-1
EOF
        ;;
    *)
        echo "Unknown command: $COMMAND" >&2
        exit 1
        ;;
esac

exit 0
`, e.tempDir, e.tempDir, e.tempDir, e.tempDir, e.tempDir, e.tempDir, e.tempDir, e.tempDir, e.tempDir, e.tempDir, e.tempDir, e.tempDir)

	err := os.WriteFile(e.mockPctPath, []byte(mockScript), 0755)
	if err != nil {
		t.Fatalf("Failed to create mock pct script: %v", err)
	}
}

func (e *MockProxmoxEnvironment) SetFailure(command string, shouldFail bool) {
	failFile := filepath.Join(e.tempDir, "fail_"+command)
	if shouldFail {
		os.WriteFile(failFile, []byte("fail"), 0644)
	} else {
		os.Remove(failFile)
	}
}

func (e *MockProxmoxEnvironment) SetContainerNotReady(notReady bool) {
	notReadyFile := filepath.Join(e.tempDir, "container_not_ready")
	if notReady {
		os.WriteFile(notReadyFile, []byte("not ready"), 0644)
	} else {
		os.Remove(notReadyFile)
	}
}

func (e *MockProxmoxEnvironment) GetExecutedCommands() []string {
	commandsFile := filepath.Join(e.tempDir, "pct_commands.log")
	data, err := os.ReadFile(commandsFile)
	if err != nil {
		return []string{}
	}
	
	lines := strings.Split(string(data), "\n")
	var commands []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			commands = append(commands, strings.TrimSpace(line))
		}
	}
	return commands
}

func (e *MockProxmoxEnvironment) GetContainerStates() []string {
	stateFile := filepath.Join(e.tempDir, "container_state.log")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return []string{}
	}
	
	lines := strings.Split(string(data), "\n")
	var states []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			states = append(states, strings.TrimSpace(line))
		}
	}
	return states
}

func (e *MockProxmoxEnvironment) Cleanup() {
	os.RemoveAll(e.tempDir)
}

func (e *MockProxmoxEnvironment) GetMockPctPath() string {
	return e.mockPctPath
}

func (e *MockProxmoxEnvironment) GetTempDir() string {
	return e.tempDir
}

func TestBuildE2E(t *testing.T) {
	// Setup mock environment
	mockEnv := NewMockProxmoxEnvironment(t)
	defer mockEnv.Cleanup()

	// Create test project directory
	projectDir := filepath.Join(mockEnv.GetTempDir(), "test-project")
	err := os.MkdirAll(projectDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Save original directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(projectDir)
	if err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}

	tests := []struct {
		name           string
		lxcfileContent string
		appFiles       map[string]string
		buildArgs      []string
		expectError    bool
		errorContains  string
		mockFailures   map[string]bool
		expectedCmds   []string
		expectedStates []string
	}{
		{
			name: "successful simple build",
			lxcfileContent: `from: "ubuntu:22.04"
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
  memory: 1024`,
			appFiles: map[string]string{
				"app/index.js": "console.log('Hello World');",
				"app/package.json": `{"name": "test-app", "version": "1.0.0"}`,
			},
			buildArgs:   []string{},
			expectError: false,
			expectedStates: []string{
				"created:",
				"started:",
				"stopped:",
				"templated:",
			},
		},
		{
			name: "build with build arguments",
			lxcfileContent: `from: "ubuntu:22.04"
setup:
  - run: "export NODE_ENV=${NODE_ENV} && echo $NODE_ENV"
  - run: "echo Version: ${VERSION}"
metadata:
  name: "test-app"`,
			buildArgs:   []string{"--build-arg", "NODE_ENV=production", "--build-arg", "VERSION=2.0"},
			expectError: false,
		},
		{
			name: "build failure during container creation",
			lxcfileContent: `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`,
			mockFailures: map[string]bool{
				"create": true,
			},
			expectError:   true,
			errorContains: "failed to create",
		},
		{
			name: "build failure during container start",
			lxcfileContent: `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`,
			mockFailures: map[string]bool{
				"start": true,
			},
			expectError:   true,
			errorContains: "failed to start",
		},
		{
			name: "build failure during command execution",
			lxcfileContent: `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`,
			mockFailures: map[string]bool{
				"exec": true,
			},
			expectError:   true,
			errorContains: "failed to execute",
		},
		{
			name: "build failure during file copy",
			lxcfileContent: `from: "ubuntu:22.04"
setup:
  - copy:
      source: "./app"
      dest: "/opt/app"`,
			appFiles: map[string]string{
				"app/test.txt": "test file",
			},
			mockFailures: map[string]bool{
				"push": true,
			},
			expectError:   true,
			errorContains: "failed to copy",
		},
		{
			name: "build failure during template creation",
			lxcfileContent: `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`,
			mockFailures: map[string]bool{
				"template": true,
			},
			expectError:   true,
			errorContains: "failed to export template",
		},
		{
			name: "container not ready timeout",
			lxcfileContent: `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`,
			mockFailures: map[string]bool{
				// This will be handled by SetContainerNotReady
			},
			expectError:   true,
			errorContains: "did not become ready",
		},
		{
			name: "complex build with all features",
			lxcfileContent: `from: "ubuntu:22.04"

metadata:
  name: "webapp"
  description: "Complete web application"
  version: "1.0.0"
  author: "test@example.com"

features:
  unprivileged: true
  nesting: true
  keyctl: false
  fuse: true

resources:
  cores: 4
  memory: 2048
  swap: 512

security:
  isolation: "strict"
  apparmor: true
  seccomp: true

setup:
  - run: "apt-get update && apt-get install -y nodejs npm nginx"
  - copy:
      source: "./app"
      dest: "/opt/app"
      owner: "www-data:www-data"
      mode: "755"
  - env:
      NODE_ENV: "production"
      PORT: "3000"
      DATABASE_URL: "postgresql://user:pass@localhost/app"
  - run: "cd /opt/app && npm install --production"

cleanup:
  - run: "apt-get clean"
  - run: "rm -rf /var/lib/apt/lists/*"

ports:
  - container: 3000
    host: 8080
    protocol: "tcp"
  - container: 80
    host: 8081
    protocol: "tcp"

mounts:
  - source: "/host/logs"
    target: "/var/log/app"
    type: "bind"
    readonly: false
  - target: "/tmp/cache"
    type: "volume"
    size: "1G"

health:
  test: "curl -f http://localhost:3000/health || exit 1"
  interval: "30s"
  timeout: "10s"
  retries: 3
  start_period: "60s"

labels:
  environment: "production"
  version: "1.0.0"
  maintainer: "test@example.com"`,
			appFiles: map[string]string{
				"app/package.json": `{
  "name": "webapp",
  "version": "1.0.0",
  "main": "server.js",
  "dependencies": {
    "express": "^4.18.0"
  }
}`,
				"app/server.js": `
const express = require('express');
const app = express();

app.get('/health', (req, res) => {
  res.json({ status: 'healthy' });
});

app.listen(3000, () => {
  console.log('Server running on port 3000');
});`,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up from previous test
			os.Remove("LXCfile.yml")
			os.RemoveAll("app")
			os.Remove(filepath.Join(mockEnv.GetTempDir(), "pct_commands.log"))
			os.Remove(filepath.Join(mockEnv.GetTempDir(), "container_state.log"))

			// Setup mock failures
			for cmd, shouldFail := range tt.mockFailures {
				mockEnv.SetFailure(cmd, shouldFail)
			}

			// Special handling for container readiness test
			if strings.Contains(tt.errorContains, "did not become ready") {
				mockEnv.SetContainerNotReady(true)
			}

			// Create LXCfile
			err := os.WriteFile("LXCfile.yml", []byte(tt.lxcfileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create LXCfile: %v", err)
			}

			// Create app files
			for path, content := range tt.appFiles {
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

			// Build pxc command
			pxcPath := buildPxcBinary(t, mockEnv.GetTempDir())
			defer os.Remove(pxcPath)

			// Prepare command arguments
			args := []string{"build", "--dry-run"} // Use dry-run to avoid actual pct calls initially
			args = append(args, tt.buildArgs...)

			// Set PATH to include mock pct
			env := append(os.Environ(), fmt.Sprintf("PATH=%s:%s", filepath.Dir(mockEnv.GetMockPctPath()), os.Getenv("PATH")))

			// For tests that shouldn't use dry-run (to test actual command execution)
			if !tt.expectError || !strings.Contains(tt.name, "failure") {
				// Remove dry-run for success cases to test actual execution
				args = []string{"build"}
				args = append(args, tt.buildArgs...)
			}

			// Execute pxc build
			cmd := exec.Command(pxcPath, args...)
			cmd.Env = env
			cmd.Dir = projectDir

			output, err := cmd.CombinedOutput()

			// Check results
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but command succeeded. Output: %s", string(output))
					return
				}
				if tt.errorContains != "" && !strings.Contains(string(output), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %s", tt.errorContains, string(output))
				}
			} else {
				if err != nil {
					t.Errorf("Expected success but got error: %v. Output: %s", err, string(output))
					return
				}
			}

			// Verify expected commands were executed (for non-dry-run tests)
			if len(tt.expectedCmds) > 0 {
				executedCmds := mockEnv.GetExecutedCommands()
				for _, expectedCmd := range tt.expectedCmds {
					found := false
					for _, cmd := range executedCmds {
						if strings.Contains(cmd, expectedCmd) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected command containing %q was not executed. Commands: %v", expectedCmd, executedCmds)
					}
				}
			}

			// Verify expected container state changes
			if len(tt.expectedStates) > 0 {
				states := mockEnv.GetContainerStates()
				for _, expectedState := range tt.expectedStates {
					found := false
					for _, state := range states {
						if strings.Contains(state, expectedState) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected state %q was not found. States: %v", expectedState, states)
					}
				}
			}

			// Reset mock failures
			for cmd := range tt.mockFailures {
				mockEnv.SetFailure(cmd, false)
			}
			mockEnv.SetContainerNotReady(false)
		})
	}
}

// buildPxcBinary compiles the pxc binary for testing
func buildPxcBinary(t *testing.T, tempDir string) string {
	// Find the project root (go back from test/e2e to project root)
	projectRoot := filepath.Join("..", "..", "..")
	pxcPath := filepath.Join(tempDir, "pxc")

	cmd := exec.Command("go", "build", "-o", pxcPath, filepath.Join(projectRoot, "cmd", "pxc"))
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build pxc binary: %v. Output: %s", err, string(output))
	}

	return pxcPath
}

func TestBuildE2EWithRealFiles(t *testing.T) {
	// This test uses real file operations to verify the complete build flow
	mockEnv := NewMockProxmoxEnvironment(t)
	defer mockEnv.Cleanup()

	projectDir := filepath.Join(mockEnv.GetTempDir(), "real-files-test")
	err := os.MkdirAll(projectDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(projectDir)
	if err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}

	// Create a realistic project structure
	lxcfileContent := `from: "ubuntu:22.04"

metadata:
  name: "nodejs-app"
  description: "Node.js application container"
  version: "1.2.3"

setup:
  - run: "apt-get update && apt-get install -y nodejs npm"
  - copy:
      source: "./src"
      dest: "/opt/app"
      owner: "node:node"
      mode: "755"
  - copy:
      source: "./config"
      dest: "/etc/app"
      mode: "644"
  - env:
      NODE_ENV: "production"
      APP_PORT: "3000"
  - run: "cd /opt/app && npm install --production"
  - run: "useradd -r -s /bin/false node || true"

cleanup:
  - run: "apt-get clean"
  - run: "rm -rf /tmp/*"

resources:
  cores: 2
  memory: 1024

ports:
  - container: 3000
    host: 8080`

	err = os.WriteFile("LXCfile.yml", []byte(lxcfileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create LXCfile: %v", err)
	}

	// Create source files
	err = os.MkdirAll("src", 0755)
	if err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}

	err = os.WriteFile("src/package.json", []byte(`{
  "name": "nodejs-app",
  "version": "1.2.3",
  "main": "app.js",
  "dependencies": {
    "express": "^4.18.0",
    "dotenv": "^16.0.0"
  },
  "scripts": {
    "start": "node app.js"
  }
}`), 0644)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	err = os.WriteFile("src/app.js", []byte(`
const express = require('express');
require('dotenv').config();

const app = express();
const PORT = process.env.APP_PORT || 3000;

app.get('/', (req, res) => {
  res.json({ 
    message: 'Hello from Node.js app!',
    env: process.env.NODE_ENV,
    port: PORT
  });
});

app.get('/health', (req, res) => {
  res.json({ status: 'healthy', timestamp: new Date().toISOString() });
});

app.listen(PORT, () => {
  console.log('Server running on port', PORT);
});
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create app.js: %v", err)
	}

	// Create config files
	err = os.MkdirAll("config", 0755)
	if err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	err = os.WriteFile("config/app.conf", []byte(`# Application configuration
server_name=nodejs-app
debug=false
max_connections=100
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create app.conf: %v", err)
	}

	err = os.WriteFile("config/logging.conf", []byte(`[loggers]
keys=root

[handlers]
keys=consoleHandler

[formatters]
keys=simpleFormatter

[logger_root]
level=INFO
handlers=consoleHandler
`), 0644)
	if err != nil {
		t.Fatalf("Failed to create logging.conf: %v", err)
	}

	// Build and execute
	pxcPath := buildPxcBinary(t, mockEnv.GetTempDir())
	defer os.Remove(pxcPath)

	env := append(os.Environ(), fmt.Sprintf("PATH=%s:%s", filepath.Dir(mockEnv.GetMockPctPath()), os.Getenv("PATH")))

	cmd := exec.Command(pxcPath, "build", "--verbose", "-t", "nodejs-app:1.2.3")
	cmd.Env = env
	cmd.Dir = projectDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Build command failed: %v. Output: %s", err, string(output))
	}

	// Verify that the expected operations occurred
	executedCmds := mockEnv.GetExecutedCommands()
	
	expectedOperations := []string{
		"create", // Container creation
		"start",  // Container start
		"exec",   // Command execution
		"push",   // File copy operations
		"set",    // Configuration application
		"stop",   // Container stop
		"template", // Template creation
	}

	for _, op := range expectedOperations {
		found := false
		for _, cmd := range executedCmds {
			if strings.Contains(cmd, op) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected operation %q was not performed. Commands: %v", op, executedCmds)
		}
	}

	// Check container state progression
	states := mockEnv.GetContainerStates()
	expectedStates := []string{"created:", "started:", "stopped:", "templated:"}
	
	for _, expectedState := range expectedStates {
		found := false
		for _, state := range states {
			if strings.Contains(state, expectedState) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected state %q was not reached. States: %v", expectedState, states)
		}
	}

	// Verify output contains expected information
	outputStr := string(output)
	if !strings.Contains(outputStr, "nodejs-app:1.2.3") {
		t.Errorf("Expected template name not found in output: %s", outputStr)
	}

	t.Logf("Build completed successfully. Output: %s", outputStr)
}
package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStackE2E(t *testing.T) {
	// Setup mock environment
	mockEnv := NewMockProxmoxEnvironment(t)
	defer mockEnv.Cleanup()

	// Create test project directory
	projectDir := filepath.Join(mockEnv.GetTempDir(), "stack-test-project")
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
		stackContent   string
		buildFiles     map[string]string
		command        string
		args           []string
		expectError    bool
		errorContains  string
		mockFailures   map[string]bool
		expectedStates []string
	}{
		{
			name: "successful simple stack up",
			stackContent: `version: "1.0"
services:
  web:
    template: "nginx:latest"
    ports:
      - "80:8080"
  database:
    template: "postgres:15"
    environment:
      POSTGRES_DB: "app"
      POSTGRES_USER: "appuser"`,
			command:     "up",
			args:        []string{"--dry-run"},
			expectError: false,
		},
		{
			name: "stack with build dependencies",
			stackContent: `version: "1.0"
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
      DATABASE_HOST: "database"
    ports:
      - "80:3000"`,
			buildFiles: map[string]string{
				"web/LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update && apt-get install -y nodejs npm"
  - copy:
      source: "./src"
      dest: "/opt/app"
  - run: "cd /opt/app && npm install"
metadata:
  name: "webapp"`,
				"web/src/package.json": `{"name": "webapp", "version": "1.0.0"}`,
				"web/src/app.js": `console.log('Web app starting...');`,
			},
			command:     "up",
			args:        []string{"--dry-run"},
			expectError: false,
		},
		{
			name: "stack with complex dependencies and networks",
			stackContent: `version: "1.0"
services:
  database:
    template: "postgres:15"
    environment:
      POSTGRES_DB: "app"
      POSTGRES_USER: "dbuser"
    networks:
      - backend
    volumes:
      - "db-data:/var/lib/postgresql/data"
  
  cache:
    template: "redis:latest"
    networks:
      - backend
  
  api:
    build: "./api"
    depends_on:
      - database
      - cache
    environment:
      DATABASE_URL: "postgresql://dbuser@database/app"
      REDIS_URL: "redis://cache:6379"
    networks:
      - frontend
      - backend
    ports:
      - "3000:3000"
  
  web:
    build: "./web"
    depends_on:
      - api
    environment:
      API_URL: "http://api:3000"
    networks:
      - frontend
    ports:
      - "80:8080"

networks:
  frontend:
    driver: "bridge"
    subnet: "172.20.0.0/24"
  backend:
    driver: "bridge"
    subnet: "172.21.0.0/24"
    internal: true

volumes:
  db-data:
    driver: "local"`,
			buildFiles: map[string]string{
				"api/LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update && apt-get install -y nodejs npm"
metadata:
  name: "api-server"`,
				"web/LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update && apt-get install -y nginx"
metadata:
  name: "web-frontend"`,
			},
			command:     "up",
			args:        []string{"--dry-run"},
			expectError: false,
		},
		{
			name: "stack up with build failure",
			stackContent: `version: "1.0"
services:
  web:
    build: "./web"`,
			buildFiles: map[string]string{
				"web/LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`,
			},
			command: "up",
			args:    []string{},
			mockFailures: map[string]bool{
				"create": true,
			},
			expectError:   true,
			errorContains: "failed to create",
		},
		{
			name: "successful stack down",
			stackContent: `version: "1.0"
services:
  web:
    template: "nginx:latest"
  database:
    template: "postgres:15"
    volumes:
      - "db-data:/var/lib/postgresql/data"

volumes:
  db-data:
    driver: "local"`,
			command:     "down",
			args:        []string{"--dry-run"},
			expectError: false,
		},
		{
			name: "stack down with volume removal",
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
			command:     "down",
			args:        []string{"--volumes", "--dry-run"},
			expectError: false,
		},
		{
			name: "stack ps command",
			stackContent: `version: "1.0"
services:
  web:
    template: "nginx:latest"
    ports:
      - "80:8080"
    labels:
      environment: "production"
      tier: "frontend"
  
  api:
    template: "node:latest"
    environment:
      NODE_ENV: "production"
    labels:
      environment: "production"
      tier: "backend"
  
  database:
    template: "postgres:15"
    volumes:
      - "db-data:/var/lib/postgresql/data"
    labels:
      environment: "production"
      tier: "database"

volumes:
  db-data:
    driver: "local"`,
			command:     "ps",
			args:        []string{},
			expectError: false,
		},
		{
			name: "stack ps with filters",
			stackContent: `version: "1.0"
services:
  web:
    template: "nginx:latest"
    labels:
      tier: "frontend"
  api:
    template: "node:latest"
    labels:
      tier: "backend"`,
			command:     "ps",
			args:        []string{"--filter", "label=tier=frontend", "--format", "json"},
			expectError: false,
		},
		{
			name: "circular dependency detection",
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
			command:       "up",
			args:          []string{"--dry-run"},
			expectError:   true,
			errorContains: "circular dependency",
		},
		{
			name: "undefined service dependency",
			stackContent: `version: "1.0"
services:
  web:
    template: "nginx:latest"
    depends_on:
      - nonexistent`,
			command:       "up",
			args:          []string{"--dry-run"},
			expectError:   true,
			errorContains: "undefined service",
		},
		{
			name: "stack with scaling",
			stackContent: `version: "1.0"
services:
  web:
    template: "nginx:latest"
    scale: 3
    ports:
      - "80"
  
  worker:
    build: "./worker"
    scale: 2
    depends_on:
      - database
  
  database:
    template: "postgres:15"
    scale: 1`,
			buildFiles: map[string]string{
				"worker/LXCfile.yml": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update && apt-get install -y python3"
metadata:
  name: "worker"`,
			},
			command:     "up",
			args:        []string{"--dry-run"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up from previous test
			os.Remove("lxc-stack.yml")
			os.RemoveAll("web")
			os.RemoveAll("api")
			os.RemoveAll("worker")
			os.Remove(filepath.Join(mockEnv.GetTempDir(), "pct_commands.log"))
			os.Remove(filepath.Join(mockEnv.GetTempDir(), "container_state.log"))

			// Setup mock failures
			for cmd, shouldFail := range tt.mockFailures {
				mockEnv.SetFailure(cmd, shouldFail)
			}

			// Create stack file
			err := os.WriteFile("lxc-stack.yml", []byte(tt.stackContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create stack file: %v", err)
			}

			// Create build files
			for path, content := range tt.buildFiles {
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
			args := []string{tt.command}
			args = append(args, tt.args...)

			// Set PATH to include mock pct
			env := append(os.Environ(), fmt.Sprintf("PATH=%s:%s", filepath.Dir(mockEnv.GetMockPctPath()), os.Getenv("PATH")))

			// Execute pxc command
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
		})
	}
}

func TestFullStackLifecycle(t *testing.T) {
	// Test complete lifecycle: up -> ps -> down
	mockEnv := NewMockProxmoxEnvironment(t)
	defer mockEnv.Cleanup()

	projectDir := filepath.Join(mockEnv.GetTempDir(), "lifecycle-test")
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

	// Create a realistic multi-service stack
	stackContent := `version: "1.0"

metadata:
  name: "webapp-stack"
  description: "Complete web application stack with database and cache"

services:
  database:
    template: "postgres:15"
    environment:
      POSTGRES_DB: "webapp"
      POSTGRES_USER: "webuser"
      POSTGRES_PASSWORD: "secretpass"
    volumes:
      - "db-data:/var/lib/postgresql/data"
    networks:
      - backend
    restart: "always"
  
  cache:
    template: "redis:latest"
    command: "redis-server --appendonly yes"
    volumes:
      - "redis-data:/data"
    networks:
      - backend
    restart: "always"
  
  api:
    build: "./api"
    depends_on:
      - database
      - cache
    environment:
      NODE_ENV: "production"
      DATABASE_URL: "postgresql://webuser:secretpass@database/webapp"
      REDIS_URL: "redis://cache:6379"
      JWT_SECRET: "supersecret"
    ports:
      - "3000:3000"
    networks:
      - frontend
      - backend
    restart: "unless-stopped"
    scale: 2
  
  web:
    build: "./web"
    depends_on:
      - api
    environment:
      API_BASE_URL: "http://api:3000"
    ports:
      - "80:80"
      - "443:443"
    networks:
      - frontend
    restart: "always"
    volumes:
      - "web-assets:/var/www/html/assets"

networks:
  frontend:
    driver: "bridge"
    subnet: "172.20.0.0/24"
    gateway: "172.20.0.1"
  backend:
    driver: "bridge"
    subnet: "172.21.0.0/24"
    gateway: "172.21.0.1"
    internal: true

volumes:
  db-data:
    driver: "local"
    labels:
      backup: "daily"
  redis-data:
    driver: "local"
  web-assets:
    driver: "local"`

	err = os.WriteFile("lxc-stack.yml", []byte(stackContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create stack file: %v", err)
	}

	// Create API service
	err = os.MkdirAll("api/src", 0755)
	if err != nil {
		t.Fatalf("Failed to create api directory: %v", err)
	}

	apiLXCfile := `from: "ubuntu:22.04"

metadata:
  name: "webapp-api"
  description: "Web application API server"
  version: "1.0.0"

setup:
  - run: "apt-get update && apt-get install -y nodejs npm postgresql-client redis-tools"
  - copy:
      source: "./src"
      dest: "/opt/api"
      owner: "node:node"
      mode: "755"
  - run: "cd /opt/api && npm install --production"
  - run: "useradd -r -s /bin/false node || true"
  - env:
      NODE_ENV: "production"
      PORT: "3000"

cleanup:
  - run: "apt-get clean && rm -rf /var/lib/apt/lists/*"

health:
  test: "curl -f http://localhost:3000/health || exit 1"
  interval: "30s"
  timeout: "5s"
  retries: 3

ports:
  - container: 3000
    protocol: "tcp"`

	err = os.WriteFile("api/LXCfile.yml", []byte(apiLXCfile), 0644)
	if err != nil {
		t.Fatalf("Failed to create API LXCfile: %v", err)
	}

	apiPackageJSON := `{
  "name": "webapp-api",
  "version": "1.0.0",
  "main": "server.js",
  "dependencies": {
    "express": "^4.18.0",
    "pg": "^8.8.0",
    "redis": "^4.3.0",
    "jsonwebtoken": "^8.5.0",
    "bcrypt": "^5.1.0",
    "dotenv": "^16.0.0"
  },
  "scripts": {
    "start": "node server.js"
  }
}`

	err = os.WriteFile("api/src/package.json", []byte(apiPackageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create API package.json: %v", err)
	}

	apiServer := `const express = require('express');
const { Client } = require('pg');
const redis = require('redis');
require('dotenv').config();

const app = express();
const PORT = process.env.PORT || 3000;

app.use(express.json());

// Health check endpoint
app.get('/health', (req, res) => {
  res.json({ 
    status: 'healthy', 
    timestamp: new Date().toISOString(),
    version: '1.0.0'
  });
});

// API routes
app.get('/api/status', (req, res) => {
  res.json({ 
    message: 'API is running',
    env: process.env.NODE_ENV 
  });
});

app.listen(PORT, () => {
  console.log('API server running on port', PORT);
});`

	err = os.WriteFile("api/src/server.js", []byte(apiServer), 0644)
	if err != nil {
		t.Fatalf("Failed to create API server.js: %v", err)
	}

	// Create Web service
	err = os.MkdirAll("web/src", 0755)
	if err != nil {
		t.Fatalf("Failed to create web directory: %v", err)
	}

	webLXCfile := `from: "ubuntu:22.04"

metadata:
  name: "webapp-frontend"
  description: "Web application frontend"
  version: "1.0.0"

setup:
  - run: "apt-get update && apt-get install -y nginx nodejs npm"
  - copy:
      source: "./src"
      dest: "/var/www/html"
      owner: "www-data:www-data"
      mode: "755"
  - copy:
      source: "./nginx.conf"
      dest: "/etc/nginx/sites-available/default"
      mode: "644"
  - run: "cd /var/www/html && npm install && npm run build"

cleanup:
  - run: "apt-get clean && rm -rf /var/lib/apt/lists/*"

health:
  test: "curl -f http://localhost || exit 1"
  interval: "30s"
  timeout: "5s"
  retries: 3

ports:
  - container: 80
    protocol: "tcp"
  - container: 443
    protocol: "tcp"`

	err = os.WriteFile("web/LXCfile.yml", []byte(webLXCfile), 0644)
	if err != nil {
		t.Fatalf("Failed to create Web LXCfile: %v", err)
	}

	webPackageJSON := `{
  "name": "webapp-frontend",
  "version": "1.0.0",
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "scripts": {
    "build": "echo 'Building frontend...' && mkdir -p build"
  }
}`

	err = os.WriteFile("web/src/package.json", []byte(webPackageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create Web package.json: %v", err)
	}

	nginxConf := `server {
    listen 80;
    server_name localhost;

    location / {
        root /var/www/html/build;
        index index.html;
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://api:3000/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}`

	err = os.WriteFile("web/nginx.conf", []byte(nginxConf), 0644)
	if err != nil {
		t.Fatalf("Failed to create nginx.conf: %v", err)
	}

	err = os.WriteFile("web/src/index.html", []byte(`<!DOCTYPE html>
<html>
<head>
    <title>Web App</title>
</head>
<body>
    <h1>Welcome to Web App</h1>
    <p>Full stack application with API and database.</p>
</body>
</html>`), 0644)
	if err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	// Build pxc binary
	pxcPath := buildPxcBinary(t, mockEnv.GetTempDir())
	defer os.Remove(pxcPath)

	env := append(os.Environ(), fmt.Sprintf("PATH=%s:%s", filepath.Dir(mockEnv.GetMockPctPath()), os.Getenv("PATH")))

	// Test 1: Stack up
	t.Run("stack_up", func(t *testing.T) {
		cmd := exec.Command(pxcPath, "up", "--dry-run", "--verbose")
		cmd.Env = env
		cmd.Dir = projectDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Stack up failed: %v. Output: %s", err, string(output))
		}

		// Verify output contains expected information
		outputStr := string(output)
		if !strings.Contains(outputStr, "database") || !strings.Contains(outputStr, "api") || !strings.Contains(outputStr, "web") {
			t.Errorf("Expected services not found in output: %s", outputStr)
		}
	})

	// Test 2: Stack ps
	t.Run("stack_ps", func(t *testing.T) {
		cmd := exec.Command(pxcPath, "ps", "--format", "wide")
		cmd.Env = env
		cmd.Dir = projectDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Stack ps failed: %v. Output: %s", err, string(output))
		}

		t.Logf("PS output: %s", string(output))
	})

	// Test 3: Stack ps with filters
	t.Run("stack_ps_filtered", func(t *testing.T) {
		cmd := exec.Command(pxcPath, "ps", "--filter", "service=api", "--format", "json")
		cmd.Env = env
		cmd.Dir = projectDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Stack ps with filter failed: %v. Output: %s", err, string(output))
		}

		t.Logf("Filtered PS output: %s", string(output))
	})

	// Test 4: Stack down (preserve volumes)
	t.Run("stack_down_preserve_volumes", func(t *testing.T) {
		cmd := exec.Command(pxcPath, "down", "--dry-run", "--verbose")
		cmd.Env = env
		cmd.Dir = projectDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Stack down failed: %v. Output: %s", err, string(output))
		}

		// Should mention preserving volumes
		outputStr := string(output)
		if !strings.Contains(outputStr, "volume") {
			t.Logf("Note: Volume preservation not explicitly mentioned in output: %s", outputStr)
		}
	})

	// Test 5: Stack down (remove volumes)
	t.Run("stack_down_remove_volumes", func(t *testing.T) {
		cmd := exec.Command(pxcPath, "down", "--volumes", "--dry-run", "--verbose")
		cmd.Env = env
		cmd.Dir = projectDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Stack down with volumes failed: %v. Output: %s", err, string(output))
		}

		t.Logf("Down with volumes output: %s", string(output))
	})
}
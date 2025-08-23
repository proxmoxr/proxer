package config

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkLoadLXCfile(b *testing.B) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "pxc-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save original directory
	originalDir, err := os.Getwd()
	if err != nil {
		b.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		b.Fatalf("Failed to change to temp directory: %v", err)
	}

	benchmarks := []struct {
		name    string
		content string
	}{
		{
			name: "simple LXCfile",
			content: `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`,
		},
		{
			name: "complex LXCfile",
			content: `from: "ubuntu:22.04"

metadata:
  name: "webapp"
  description: "Web application container"
  version: "1.0.0"
  author: "developer@example.com"

features:
  unprivileged: true
  nesting: true
  keyctl: false
  fuse: true

resources:
  cores: 4
  memory: 2048
  swap: 1024

security:
  isolation: "strict"
  apparmor: true
  seccomp: true

setup:
  - run: "apt-get update && apt-get install -y nodejs npm nginx postgresql-client"
  - copy:
      source: "./app"
      dest: "/opt/app"
      owner: "www-data:www-data"
      mode: "755"
  - copy:
      source: "./config"
      dest: "/etc/app"
      mode: "644"
  - env:
      NODE_ENV: "production"
      PORT: "3000"
      DATABASE_URL: "postgresql://user:pass@localhost/app"
      REDIS_URL: "redis://localhost:6379"
  - run: "cd /opt/app && npm install --production"
  - run: "systemctl enable nginx"

cleanup:
  - run: "apt-get clean"
  - run: "rm -rf /var/lib/apt/lists/*"
  - run: "rm -rf /tmp/*"

ports:
  - container: 3000
    host: 8080
    protocol: "tcp"
  - container: 80
    host: 8081
    protocol: "tcp"
  - container: 443
    host: 8443
    protocol: "tcp"

mounts:
  - source: "/host/data"
    target: "/var/lib/app/data"
    type: "bind"
    readonly: false
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
  maintainer: "developer@example.com"
  tier: "backend"`,
		},
		{
			name: "large LXCfile with many steps",
			content: generateLargeLXCfile(50), // 50 setup steps
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Create test file
			testFile := filepath.Join(tempDir, "LXCfile.yml")
			err := os.WriteFile(testFile, []byte(bm.content), 0644)
			if err != nil {
				b.Fatalf("Failed to write test file: %v", err)
			}

			// Reset timer to exclude setup
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := LoadLXCfile(testFile)
				if err != nil {
					b.Fatalf("LoadLXCfile failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkLoadLXCStack(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "pxc-stack-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	if err != nil {
		b.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		b.Fatalf("Failed to change to temp directory: %v", err)
	}

	benchmarks := []struct {
		name    string
		content string
	}{
		{
			name: "simple stack",
			content: `version: "1.0"
services:
  web:
    template: "nginx:latest"`,
		},
		{
			name: "complex stack",
			content: generateLargeStack(10, 5, 3), // 10 services, 5 networks, 3 volumes
		},
		{
			name: "large stack",
			content: generateLargeStack(50, 10, 10), // 50 services, 10 networks, 10 volumes
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			testFile := filepath.Join(tempDir, "lxc-stack.yml")
			err := os.WriteFile(testFile, []byte(bm.content), 0644)
			if err != nil {
				b.Fatalf("Failed to write test file: %v", err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := LoadLXCStack(testFile)
				if err != nil {
					b.Fatalf("LoadLXCStack failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkLXCfileValidation(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "pxc-validation-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	if err != nil {
		b.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		b.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Load test files of different complexities
	testFiles := map[string]string{
		"simple": `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`,
		"complex": generateLargeLXCfile(20),
		"large":   generateLargeLXCfile(100),
	}

	lxcfiles := make(map[string]string)
	for name, content := range testFiles {
		testFile := filepath.Join(tempDir, name+".yml")
		err := os.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			b.Fatalf("Failed to write test file: %v", err)
		}
		lxcfiles[name] = testFile
	}

	for name, testFile := range lxcfiles {
		b.Run(name, func(b *testing.B) {
			// Load once outside the benchmark
			lxcfile, err := LoadLXCfile(testFile)
			if err != nil {
				b.Fatalf("Failed to load LXCfile: %v", err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				err := lxcfile.Validate()
				if err != nil {
					b.Fatalf("Validation failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkStackValidation(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "pxc-stack-validation-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	if err != nil {
		b.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		b.Fatalf("Failed to change to temp directory: %v", err)
	}

	testStacks := map[string]string{
		"simple": `version: "1.0"
services:
  web:
    template: "nginx:latest"`,
		"complex": generateLargeStack(5, 3, 2),
		"large":   generateLargeStack(25, 5, 5),
	}

	stacks := make(map[string]string)
	for name, content := range testStacks {
		testFile := filepath.Join(tempDir, name+".yml")
		err := os.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			b.Fatalf("Failed to write test file: %v", err)
		}
		stacks[name] = testFile
	}

	for name, testFile := range stacks {
		b.Run(name, func(b *testing.B) {
			stack, err := LoadLXCStack(testFile)
			if err != nil {
				b.Fatalf("Failed to load stack: %v", err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				err := stack.Validate()
				if err != nil {
					b.Fatalf("Validation failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkGetDefaultLXCfile(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "pxc-default-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	if err != nil {
		b.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		b.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create test file
	err = os.WriteFile("LXCfile.yml", []byte("test"), 0644)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = GetDefaultLXCfile()
	}
}

// Helper functions to generate large test files
func generateLargeLXCfile(numSteps int) string {
	content := `from: "ubuntu:22.04"

metadata:
  name: "large-app"
  description: "Large application for benchmarking"
  version: "1.0.0"

setup:`

	for i := 0; i < numSteps; i++ {
		content += `
  - run: "echo 'Step ` + string(rune('0'+i%10)) + `' && sleep 0.1"`
	}

	content += `

cleanup:
  - run: "apt-get clean"
  - run: "rm -rf /tmp/*"

resources:
  cores: 2
  memory: 1024

ports:
  - container: 3000
    host: 8080`

	return content
}

func generateLargeStack(numServices, numNetworks, numVolumes int) string {
	content := `version: "1.0"

services:`

	for i := 0; i < numServices; i++ {
		content += `
  service` + string(rune('0'+i%10)) + `:
    template: "alpine:latest"
    environment:
      SERVICE_ID: "` + string(rune('0'+i%10)) + `"`

		if i > 0 && i%3 == 0 {
			content += `
    depends_on:
      - service` + string(rune('0'+(i-1)%10)) + ``
		}

		if i%2 == 0 {
			content += `
    networks:
      - net` + string(rune('0'+i%numNetworks)) + ``
		}
	}

	if numNetworks > 0 {
		content += `

networks:`
		for i := 0; i < numNetworks; i++ {
			content += `
  net` + string(rune('0'+i)) + `:
    driver: "bridge"
    subnet: "172.` + string(rune('2'+i/10)) + string(rune('0'+i%10)) + `.0.0/24"`
		}
	}

	if numVolumes > 0 {
		content += `

volumes:`
		for i := 0; i < numVolumes; i++ {
			content += `
  vol` + string(rune('0'+i)) + `:
    driver: "local"`
		}
	}

	return content
}

func BenchmarkFileResolution(b *testing.B) {
	// Benchmark the file resolution logic with different directory structures
	tempDir, err := os.MkdirTemp("", "pxc-resolution-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	if err != nil {
		b.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	scenarios := []struct {
		name  string
		setup func(string) error
	}{
		{
			name: "file exists",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "LXCfile.yml"), []byte("test"), 0644)
			},
		},
		{
			name: "file doesn't exist",
			setup: func(dir string) error {
				return nil // Don't create the file
			},
		},
		{
			name: "multiple candidates",
			setup: func(dir string) error {
				os.WriteFile(filepath.Join(dir, "LXCfile.yml"), []byte("test"), 0644)
				os.WriteFile(filepath.Join(dir, "LXCfile.yaml"), []byte("test"), 0644)
				return os.WriteFile(filepath.Join(dir, "lxcfile.yml"), []byte("test"), 0644)
			},
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			testDir := filepath.Join(tempDir, scenario.name)
			err := os.MkdirAll(testDir, 0755)
			if err != nil {
				b.Fatalf("Failed to create test dir: %v", err)
			}

			err = scenario.setup(testDir)
			if err != nil {
				b.Fatalf("Failed to setup scenario: %v", err)
			}

			err = os.Chdir(testDir)
			if err != nil {
				b.Fatalf("Failed to change to test dir: %v", err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = GetDefaultLXCfile()
			}
		})
	}
}
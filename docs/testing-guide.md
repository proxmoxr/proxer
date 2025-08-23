# Testing Guide for Proxer

This document provides comprehensive guidance on testing Proxer, including test types, running tests, writing new tests, and best practices.

## Table of Contents

- [Overview](#overview)
- [Test Structure](#test-structure)
- [Running Tests](#running-tests)
- [Test Types](#test-types)
- [Writing Tests](#writing-tests)
- [Performance Testing](#performance-testing)
- [CI/CD Integration](#cicd-integration)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

Proxer uses a comprehensive testing strategy that includes:

- **Unit Tests**: Test individual components in isolation
- **Integration Tests**: Test CLI commands and component interactions
- **End-to-End Tests**: Test complete workflows with mock Proxmox environment
- **Performance Tests**: Benchmark critical operations
- **Security Tests**: Vulnerability scanning and security analysis

## Test Structure

```
proxer/
├── internal/
│   ├── cmd/
│   │   ├── build_test.go       # CLI command integration tests
│   │   ├── up_test.go
│   │   ├── down_test.go
│   │   └── ps_test.go
│   └── models/
│       ├── lxcfile_test.go     # Unit tests for LXCfile validation
│       ├── stack_test.go       # Unit tests for stack validation
│       └── validation_bench_test.go  # Performance benchmarks
├── pkg/
│   └── config/
│       ├── loader_test.go      # Unit tests for configuration loading
│       └── loader_bench_test.go # Performance benchmarks
├── test/
│   ├── e2e/
│   │   ├── build_e2e_test.go   # End-to-end build tests
│   │   └── stack_e2e_test.go   # End-to-end stack tests
│   ├── coverage/               # Coverage reports
│   ├── reports/                # Test reports
│   └── run_tests.sh           # Test runner script
├── .github/
│   └── workflows/
│       ├── test.yml           # CI test workflow
│       └── release.yml        # Release workflow
└── Makefile                   # Build and test targets
```

## Running Tests

### Quick Start

```bash
# Run all tests
make test-all

# Run specific test types
make test           # Unit tests only
make test-integration  # Integration tests only
make test-e2e      # End-to-end tests only

# Run with coverage
make coverage
```

### Using the Test Runner Script

```bash
# Run all tests with verbose output
./test/run_tests.sh all -v

# Run specific test suites
./test/run_tests.sh unit -v
./test/run_tests.sh integration -v
./test/run_tests.sh e2e -v

# Run CI test suite (recommended for development)
./test/run_tests.sh ci -v

# Run performance tests
./test/run_tests.sh benchmark -v

# Run security scans
./test/run_tests.sh security -v
```

### Direct Go Commands

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -race -coverprofile=coverage.txt -covermode=atomic ./...

# Run specific package tests
go test ./internal/models
go test ./pkg/config

# Run benchmarks
go test -bench=. -benchmem ./...

# Run tests with verbose output
go test -v ./...
```

## Test Types

### Unit Tests

**Location**: `internal/models/*_test.go`, `pkg/config/*_test.go`

**Purpose**: Test individual functions and methods in isolation

**Examples**:
- LXCfile validation logic
- Stack configuration parsing
- Build configuration processing
- Template name generation

```bash
# Run unit tests
make test
./test/run_tests.sh unit
```

### Integration Tests

**Location**: `internal/cmd/*_test.go`

**Purpose**: Test CLI command functionality and component interactions

**Features**:
- Command-line argument parsing
- Flag validation
- Configuration loading
- Error handling
- Dry-run functionality

```bash
# Run integration tests
make test-integration
./test/run_tests.sh integration
```

### End-to-End Tests

**Location**: `test/e2e/*_test.go`

**Purpose**: Test complete workflows with mock Proxmox environment

**Features**:
- Mock `pct` command interactions
- Complete build and deployment workflows
- Multi-service stack orchestration
- Container lifecycle management
- Real file operations

```bash
# Run end-to-end tests
make test-e2e
./test/run_tests.sh e2e
```

### Performance Tests

**Location**: `*_bench_test.go` files

**Purpose**: Benchmark critical operations for performance regression detection

**Metrics**:
- YAML parsing performance
- Validation speed
- Configuration loading time
- Memory usage patterns

```bash
# Run performance tests
make benchmark
./test/run_tests.sh benchmark
```

## Writing Tests

### Unit Test Example

```go
func TestLXCfileValidation(t *testing.T) {
    tests := []struct {
        name     string
        lxcfile  LXCfile
        wantErr  bool
        errorMsg string
    }{
        {
            name: "valid minimal LXCfile",
            lxcfile: LXCfile{
                From: "ubuntu:22.04",
                Setup: []SetupStep{
                    {Run: "apt-get update"},
                },
            },
            wantErr: false,
        },
        {
            name: "missing from field",
            lxcfile: LXCfile{
                Setup: []SetupStep{
                    {Run: "apt-get update"},
                },
            },
            wantErr:  true,
            errorMsg: "'from' field is required",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.lxcfile.Validate()
            if tt.wantErr {
                if err == nil {
                    t.Errorf("Expected error but got none")
                    return
                }
                if tt.errorMsg != "" && err.Error() != tt.errorMsg {
                    t.Errorf("Expected error %q, got %q", tt.errorMsg, err.Error())
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error but got: %v", err)
                }
            }
        })
    }
}
```

### Integration Test Example

```go
func TestBuildCommand(t *testing.T) {
    // Create temporary directory for test files
    tempDir, err := os.MkdirTemp("", "pxc-test-*")
    if err != nil {
        t.Fatalf("Failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tempDir)

    // Change to test directory
    originalDir, _ := os.Getwd()
    defer os.Chdir(originalDir)
    os.Chdir(tempDir)

    // Create test LXCfile
    lxcfileContent := `from: "ubuntu:22.04"
setup:
  - run: "apt-get update"`
    
    err = os.WriteFile("LXCfile.yml", []byte(lxcfileContent), 0644)
    if err != nil {
        t.Fatalf("Failed to create LXCfile: %v", err)
    }

    // Create and execute command
    cmd := &cobra.Command{Use: "build", RunE: runBuild}
    cmd.Flags().Bool("dry-run", false, "Dry run")
    cmd.SetArgs([]string{"--dry-run"})

    err = cmd.Execute()
    if err != nil {
        t.Errorf("Build command failed: %v", err)
    }
}
```

### End-to-End Test Structure

```go
func TestBuildE2E(t *testing.T) {
    // Setup mock Proxmox environment
    mockEnv := NewMockProxmoxEnvironment(t)
    defer mockEnv.Cleanup()

    // Create project structure
    setupProjectFiles(t, mockEnv.GetTempDir())

    // Build the pxc binary
    pxcPath := buildPxcBinary(t, mockEnv.GetTempDir())

    // Execute command with mock environment
    cmd := exec.Command(pxcPath, "build", "--verbose")
    cmd.Env = append(os.Environ(), 
        fmt.Sprintf("PATH=%s:%s", 
            filepath.Dir(mockEnv.GetMockPctPath()), 
            os.Getenv("PATH")))

    output, err := cmd.CombinedOutput()
    
    // Verify results
    if err != nil {
        t.Fatalf("Command failed: %v\nOutput: %s", err, output)
    }

    // Check that expected operations occurred
    commands := mockEnv.GetExecutedCommands()
    // ... verify commands
}
```

### Benchmark Test Example

```go
func BenchmarkLXCfileValidation(b *testing.B) {
    lxcfile := &LXCfile{
        From: "ubuntu:22.04",
        Setup: []SetupStep{
            {Run: "apt-get update"},
        },
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        err := lxcfile.Validate()
        if err != nil {
            b.Fatalf("Validation failed: %v", err)
        }
    }
}
```

## Performance Testing

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. ./...

# Run specific benchmarks
go test -bench=BenchmarkLXCfileValidation ./internal/models

# Run benchmarks with memory profiling
go test -bench=. -benchmem ./...

# Run benchmarks multiple times for stability
go test -bench=. -count=5 ./...
```

### Analyzing Results

```bash
# Compare benchmark results
go test -bench=. ./... > bench_old.txt
# Make changes
go test -bench=. ./... > bench_new.txt
benchcmp bench_old.txt bench_new.txt
```

### Performance Thresholds

| Operation | Target Performance |
|-----------|-------------------|
| LXCfile validation | < 1ms for typical files |
| Stack validation | < 5ms for 10 services |
| YAML parsing | < 2ms for complex files |
| Configuration loading | < 1ms |

## CI/CD Integration

### GitHub Actions Workflow

The project includes comprehensive CI/CD workflows:

**Test Workflow** (`.github/workflows/test.yml`):
- Runs on every push and pull request
- Executes unit, integration, and e2e tests
- Generates coverage reports
- Performs security scans
- Tests on multiple Go versions

**Release Workflow** (`.github/workflows/release.yml`):
- Triggered by version tags
- Runs full test suite before release
- Builds binaries for multiple platforms
- Creates GitHub releases with artifacts

### Running CI Tests Locally

```bash
# Run the same tests as CI
./test/run_tests.sh ci -v

# Or using Make
make test-ci
```

### Coverage Requirements

- **Minimum Coverage**: 80%
- **Critical Packages**: 90%+ coverage required
  - `internal/models`
  - `pkg/config`
  - `pkg/builder` (when implemented)

## Best Practices

### Test Organization

1. **Use table-driven tests** for multiple scenarios
2. **Group related tests** in the same file
3. **Use descriptive test names** that explain the scenario
4. **Test both success and failure cases**
5. **Include edge cases** and boundary conditions

### Test Data Management

1. **Use temporary directories** for file operations
2. **Clean up resources** in defer statements
3. **Use realistic test data** that mirrors real usage
4. **Avoid hardcoded paths** or system dependencies

### Mock and Fixture Usage

1. **Mock external dependencies** (Proxmox commands)
2. **Use consistent test fixtures** across test suites
3. **Make mocks configurable** for different scenarios
4. **Document mock behavior** and limitations

### Performance Testing

1. **Reset timers** before benchmark loops
2. **Use realistic data sizes** for benchmarks
3. **Test with different complexity levels**
4. **Monitor memory allocations** with `-benchmem`

### Error Testing

1. **Test all error conditions**
2. **Verify error messages** are helpful
3. **Test error propagation** through layers
4. **Include validation error scenarios**

## Troubleshooting

### Common Issues

**Tests fail with "command not found: pct"**
- End-to-end tests use mock `pct` commands
- Ensure the test runner creates the mock environment properly

**Coverage reports are incomplete**
- Run tests from the project root directory
- Ensure all packages are included: `go test ./...`

**Benchmark results vary significantly**
- Run benchmarks multiple times: `go test -bench=. -count=5`
- Ensure system is not under load during benchmarking

**Integration tests timeout**
- Increase test timeout: `go test -timeout=10m`
- Check for deadlocks or infinite loops

### Debugging Tests

```bash
# Run specific test with verbose output
go test -v -run TestSpecificFunction ./path/to/package

# Run tests with race detection
go test -race ./...

# Run tests with CPU profiling
go test -cpuprofile=cpu.prof -bench=.

# Run tests with memory profiling
go test -memprofile=mem.prof -bench=.
```

### Test Environment

**Required Tools**:
- Go 1.20+
- Make (optional, for Makefile targets)
- golangci-lint (for linting)
- gosec (for security scanning)
- govulncheck (for vulnerability scanning)

**Installation**:
```bash
# Install development dependencies
make deps-dev

# Or install individually
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

## Contributing Tests

When adding new features or fixing bugs:

1. **Write tests first** (TDD approach)
2. **Ensure adequate coverage** for new code
3. **Update existing tests** if behavior changes
4. **Add integration tests** for new CLI features
5. **Include benchmark tests** for performance-critical code
6. **Document test scenarios** in pull requests

### Test Review Checklist

- [ ] Tests cover both success and failure cases
- [ ] Test names clearly describe the scenario
- [ ] Tests are deterministic and don't depend on external state
- [ ] Resource cleanup is handled properly
- [ ] Tests run quickly (unit tests < 1s, integration tests < 10s)
- [ ] Coverage meets minimum requirements
- [ ] Benchmarks are included for performance-critical code

---

For questions about testing or contributions, please refer to the main project documentation or open an issue on GitHub.
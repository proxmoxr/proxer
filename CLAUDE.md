# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Proxer is a Docker-like CLI tool for Proxmox LXC containers. It provides familiar commands like `pxc build`, `pxc up`, `pxc down`, and `pxc ps` to build reusable LXC templates and orchestrate multi-container applications using declarative YAML configuration files.

The project bridges the gap between Docker's developer experience and Proxmox's LXC containers, enabling Docker-style workflows while leveraging LXC's performance benefits and tight Proxmox integration.

## Essential Development Commands

### Building
```bash
# Build the binary
go build -o pxc ./cmd/pxc

# Build for Linux (typical deployment target)
GOOS=linux GOARCH=amd64 go build -o pxc-linux ./cmd/pxc

# Test build and basic functionality
./pxc --help
./pxc version
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -race -coverprofile=coverage.txt -covermode=atomic ./...

# Run specific package tests
go test ./pkg/builder
go test ./internal/models
```

### Linting
```bash
# Install golangci-lint if needed
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter (configured in .golangci.yml)
golangci-lint run --timeout=5m

# Auto-fix formatting issues
go fmt ./...
goimports -w .
```

### Development Testing
```bash
# Test with dry-run (safe, doesn't create containers)
./pxc build --dry-run --verbose -f examples/simple-app/LXCfile.yml

# Test configuration loading
echo "storage: local-lvm" > .pxc.yaml
./pxc build --dry-run --verbose
```

## Core Architecture

### Configuration System
The project uses a hierarchical configuration approach:
1. **Built-in defaults** in `pkg/builder/builder.go` (storage: "local-lvm", template_storage: "local")
2. **Config file overrides** via Viper (`.pxc.yaml`, `~/.pxc.yaml`)
3. **Command-line flags** take highest precedence

Configuration loading happens in `internal/cmd/root.go` using Viper, then gets passed to builder and orchestrator components.

### Data Flow Architecture

**Build Process (`pxc build`):**
1. `internal/cmd/build.go` → Parse LXCfile.yml via `pkg/config/loader.go`
2. Create `pkg/builder.Builder` with config from Viper
3. Builder executes: create temp container → run setup steps → apply config → export template → cleanup
4. Uses `pct` commands directly via `exec.Command()`

**Orchestration Process (`pxc up`):**
1. `internal/cmd/up.go` → Parse lxc-stack.yml via `pkg/config/loader.go`
2. Create `pkg/runner.Orchestrator` with builder and Proxmox client
3. Resolve service dependencies → build templates → create containers → start in order

### Key Components

**Models (`internal/models/`):**
- `lxcfile.go`: Complete LXCfile.yml schema with validation tags
- `stack.go`: lxc-stack.yml schema for multi-container orchestration

**Builder (`pkg/builder/`):**
- Core template building logic using native `pct` commands
- Handles container lifecycle: create → configure → export → cleanup
- Supports build arguments, resource limits, security settings

**Orchestrator (`pkg/runner/`):**
- Multi-container deployment management
- Dependency resolution and ordered startup
- Network and volume management
- Integrates with Builder for template creation

**Configuration (`pkg/config/`):**
- YAML parsing and validation for both LXCfile.yml and lxc-stack.yml
- Helpful error messages with file suggestions
- Schema validation with detailed error reporting

### Proxmox Integration

The tool integrates with Proxmox by executing native `pct` (Proxmox Container Toolkit) commands rather than using APIs. This approach:
- Leverages existing Proxmox permissions and authentication
- Ensures compatibility with all Proxmox versions
- Provides familiar command patterns for Proxmox administrators

**Critical**: All `pct` commands are executed via `exec.Command()` in `pkg/builder/builder.go`. The tool expects to run on Proxmox hosts with `pct` available in PATH.

## Configuration Schema

### LXCfile.yml Structure
- **from**: Base template (required) - supports full storage paths like "cephfs:vztmpl/ubuntu-22.04.tar.zst"
- **setup**: Build steps (required) - run commands, copy files, set environment variables
- **resources**: CPU, memory, swap limits
- **features**: LXC-specific features (nesting, keyctl, fuse)
- **security**: Isolation, capabilities, AppArmor settings
- **ports**: Port mapping configuration
- **health**: Health check configuration

### lxc-stack.yml Structure
- **services**: Container definitions with build or template sources
- **networks**: Bridge network configuration
- **volumes**: Persistent storage definitions
- **dependencies**: Service startup ordering via depends_on

## Storage Configuration

**Critical for Proxmox deployments**: Storage configuration varies significantly between Proxmox installations. The tool uses these defaults:
- Container storage: `local-lvm` (most common LVM thin pool)
- Template storage: `local` (for saving built templates)

Override via configuration file for different storage backends (ZFS, Ceph, NFS):
```yaml
storage: "data2"              # For container rootfs
template_storage: "cephfs"    # For saving templates
proxmox_node: "pveserver1"    # Target Proxmox node
```

## Testing on Proxmox

When developing features that interact with Proxmox:
1. Use `--dry-run` flag extensively to validate logic without creating containers
2. Test with different storage backends (local-lvm, NFS, Ceph)
3. Verify template paths work with different storage types
4. Test with both privileged and unprivileged containers

## Common Gotchas

1. **Storage Compatibility**: Not all Proxmox storage supports container directories. `local` storage often only supports templates/ISOs, not container rootfs.

2. **Template Path Format**: Base templates must include full storage path for non-local storage: `"cephfs:vztmpl/ubuntu-22.04.tar.zst"` not just `"ubuntu-22.04"`

3. **Configuration Loading**: Viper configuration must be explicitly passed to builder/orchestrator. The pattern is: `viper.GetString("storage")` in command files.

4. **Container ID Generation**: Uses timestamp-based IDs to avoid conflicts. Production deployments should check existing container IDs.

5. **Error Handling**: Always provide helpful error messages pointing to configuration issues, missing templates, or storage problems.

## Release Process

Tags trigger GitHub Actions to build multi-platform binaries. The CI/CD pipeline:
1. Runs tests and linting
2. Builds binaries for Linux/macOS (amd64/arm64)
3. Runs security scanning (gosec, govulncheck)
4. Creates GitHub releases with downloadable binaries

Critical: Ensure `cmd/pxc/main.go` is committed (was previously ignored by overly broad .gitignore pattern).
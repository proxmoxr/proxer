# CLI Reference

Complete command-line interface reference for pxc (Proxmox Container eXecutor).

## Global Options

These options are available for all commands:

### Configuration
- **`--config <file>`** - Specify config file (default: `./.pxc.yaml` or `$HOME/.pxc.yaml`)
- **`--verbose, -v`** - Enable verbose output with detailed operation logging
- **`--dry-run`** - Show what would be done without executing any changes

### Help and Version
- **`--help, -h`** - Show help for any command
- **`pxc version`** - Show version information (use `--verbose` for build details)

## Environment Variables

pxc respects these environment variables for configuration:

### Storage Configuration
- **`PXC_STORAGE`** - Override default container storage backend (default: `local-lvm`)
- **`PXC_TEMPLATE_STORAGE`** - Override template storage location (default: `local`)
- **`PXC_PROXMOX_NODE`** - Override target Proxmox node (default: current node)

### Runtime Configuration  
- **`PXC_CONFIG`** - Override config file location
- **`PXC_VERBOSE`** - Enable verbose mode (`true`/`false`)
- **`PXC_DRY_RUN`** - Enable dry-run mode (`true`/`false`)

### Build Configuration
- **`PXC_TEMP_PREFIX`** - Prefix for temporary build containers (default: `pxc-build-`)
- **`PXC_BUILD_TIMEOUT`** - Timeout for build operations in seconds (default: `300`)

### Example Usage
```bash
# Use different storage for high-performance builds
PXC_STORAGE=fast-ssd PXC_TEMPLATE_STORAGE=fast-ssd pxc build -t webapp:1.0

# Target specific node in cluster
PXC_PROXMOX_NODE=pve-node-2 pxc up

# Enable verbose mode globally
export PXC_VERBOSE=true
pxc build
pxc up
```

## Exit Codes

pxc uses consistent exit codes across all commands:

- **`0`** - Success - Command completed successfully
- **`1`** - General Error - Configuration issues, validation failures, user errors
- **`2`** - Execution Error - Command execution failed (Proxmox/system errors)
- **`3`** - Resource Error - Resource allocation failed (insufficient memory, storage, etc.)

### Exit Code Examples
```bash
# Check if build succeeded
pxc build -t myapp:1.0
if [ $? -eq 0 ]; then
    echo "Build successful"
else
    echo "Build failed with code $?"
fi

# Handle different error types
pxc up
case $? in
    0) echo "Stack deployed successfully" ;;
    1) echo "Configuration error - check lxc-stack.yml" ;;
    2) echo "Deployment failed - check Proxmox status" ;;
    3) echo "Insufficient resources - check node capacity" ;;
esac
```

## Command Reference

### pxc build

Build LXC container templates from LXCfile.yml.

**Usage:** `pxc build [OPTIONS]`

**Options:**
- **`-f, --file <file>`** - Path to LXCfile (default: `LXCfile.yml`)
- **`-t, --tag <name:version>`** - Template name and optional version tag
- **`--build-arg <key=value>`** - Set build-time variables (can specify multiple)

**Examples:**
```bash
# Basic build from LXCfile.yml
pxc build

# Build with custom file and tag
pxc build -f custom.yml -t webapp:2.0

# Build with multiple build arguments
pxc build --build-arg NODE_ENV=production --build-arg VERSION=1.2.3

# Dry run to validate before building
pxc build --dry-run --verbose
```

### pxc up

Deploy multi-container applications from lxc-stack.yml.

**Usage:** `pxc up [OPTIONS] [SERVICE...]`

**Options:**
- **`-f, --file <file>`** - Path to stack file (default: `lxc-stack.yml`)
- **`--project-name <name>`** - Project name for isolation (default: directory name)
- **`-d, --detach`** - Run containers in background (detached mode)
- **`--build <services>`** - Build only specified services (comma-separated)
- **`--build-arg <key=value>`** - Set build-time variables for all services

**Examples:**
```bash
# Deploy all services from lxc-stack.yml
pxc up

# Deploy specific services only
pxc up web database

# Deploy in background with custom project name
pxc up --detach --project-name myapp-prod

# Force rebuild of specific services
pxc up --build web,api --build-arg VERSION=1.2.3
```

### pxc down

Stop and remove containers, networks, and volumes.

**Usage:** `pxc down [OPTIONS]`

**Options:**
- **`-f, --file <file>`** - Path to stack file (default: `lxc-stack.yml`)
- **`--project-name <name>`** - Project name (default: directory name)
- **`--volumes`** - Remove named volumes (DESTRUCTIVE - data will be lost)
- **`--remove-orphans`** - Remove containers not defined in current stack
- **`-t, --timeout <seconds>`** - Timeout for container stop (default: 10)

**Examples:**
```bash
# Stop and remove containers (preserves volumes)
pxc down

# Remove everything including persistent data
pxc down --volumes

# Increase timeout for graceful database shutdown
pxc down --timeout 60

# Clean up orphaned containers
pxc down --remove-orphans
```

### pxc ps

List LXC containers with status and resource information.

**Usage:** `pxc ps [OPTIONS]`

**Options:**
- **`-a, --all`** - Show all containers (not just pxc-managed)
- **`-q, --quiet`** - Show only container IDs (useful for scripting)
- **`--filter <key=value>`** - Filter containers (tag, name, status)
- **`--format <template>`** - Custom output format using Go templates
- **`--no-trunc`** - Don't truncate output fields

**Format Fields:**
- `{{.VMID}}` - Container ID
- `{{.Name}}` - Container name
- `{{.Status}}` - Container status
- `{{.Uptime}}` - Container uptime
- `{{.Memory}}` - Memory usage
- `{{.CPU}}` - CPU usage percentage
- `{{.Template}}` - Source template
- `{{.Node}}` - Proxmox node

**Examples:**
```bash
# List pxc-managed containers
pxc ps

# List all containers on system
pxc ps --all

# Get container IDs for scripting
pxc ps --quiet

# Filter by status
pxc ps --filter status=running

# Custom output format
pxc ps --format "table {{.VMID}}\t{{.Name}}\t{{.Status}}\t{{.Memory}}"

# CSV output for data processing
pxc ps --format "{{.VMID}},{{.Name}},{{.Status}},{{.Memory}}"
```

## Configuration Files

### Global Configuration (.pxc.yaml)

pxc looks for configuration files in this order:
1. `--config` flag value
2. `./.pxc.yaml` (current directory)  
3. `$HOME/.pxc.yaml` (home directory)

**Configuration Options:**
```yaml
# Storage configuration
storage: "local-lvm"              # Container storage backend
template_storage: "local"         # Template storage location
proxmox_node: "pve"               # Target Proxmox node

# Build configuration
temp_container_prefix: "pxc-build-"  # Temporary container naming
build_timeout: 300                # Build timeout in seconds

# Network configuration
default_bridge: "vmbr0"           # Default network bridge
default_network: "172.20.0.0/24"  # Default container network

# Security defaults
default_unprivileged: true         # Use unprivileged containers by default
```

### LXCfile.yml

Container build configuration. See [LXCfile Reference](LXCfile-reference.md) for complete documentation.

### lxc-stack.yml

Multi-container orchestration configuration. See [lxc-stack Reference](lxc-stack-reference.md) for complete documentation.

## Common Workflows

### Development Workflow
```bash
# 1. Create LXCfile.yml for your application
cat > LXCfile.yml <<EOF
from: "ubuntu:22.04"
metadata:
  name: "myapp"
  version: "dev"
setup:
  - run: "apt-get update && apt-get install -y nodejs npm"
  - copy:
      source: "./src"
      dest: "/opt/app"
EOF

# 2. Build and test the container
pxc build --dry-run --verbose    # Validate configuration
pxc build -t myapp:dev           # Build the template

# 3. Create stack for development
cat > lxc-stack.yml <<EOF
version: "1.0"
services:
  app:
    build: "."
    ports:
      - "3000:3000"
EOF

# 4. Deploy and develop
pxc up                           # Start the stack
# ... make changes ...
pxc down && pxc up --build       # Rebuild and restart
```

### Production Deployment
```bash
# 1. Build production images
pxc build -f LXCfile.yml -t myapp:1.0 --build-arg NODE_ENV=production

# 2. Deploy with production stack
pxc up -f production-stack.yml --project-name myapp-prod

# 3. Monitor deployment
pxc ps --filter tag=myapp-prod

# 4. Update deployment
pxc build -t myapp:1.1 --build-arg VERSION=1.1.0
pxc up --build --project-name myapp-prod
```

### Troubleshooting Workflow
```bash
# 1. Check container status
pxc ps --all

# 2. Validate configuration
pxc build --dry-run --verbose
pxc up --dry-run --verbose

# 3. Check resource usage
pxc ps --format "table {{.Name}}\t{{.Status}}\t{{.Memory}}\t{{.CPU}}"

# 4. Manual container inspection
pct list                        # See all containers
pct config <container-id>       # Check container configuration
pct status <container-id>       # Check detailed status
```

## Integration and Automation

### Shell Integration
```bash
# Add to .bashrc or .zshrc for better experience
alias pxl='pxc ps'                     # List containers
alias pxu='pxc up'                     # Start stack
alias pxd='pxc down'                   # Stop stack
alias pxb='pxc build'                  # Build template

# Function to build and deploy in one command
pxc-deploy() {
    pxc build -t "$1" && pxc up --build
}
```

### CI/CD Integration
```bash
#!/bin/bash
# Example CI/CD script

set -e  # Exit on any error

# Build application template
echo "Building application template..."
pxc build -t "${APP_NAME}:${BUILD_VERSION}" \
  --build-arg VERSION="${BUILD_VERSION}" \
  --build-arg BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Deploy to staging
echo "Deploying to staging..."
PXC_PROXMOX_NODE=staging-node pxc up \
  --project-name "${APP_NAME}-staging" \
  --build-arg VERSION="${BUILD_VERSION}"

# Run health checks
echo "Running health checks..."
sleep 30  # Wait for services to start
curl -f http://staging-host/health || exit 1

# Deploy to production (manual approval required)
echo "Ready for production deployment"
```

### Monitoring Integration
```bash
#!/bin/bash
# Container monitoring script

# Check all pxc containers
while IFS= read -r container_id; do
    status=$(pct status "$container_id" | awk '{print $2}')
    if [ "$status" != "running" ]; then
        echo "ALERT: Container $container_id is $status"
        # Send alert notification
    fi
done < <(pxc ps --quiet)

# Check resource usage
pxc ps --format "{{.Name}},{{.Memory}},{{.CPU}}" | while IFS=, read -r name memory cpu; do
    # Parse memory usage percentage
    if [[ $memory =~ ([0-9]+)MB/([0-9]+)MB ]]; then
        used=${BASH_REMATCH[1]}
        total=${BASH_REMATCH[2]}
        usage=$((used * 100 / total))
        
        if [ $usage -gt 90 ]; then
            echo "ALERT: $name memory usage at ${usage}%"
        fi
    fi
done
```

This CLI reference provides comprehensive documentation for all pxc commands, options, configuration, and common usage patterns.
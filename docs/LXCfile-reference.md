# LXCfile.yml Reference

Complete reference for LXCfile.yml configuration files used to build LXC container templates.

## Overview

An LXCfile.yml defines how to build a reusable LXC container template from a base image. Similar to a Dockerfile, it specifies the base image, build steps, configuration, and metadata needed to create containers.

## File Structure

```yaml
# Required fields
from: "debian:12"
setup:
  - run: "apt-get update"

# Optional sections (all can be omitted)
metadata: {...}
features: {...}
resources: {...}
security: {...}
startup: {...}
network: {...}
mounts: [...]
ports: [...]
health: {...}
cleanup: [...]
labels: {...}
```

## Required Fields

### `from` (string, required)

**Description:** Base template or image to start the container build from.

**Format:** `"<template_name>"` or `"<storage>:vztmpl/<template_file>"`

**Examples:**
```yaml
# Distribution templates (common)
from: "debian:12"
from: "ubuntu:22.04"
from: "alpine:latest"

# Full storage path (for custom storage)
from: "cephfs:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst"
from: "local:vztmpl/custom-template.tar.zst"
```

**Valid Values:**
- Standard distribution names: `debian:11`, `debian:12`, `ubuntu:20.04`, `ubuntu:22.04`, `alpine:3.17`, etc.
- Full template paths for custom storage backends
- Must correspond to templates available in Proxmox template storage

### `setup` (array, required)

**Description:** Build steps executed in order during container template creation. At least one step is required.

**Step Types:**

#### Run Commands
```yaml
setup:
  - run: "apt-get update && apt-get install -y nginx"
  - run: |
      systemctl enable nginx
      systemctl start nginx
```

#### Copy Files
```yaml
setup:
  - copy:
      source: "./app"           # Required: source path on host
      dest: "/opt/app"          # Required: destination in container
      owner: "www-data:www-data" # Optional: ownership (default: root:root)
      mode: "755"               # Optional: permissions (default: preserve)
```

#### Environment Variables
```yaml
setup:
  - env:
      NODE_ENV: "production"
      APP_VERSION: "1.0.0"
      PORT: "3000"
```

#### Working Directory
```yaml
setup:
  - workdir: "/opt/app"  # Changes working directory for subsequent steps
```

**Validation Rules:**
- At least one setup step is required
- Each step must have at least one action (`run`, `copy`, `env`, or `workdir`)
- `copy` steps require both `source` and `dest` fields
- Steps are executed in the order specified

## Optional Fields

### `metadata` (object, optional)

**Description:** Container metadata for identification and organization.

```yaml
metadata:
  name: "web-server"                    # Template name
  description: "Production web server"  # Human-readable description
  version: "1.0.0"                     # Version string
  author: "team@example.com"           # Creator information
```

**Default Values:**
- `name`: `"custom-template"` if not specified
- All other fields: empty if not specified

**Template Naming:** The template name becomes `<name>:<version>` if both are provided, otherwise just `<name>`.

### `features` (object, optional)

**Description:** LXC-specific features and capabilities.

```yaml
features:
  unprivileged: true    # Run as unprivileged container (recommended)
  nesting: false        # Allow nested containers (Docker-in-LXC)
  keyctl: false         # Allow keyring access
  fuse: false           # Enable FUSE filesystem support
  mount:                # Additional mount options
    - "nfs"
    - "cifs"
```

**Default Values:**
- `unprivileged`: `true` (containers run unprivileged by default)
- `nesting`: `false`
- `keyctl`: `false`
- `fuse`: `false`
- `mount`: `[]` (empty array)

**Security Note:** `unprivileged: true` is strongly recommended for security. Only set to `false` if you need privileged container features.

### `resources` (object, optional)

**Description:** Resource limits and hardware allocation.

```yaml
resources:
  cores: 2              # CPU cores (can be fractional like 1.5)
  cpulimit: 50          # CPU limit percentage (0-100, 0 = unlimited)
  cpuunits: 1024        # CPU scheduling weight (100-262144)
  memory: 1024          # Memory in MB
  swap: 512             # Swap in MB
  rootfs: 8             # Root filesystem size in GB
  net_rate: 100         # Network rate limit in MB/s (0 = unlimited)
```

**Default Values:**
- `cores`: `1` (during build), not set in final template
- `memory`: `512` (during build), not set in final template
- All other fields: not set (uses Proxmox defaults)

**Ranges:**
- `cores`: 1-64 (depends on host)
- `cpulimit`: 0-100 (percentage)
- `cpuunits`: 100-262144
- `memory`: 16-available host memory (MB)
- `swap`: 0-available host swap (MB)
- `rootfs`: 1-available storage (GB)
- `net_rate`: 0-10000 (MB/s)

### `security` (object, optional)

**Description:** Security and isolation settings.

```yaml
security:
  isolation: "default"      # Isolation level
  apparmor: true           # Enable AppArmor protection
  seccomp: true            # Enable seccomp filtering
  capabilities:            # Linux capabilities management
    add:                   # Capabilities to add
      - "SYS_ADMIN"
      - "NET_ADMIN"
    drop:                  # Capabilities to drop
      - "SYS_MODULE"
      - "SYS_TIME"
```

**Default Values:**
- `isolation`: `"default"`
- `apparmor`: `true`
- `seccomp`: `true`
- `capabilities`: not set

**Valid Values:**
- `isolation`: `"default"`, `"strict"`, `"privileged"`
- `apparmor`: `true`, `false`
- `seccomp`: `true`, `false`
- `capabilities.add/drop`: Linux capability names (see `man 7 capabilities`)

### `startup` (object, optional)

**Description:** Default startup configuration when container starts.

```yaml
startup:
  command: "systemctl start nginx"  # Command to run on container start
  user: "root"                     # User to run command as
  working_dir: "/opt/app"          # Working directory for command
```

**Default Values:**
- `command`: not set (uses container's default init)
- `user`: `"root"`
- `working_dir`: `"/"`

### `network` (object, optional)

**Description:** Network configuration for the container.

```yaml
network:
  hostname: "web-server"     # Container hostname
  domain: "example.com"      # DNS domain
  dns:                       # DNS servers
    - "8.8.8.8"
    - "1.1.1.1"
  searchdomain: "local"      # DNS search domain
```

**Default Values:**
- `hostname`: container ID if not specified
- `domain`: not set
- `dns`: uses host DNS settings
- `searchdomain`: not set

### `mounts` (array, optional)

**Description:** Mount points and volume definitions.

```yaml
mounts:
  # Bind mount from host
  - source: "/host/data"        # Host path
    target: "/opt/app/data"     # Container path
    type: "bind"                # Mount type
    readonly: false             # Read-only mount
    backup: true                # Include in Proxmox backups

  # Volume mount (directory created on host)
  - target: "/opt/app/logs"     # Container path
    type: "volume"              # Mount type
    size: "1G"                  # Size limit
    backup: false               # Exclude from backups
```

**Mount Types:**
- `bind`: Bind mount existing host directory
- `volume`: Create managed volume directory

**Default Values:**
- `type`: `"bind"`
- `readonly`: `false`
- `backup`: `true`
- `size`: not set (for bind mounts)

**Validation:**
- `target` is required for all mounts
- `source` is required for `bind` type mounts
- `size` only applies to `volume` type mounts

### `ports` (array, optional)

**Description:** Port mappings for network access.

```yaml
ports:
  - container: 80          # Container port (required)
    host: 8080            # Host port (optional)
    protocol: "tcp"       # Protocol

  - container: 443        # No host port = use same port on host
    protocol: "tcp"
```

**Default Values:**
- `host`: same as container port if not specified
- `protocol`: `"tcp"`

**Valid Values:**
- `container`: 1-65535
- `host`: 1-65535
- `protocol`: `"tcp"`, `"udp"`

### `health` (object, optional)

**Description:** Health check configuration for monitoring container status.

```yaml
health:
  test: "curl -f http://localhost:3000/health || exit 1"  # Health check command
  interval: "30s"          # How often to run check
  timeout: "5s"            # Timeout for each check
  retries: 3               # Consecutive failures before unhealthy
  start_period: "60s"      # Grace period after container start
```

**Default Values:**
- `test`: required if health section is present
- `interval`: `"30s"`
- `timeout`: `"5s"`
- `retries`: `3`
- `start_period`: `"0s"`

**Time Format:** Duration strings like `"30s"`, `"5m"`, `"1h"`

### `cleanup` (array, optional)

**Description:** Post-build cleanup steps to optimize the final template.

```yaml
cleanup:
  - run: |
      apt-get autoremove -y
      apt-get autoclean
      rm -rf /var/lib/apt/lists/*
      rm -rf /tmp/*
```

**Format:** Same as `setup` steps (run, copy, env, workdir)

**Purpose:** Remove temporary files, clear caches, uninstall build dependencies

### `labels` (object, optional)

**Description:** Key-value labels for metadata and organization.

```yaml
labels:
  environment: "production"
  project: "web-app"
  tier: "backend"
  version: "1.0.0"
  maintainer: "team@example.com"
```

**Default Values:** Empty if not specified

**Usage:** Labels are stored with the container template and can be used for organization, filtering, and automation.

## Build Process

1. **Parse Configuration:** Validate LXCfile.yml syntax and required fields
2. **Create Temporary Container:** `pct create` with base template
3. **Execute Setup Steps:** Run commands, copy files, set environment variables
4. **Apply Configuration:** Set resources, security, features from LXCfile
5. **Execute Cleanup Steps:** Run optimization and cleanup commands
6. **Export Template:** `pct export` to create reusable template
7. **Remove Temporary Container:** Clean up build artifacts

## Configuration Examples

### Simple Web Application
```yaml
from: "ubuntu:22.04"

metadata:
  name: "nodejs-app"
  version: "1.0.0"

resources:
  cores: 2
  memory: 1024

setup:
  - run: |
      apt-get update
      apt-get install -y nodejs npm nginx
  - copy:
      source: "./app"
      dest: "/opt/app"
      owner: "www-data:www-data"
  - run: |
      cd /opt/app && npm ci --only=production
      systemctl enable nginx

ports:
  - container: 80
    host: 8080

health:
  test: "curl -f http://localhost || exit 1"
  interval: "30s"
```

### Database Server
```yaml
from: "debian:12"

metadata:
  name: "postgres-db"
  version: "15.0"

features:
  unprivileged: true

resources:
  cores: 4
  memory: 2048
  swap: 1024

security:
  isolation: "strict"
  apparmor: true

setup:
  - run: |
      apt-get update
      apt-get install -y postgresql-15 postgresql-contrib
  - env:
      POSTGRES_VERSION: "15"
  - run: |
      systemctl enable postgresql

mounts:
  - target: "/var/lib/postgresql/data"
    type: "volume"
    size: "10G"
    backup: true

health:
  test: "pg_isready -U postgres"
  interval: "30s"
  retries: 5

cleanup:
  - run: |
      apt-get autoremove -y
      apt-get autoclean
```

## Best Practices

1. **Always use unprivileged containers** unless absolutely necessary
2. **Set appropriate resource limits** to prevent resource exhaustion
3. **Use multi-stage setup** for complex builds (install, configure, cleanup)
4. **Include health checks** for production containers
5. **Label containers** for organization and automation
6. **Clean up after builds** to minimize template size
7. **Use specific base images** rather than `:latest` for reproducibility

## Troubleshooting

### Common Errors

**"Base template not found"**
- Check template name spelling and availability in Proxmox
- Use full storage path for non-local storage: `"storage:vztmpl/template.tar.zst"`

**"Setup step failed"**
- Use `--dry-run --verbose` to see what commands would be executed
- Check file paths in copy operations exist on host
- Verify package names and availability in base image

**"Resource allocation failed"**
- Check available resources on Proxmox host
- Reduce memory/CPU requirements for build
- Verify storage has sufficient space

### Validation Commands

```bash
# Validate LXCfile syntax without building
pxc build --dry-run --verbose

# Build with detailed output
pxc build --verbose -t myapp:1.0

# Test template after building
pct create 999 local:template/myapp-1.0.tar.zst --start 1
```
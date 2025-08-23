# lxc-stack.yml Reference

Complete reference for lxc-stack.yml configuration files used to define and orchestrate multi-container applications.

## Overview

An lxc-stack.yml defines a complete application composed of multiple LXC containers, similar to Docker Compose. It specifies services, networks, volumes, and their relationships to deploy complex applications with a single command.

## File Structure

```yaml
# Required fields
version: "1.0"
services:
  web: {...}
  db: {...}

# Optional sections
metadata: {...}
volumes: {...}
networks: {...}
secrets: {...}
configs: {...}
settings: {...}
hooks: {...}
development: {...}
```

## Required Fields

### `version` (string, required)

**Description:** Schema version for compatibility and feature detection.

**Valid Values:** `"1.0"` (current version)

**Example:**
```yaml
version: "1.0"
```

### `services` (object, required)

**Description:** Container service definitions. At least one service is required.

**Format:** Key-value pairs where key is service name, value is service configuration.

**Example:**
```yaml
services:
  web:
    build: "./web"
  database:
    template: "postgres:15"
```

## Service Configuration

Each service in the `services` section can have the following configuration:

### Source Configuration (required, one of)

#### `build` (string or object)

**Description:** Build container from LXCfile. Can be directory path or detailed build config.

**String Format:**
```yaml
services:
  web:
    build: "./web"  # Directory containing LXCfile.yml
```

**Object Format:**
```yaml
services:
  web:
    build:
      context: "./web"                    # Required: build directory
      dockerfile: "LXCfile.yml"           # Optional: custom filename (default: LXCfile.yml)
      args:                               # Optional: build arguments
        NODE_ENV: "production"
        VERSION: "1.0.0"
      target: "production"                # Optional: build target stage
```

#### `template` (string)

**Description:** Use pre-built template instead of building.

**Format:** `"<template_name>"` or `"<template_name>:<version>"`

**Examples:**
```yaml
services:
  web:
    template: "nginx:latest"
  database:
    template: "my-postgres:15.2"
```

**Validation:** Cannot specify both `build` and `template` for the same service.

### Container Configuration

#### `hostname` (string, optional)

**Description:** Container hostname.

**Default:** Service name if not specified

**Example:**
```yaml
services:
  web:
    hostname: "web-server-01"
```

#### `resources` (object, optional)

**Description:** Resource limits that override LXCfile settings.

```yaml
services:
  web:
    resources:
      cores: 4              # CPU cores
      cpulimit: 80          # CPU limit percentage
      cpuunits: 2048        # CPU scheduling weight
      memory: 2048          # Memory in MB
      swap: 1024            # Swap in MB
      rootfs: 20            # Root filesystem size in GB
      net_rate: 1000        # Network rate limit in MB/s
```

**Default Values:** Uses values from LXCfile.yml or global defaults

#### `environment` (object, optional)

**Description:** Environment variables for the container.

```yaml
services:
  web:
    environment:
      NODE_ENV: "production"
      DATABASE_URL: "postgres://user:pass@database:5432/app"
      API_KEY: "secret-key"
```

**Variable Expansion:** Service names can be used for internal communication (e.g., `database` resolves to database service IP).

#### `ports` (array, optional)

**Description:** Port mappings from container to host.

**Format:** `"<host_port>:<container_port>"` or `"<container_port>"` (uses same port on host)

```yaml
services:
  web:
    ports:
      - "80:3000"       # Map host:80 to container:3000
      - "443:3443"      # Map host:443 to container:3443
      - "8080"          # Map host:8080 to container:8080
```

#### `expose` (array, optional)

**Description:** Expose ports for internal service communication without mapping to host.

```yaml
services:
  database:
    expose:
      - "5432"          # Accessible from other services but not host
      - "9090"          # Monitoring port
```

#### `volumes` (array, optional)

**Description:** Volume mounts for persistent storage.

**Formats:**
- `"<volume_name>:<container_path>"` - Named volume
- `"<host_path>:<container_path>"` - Host path bind mount
- `"<host_path>:<container_path>:ro"` - Read-only mount

```yaml
services:
  database:
    volumes:
      - "db-data:/var/lib/postgresql/data"    # Named volume
      - "/host/backup:/backup"                # Host bind mount
      - "/host/config:/etc/app:ro"            # Read-only bind mount
```

#### `depends_on` (array, optional)

**Description:** Service dependencies that control startup order.

```yaml
services:
  web:
    depends_on:
      - database
      - cache
  worker:
    depends_on:
      - database
```

**Behavior:** Services wait for dependencies to start before starting themselves.

#### `health` (object, optional)

**Description:** Health check configuration that overrides LXCfile settings.

```yaml
services:
  web:
    health:
      test: "curl -f http://localhost:3000/health"
      interval: "15s"
      timeout: "3s"
      retries: 3
      start_period: "30s"
```

#### `restart` (string, optional)

**Description:** Container restart policy.

**Valid Values:**
- `"no"` - Never restart
- `"always"` - Always restart on exit
- `"on-failure"` - Restart only on non-zero exit
- `"unless-stopped"` - Restart unless manually stopped

**Default:** `"unless-stopped"`

#### `security` (object, optional)

**Description:** Security settings that override LXCfile configuration.

```yaml
services:
  web:
    security:
      isolation: "strict"
      apparmor: true
      capabilities:
        add: ["SYS_ADMIN"]
        drop: ["SYS_MODULE"]
```

#### `networks` (array, optional)

**Description:** Networks this service should connect to.

```yaml
services:
  web:
    networks:
      - frontend
      - backend
```

**Default:** Connected to `default` network if not specified.

#### `backup` (object, optional)

**Description:** Backup configuration for this service.

```yaml
services:
  database:
    backup:
      enabled: true
      schedule: "0 2 * * *"     # Daily at 2 AM (cron format)
      retention: 7              # Keep 7 backups
```

#### `scale` (integer, optional)

**Description:** Number of container instances to run for this service.

```yaml
services:
  worker:
    scale: 3    # Run 3 instances of this service
```

**Default:** `1`

**Naming:** Scaled instances are named `<service>-1`, `<service>-2`, etc.

#### `labels` (object, optional)

**Description:** Labels for service organization and metadata.

```yaml
services:
  web:
    labels:
      tier: "frontend"
      version: "1.0.0"
      monitoring: "enabled"
```

## Optional Top-Level Sections

### `metadata` (object, optional)

**Description:** Stack metadata for identification.

```yaml
metadata:
  name: "web-app-stack"
  description: "Full web application with database"
  version: "2.1.0"
  author: "team@example.com"
```

### `volumes` (object, optional)

**Description:** Named volume definitions for persistent storage.

```yaml
volumes:
  db-data:
    driver: "zfs"               # Storage driver
    options:
      compression: "lz4"        # ZFS compression
      recordsize: "8k"          # ZFS record size

  cache-data:
    driver: "local"             # Standard directory storage

  app-config:
    driver: "local"
    options:
      type: "tmpfs"             # In-memory storage
      device: "tmpfs"
      o: "size=100m"
```

**Volume Drivers:**
- `"local"` - Standard host directory (default)
- `"zfs"` - ZFS dataset (if ZFS storage is available)
- `"nfs"` - NFS mount
- Custom drivers based on Proxmox storage configuration

**Default Values:**
- `driver`: `"local"`
- `options`: empty

### `networks` (object, optional)

**Description:** Network definitions for service communication.

```yaml
networks:
  frontend:
    driver: "bridge"            # Network driver
    name: "web-frontend"        # Custom bridge name
    subnet: "172.20.0.0/24"     # Network subnet
    gateway: "172.20.0.1"       # Gateway IP
    options:
      parent: "vmbr0"           # Parent Proxmox bridge

  backend:
    driver: "bridge"
    subnet: "172.21.0.0/24"
    internal: true              # No external access
    options:
      parent: "vmbr1"
```

**Network Drivers:**
- `"bridge"` - Bridge network (default)
- `"host"` - Use host networking
- `"none"` - No networking

**Default Values:**
- `driver`: `"bridge"`
- `internal`: `false`
- `name`: auto-generated from stack name and network name

### `secrets` (object, optional)

**Description:** Secret management for sensitive data.

```yaml
secrets:
  db_password:
    file: "./secrets/db_password.txt"    # Read from file
  
  api_key:
    external: true                       # Managed externally
    name: "app_api_key"                 # External secret name
```

**Usage in Services:**
```yaml
services:
  database:
    environment:
      POSTGRES_PASSWORD_FILE: "/run/secrets/db_password"
```

### `configs` (object, optional)

**Description:** Configuration file management.

```yaml
configs:
  nginx_conf:
    file: "./config/nginx.conf"         # Source file
    target: "/etc/nginx/nginx.conf"     # Target in container
    mode: 0644                          # File permissions
    user: "nginx"                       # File owner
    group: "nginx"                      # File group
```

### `settings` (object, optional)

**Description:** Global stack configuration and defaults.

```yaml
settings:
  # Default resource limits for all services
  default_resources:
    cores: 1
    memory: 512
    swap: 256

  # Default security settings
  default_security:
    isolation: "default"
    unprivileged: true
    apparmor: true

  # Default network for services
  default_network: "frontend"

  # Default backup settings
  default_backup:
    enabled: false
    retention: 3

  # Proxmox-specific settings
  proxmox:
    node: "pve-node-1"                  # Target Proxmox node
    storage: "local-zfs"                # Container storage
    template_storage: "local"           # Template storage
```

**Default Values:**
- `default_resources.cores`: `1`
- `default_resources.memory`: `512`
- `default_security.isolation`: `"default"`
- `default_security.unprivileged`: `true`
- `default_backup.enabled`: `false`
- `proxmox.storage`: `"local-lvm"`
- `proxmox.template_storage`: `"local"`

### `hooks` (object, optional)

**Description:** Lifecycle event hooks for custom automation.

```yaml
hooks:
  pre_start:                            # Before starting stack
    - "echo 'Starting application...'"
    - "./scripts/pre-start.sh"

  post_start:                           # After starting stack
    - "./scripts/health-check.sh"
    - "echo 'Stack started successfully'"

  pre_stop:                             # Before stopping stack
    - "./scripts/backup-data.sh"

  post_stop:                            # After stopping stack
    - "echo 'Stack stopped cleanly'"
```

**Hook Types:**
- `pre_start`: Execute before any containers start
- `post_start`: Execute after all containers are running
- `pre_stop`: Execute before stopping containers
- `post_stop`: Execute after all containers are stopped

### `development` (object, optional)

**Description:** Development environment overrides and additional services.

```yaml
development:
  # Override service settings for development
  services:
    web:
      environment:
        NODE_ENV: "development"
        DEBUG: "true"
      volumes:
        - "./src:/opt/app/src"          # Live code reload
      ports:
        - "3000:3000"                   # Direct port access

  # Additional services only for development
  extra_services:
    debug:
      build: "./debug"
      ports:
        - "9229:9229"                   # Node.js debug port
    
    docs:
      build: "./docs"
      ports:
        - "4000:4000"                   # Documentation server
```

**Usage:** Development overrides are applied when using development-specific commands or flags.

## Stack Orchestration Process

1. **Parse Configuration:** Load and validate lxc-stack.yml
2. **Dependency Resolution:** Determine service startup order based on `depends_on`
3. **Network Creation:** Create custom networks defined in `networks` section
4. **Volume Creation:** Initialize named volumes from `volumes` section
5. **Service Building:** Build containers that specify `build` configuration
6. **Container Creation:** Create containers for each service with proper configuration
7. **Container Startup:** Start containers in dependency order
8. **Health Checks:** Monitor service health and wait for services to be ready
9. **Hook Execution:** Run post-start hooks after all services are running

## Configuration Examples

### Simple Web Application
```yaml
version: "1.0"

metadata:
  name: "blog-app"
  version: "1.0.0"

services:
  web:
    build: "./web"
    ports:
      - "80:3000"
    environment:
      DATABASE_HOST: "database"
    depends_on:
      - database

  database:
    build: "./database"
    volumes:
      - "db-data:/var/lib/postgresql/data"
    environment:
      POSTGRES_DB: "blog"
      POSTGRES_USER: "blogger"
      POSTGRES_PASSWORD_FILE: "/run/secrets/db_password"

volumes:
  db-data:
    driver: "zfs"
    options:
      compression: "lz4"

secrets:
  db_password:
    file: "./secrets/db_password.txt"
```

### Microservices Architecture
```yaml
version: "1.0"

services:
  frontend:
    build: "./frontend"
    ports:
      - "80:80"
      - "443:443"
    networks:
      - frontend
    depends_on:
      - api-gateway

  api-gateway:
    build: "./gateway"
    networks:
      - frontend
      - backend
    depends_on:
      - user-service
      - order-service

  user-service:
    build: "./services/users"
    networks:
      - backend
    depends_on:
      - database
      - cache

  order-service:
    build: "./services/orders"
    scale: 2                    # Run 2 instances
    networks:
      - backend
    depends_on:
      - database
      - cache

  database:
    template: "postgres:15"
    networks:
      - backend
    volumes:
      - "db-data:/var/lib/postgresql/data"
    backup:
      enabled: true
      schedule: "0 2 * * *"

  cache:
    template: "redis:7"
    networks:
      - backend
    volumes:
      - "cache-data:/data"

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
    driver: "zfs"
  cache-data:
    driver: "local"

settings:
  default_resources:
    cores: 2
    memory: 1024
  
  proxmox:
    storage: "fast-ssd"
    node: "pve-node-2"
```

### Development Environment
```yaml
version: "1.0"

services:
  web:
    build: "./web"
    ports:
      - "3000:3000"
    environment:
      NODE_ENV: "production"
    depends_on:
      - database

  database:
    template: "postgres:15"
    volumes:
      - "db-data:/var/lib/postgresql/data"

volumes:
  db-data:

# Development overrides
development:
  services:
    web:
      environment:
        NODE_ENV: "development"
        DEBUG: "true"
      volumes:
        - "./src:/opt/app/src"    # Live reload
      ports:
        - "3000:3000"
        - "9229:9229"             # Debug port

  extra_services:
    adminer:
      template: "adminer:latest"
      ports:
        - "8080:8080"
      depends_on:
        - database
```

## Best Practices

1. **Use dependency ordering** with `depends_on` to ensure proper startup sequence
2. **Isolate services** with custom networks for security
3. **Use named volumes** for persistent data that should survive container recreation
4. **Set resource limits** to prevent services from consuming excessive resources
5. **Enable backups** for stateful services like databases
6. **Use secrets management** for sensitive configuration
7. **Scale horizontally** with the `scale` parameter for load distribution
8. **Monitor with health checks** to ensure service availability
9. **Use development overrides** to maintain separate dev/prod configurations

## Troubleshooting

### Common Issues

**"Circular dependency detected"**
- Check `depends_on` relationships for loops
- Remove unnecessary dependencies
- Use health checks instead of startup dependencies when possible

**"Service references undefined network"**
- Ensure all networks referenced in services are defined in `networks` section
- Check network name spelling

**"Volume not found"**
- Verify named volumes are defined in `volumes` section
- Check volume name spelling in service definitions

**"Resource allocation failed"**
- Reduce resource requirements in service `resources` section
- Check available resources on target Proxmox node
- Scale down number of service instances

### Validation Commands

```bash
# Validate stack file syntax
pxc up --dry-run --verbose

# Start with specific services only
pxc up web database --dry-run

# Check service dependency order
pxc up --dry-run | grep "Starting"

# Monitor resource usage
pxc ps --format table
```

## Command Reference

```bash
# Deploy complete stack
pxc up

# Deploy specific services
pxc up web database

# Deploy in background
pxc up --detach

# Force rebuild of all images
pxc up --build

# Stop and remove stack
pxc down

# Stop stack but preserve data
pxc down --volumes=false

# View stack status
pxc ps

# Follow logs from all services
pxc logs --follow

# Scale a service
pxc scale web=3
```
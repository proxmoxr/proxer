# Configuration Best Practices Guide

Best practices, patterns, and recommendations for configuring Proxer LXC containers and stacks.

## Overview

This guide provides practical advice for configuring LXCfile.yml and lxc-stack.yml files based on common use cases, security considerations, and operational requirements.

## LXCfile Configuration Best Practices

### Base Image Selection

**✅ Use specific versions instead of latest**
```yaml
# Good - reproducible builds
from: "ubuntu:22.04"
from: "debian:12"

# Avoid - can break builds over time
from: "ubuntu:latest"
from: "debian:latest"
```

**✅ Choose minimal base images when possible**
```yaml
# Lightweight for simple applications
from: "alpine:3.18"

# Full-featured for complex applications
from: "ubuntu:22.04"
```

### Resource Management

**✅ Set appropriate resource limits**
```yaml
resources:
  cores: 2              # Match application needs
  memory: 1024          # Leave room for OS overhead
  swap: 512             # 50% of memory is often sufficient
  rootfs: 8             # Size for app + logs + temporary files
```

**📋 Resource Sizing Guidelines:**
- **Web Applications:** 1-2 cores, 512-1024MB memory
- **Databases:** 2-4 cores, 2048-4096MB memory, generous swap
- **Build Containers:** 2-4 cores, 1024-2048MB memory (temporary)
- **Worker Services:** 1-2 cores, 512-1024MB memory per worker

### Security Configuration

**✅ Always use unprivileged containers**
```yaml
features:
  unprivileged: true    # Critical for security
  nesting: false        # Only enable if needed for Docker-in-LXC
```

**✅ Apply strict security when handling sensitive data**
```yaml
security:
  isolation: "strict"
  apparmor: true
  seccomp: true
  capabilities:
    drop:               # Remove unnecessary capabilities
      - "SYS_MODULE"
      - "SYS_TIME"
      - "SYS_PTRACE"
```

**⚠️ Only add capabilities when absolutely necessary**
```yaml
security:
  capabilities:
    add:
      - "SYS_ADMIN"     # Only if container needs mount operations
      - "NET_ADMIN"     # Only if container manages network interfaces
```

### Build Process Optimization

**✅ Order steps for efficient caching**
```yaml
setup:
  # Install packages first (changes less frequently)
  - run: |
      apt-get update
      apt-get install -y nodejs npm nginx

  # Copy dependency files before source code
  - copy:
      source: "./package.json"
      dest: "/opt/app/package.json"
  
  # Install dependencies (cached if package.json unchanged)
  - run: cd /opt/app && npm ci --only=production
  
  # Copy source code last (changes most frequently)
  - copy:
      source: "./src"
      dest: "/opt/app/src"
```

**✅ Use multi-line scripts for related commands**
```yaml
setup:
  - run: |
      # Update package lists
      apt-get update
      
      # Install packages in single layer
      apt-get install -y \
        nodejs \
        npm \
        nginx \
        curl \
        ca-certificates
      
      # Configure services
      systemctl enable nginx
      systemctl enable myapp
```

**✅ Always include cleanup steps**
```yaml
cleanup:
  - run: |
      # Remove package caches
      apt-get autoremove -y
      apt-get autoclean
      rm -rf /var/lib/apt/lists/*
      
      # Clean temporary files
      rm -rf /tmp/* /var/tmp/*
      
      # Remove build dependencies if any
      apt-get remove -y build-essential
```

### File Management

**✅ Set proper ownership and permissions**
```yaml
setup:
  - copy:
      source: "./app"
      dest: "/opt/app"
      owner: "www-data:www-data"    # Use appropriate service user
      mode: "755"                   # Executable directories
  
  - copy:
      source: "./config/app.conf"
      dest: "/etc/app/app.conf"
      owner: "root:root"           # Config files owned by root
      mode: "644"                  # Read-only for others
```

**✅ Use appropriate mount strategies**
```yaml
mounts:
  # Persistent data - always backup
  - source: "/host/app-data"
    target: "/opt/app/data"
    type: "bind"
    backup: true

  # Temporary/cache data - exclude from backup
  - target: "/opt/app/cache"
    type: "volume"
    size: "1G"
    backup: false

  # Log directories - consider backup needs
  - target: "/opt/app/logs"
    type: "volume"
    size: "2G"
    backup: false           # Logs usually don't need backup
```

### Health Checks

**✅ Implement meaningful health checks**
```yaml
health:
  test: "curl -f http://localhost:3000/health || exit 1"
  interval: "30s"
  timeout: "5s"
  retries: 3
  start_period: "60s"       # Allow time for application startup
```

**📋 Health Check Patterns:**
```yaml
# Web application
health:
  test: "curl -f http://localhost:8080/health"

# Database
health:
  test: "pg_isready -U postgres"

# Service with custom script
health:
  test: "/opt/app/scripts/health-check.sh"

# Simple process check
health:
  test: "pgrep -f myapp"
```

## Stack Configuration Best Practices

### Service Architecture

**✅ Design for single responsibility**
```yaml
services:
  # Separate web tier
  web:
    build: "./web"
    ports:
      - "80:80"
    
  # Separate API tier
  api:
    build: "./api"
    
  # Separate database tier
  database:
    build: "./database"
```

**✅ Use dependency ordering strategically**
```yaml
services:
  web:
    depends_on:
      - api           # Web depends on API
  
  api:
    depends_on:
      - database      # API depends on database
      - cache         # API depends on cache
  
  worker:
    depends_on:
      - database      # Worker processes database tasks
```

### Network Segmentation

**✅ Isolate tiers with custom networks**
```yaml
services:
  web:
    networks:
      - frontend      # Web accessible from external
      - backend       # Web can access API

  api:
    networks:
      - backend       # API not accessible from external

  database:
    networks:
      - backend       # Database completely isolated

networks:
  frontend:
    subnet: "172.20.0.0/24"
    
  backend:
    subnet: "172.21.0.0/24"
    internal: true    # No external access
```

**✅ Use expose instead of ports for internal services**
```yaml
services:
  web:
    ports:
      - "80:3000"     # External access needed
      
  api:
    expose:
      - "8080"        # Internal access only
      
  database:
    expose:
      - "5432"        # Internal access only
```

### Data Management

**✅ Use appropriate volume types**
```yaml
volumes:
  # Database data - use ZFS for performance and snapshots
  db-data:
    driver: "zfs"
    options:
      compression: "lz4"
      recordsize: "8k"

  # Application logs - use local storage
  app-logs:
    driver: "local"

  # Temporary cache - use tmpfs for speed
  cache:
    driver: "local"
    options:
      type: "tmpfs"
      device: "tmpfs"
      o: "size=500m"
```

**✅ Configure backups for persistent data**
```yaml
services:
  database:
    backup:
      enabled: true
      schedule: "0 2 * * *"     # Daily at 2 AM
      retention: 7              # Keep 1 week
    volumes:
      - "db-data:/var/lib/postgresql/data"

  web:
    backup:
      enabled: false            # Stateless service, no backup needed
```

### Resource Planning

**✅ Set global defaults and per-service overrides**
```yaml
settings:
  default_resources:
    cores: 1
    memory: 512
    swap: 256

services:
  # Most services use defaults
  cache:
    template: "redis:7"
    
  # Database needs more resources
  database:
    template: "postgres:15"
    resources:
      cores: 4
      memory: 2048
      swap: 1024
```

**✅ Plan for scaling**
```yaml
services:
  # Scale stateless services
  worker:
    build: "./worker"
    scale: 3            # Multiple workers for parallel processing
    
  # Don't scale stateful services
  database:
    template: "postgres:15"
    # scale: 1 (implicit, don't scale databases)
```

### Environment Configuration

**✅ Use environment variables for configuration**
```yaml
services:
  web:
    environment:
      NODE_ENV: "production"
      DATABASE_URL: "postgres://app:${DB_PASSWORD}@database:5432/app"
      REDIS_URL: "redis://cache:6379"
      LOG_LEVEL: "info"
```

**✅ Manage secrets properly**
```yaml
secrets:
  db_password:
    file: "./secrets/db_password.txt"
  api_key:
    file: "./secrets/api_key.txt"

services:
  api:
    environment:
      DATABASE_PASSWORD_FILE: "/run/secrets/db_password"
      API_KEY_FILE: "/run/secrets/api_key"
```

### Development vs Production

**✅ Use development overrides for local development**
```yaml
# Base production configuration
services:
  web:
    build: "./web"
    environment:
      NODE_ENV: "production"

# Development overrides
development:
  services:
    web:
      environment:
        NODE_ENV: "development"
        DEBUG: "true"
      volumes:
        - "./src:/opt/app/src"      # Live code reload
      ports:
        - "3000:3000"               # Direct access for debugging
        - "9229:9229"               # Debug port
  
  extra_services:
    # Additional services only for development
    debug-ui:
      template: "debug-dashboard:latest"
      ports:
        - "4000:4000"
```

## Common Patterns

### Web Application Stack
```yaml
version: "1.0"

services:
  frontend:
    build: "./frontend"
    ports:
      - "80:80"
      - "443:443"
    depends_on:
      - backend
    networks:
      - frontend

  backend:
    build: "./backend"
    expose:
      - "8080"
    environment:
      DATABASE_HOST: "database"
      CACHE_HOST: "cache"
    depends_on:
      - database
      - cache
    networks:
      - frontend
      - backend

  database:
    build: "./database"
    expose:
      - "5432"
    volumes:
      - "db-data:/var/lib/postgresql/data"
    networks:
      - backend
    backup:
      enabled: true
      schedule: "0 2 * * *"

  cache:
    template: "redis:7"
    expose:
      - "6379"
    networks:
      - backend

networks:
  frontend:
    subnet: "172.20.0.0/24"
  backend:
    subnet: "172.21.0.0/24"
    internal: true

volumes:
  db-data:
    driver: "zfs"
```

### Microservices Architecture
```yaml
version: "1.0"

services:
  api-gateway:
    build: "./gateway"
    ports:
      - "80:80"
    depends_on:
      - user-service
      - order-service
    networks:
      - frontend
      - backend

  user-service:
    build: "./services/users"
    expose:
      - "8080"
    depends_on:
      - database
    networks:
      - backend

  order-service:
    build: "./services/orders"
    scale: 2                    # Multiple instances for load
    expose:
      - "8080"
    depends_on:
      - database
      - message-queue
    networks:
      - backend

  message-queue:
    template: "rabbitmq:3-management"
    expose:
      - "5672"
      - "15672"
    volumes:
      - "mq-data:/var/lib/rabbitmq"
    networks:
      - backend

  database:
    template: "postgres:15"
    expose:
      - "5432"
    volumes:
      - "db-data:/var/lib/postgresql/data"
    networks:
      - backend
    backup:
      enabled: true
      schedule: "0 2 * * *"

networks:
  frontend:
    subnet: "172.20.0.0/24"
  backend:
    subnet: "172.21.0.0/24"
    internal: true

volumes:
  db-data:
    driver: "zfs"
  mq-data:
    driver: "local"
```

### CI/CD Pipeline
```yaml
version: "1.0"

services:
  jenkins:
    build: "./jenkins"
    ports:
      - "8080:8080"
    volumes:
      - "jenkins-data:/var/jenkins_home"
      - "/var/run/docker.sock:/var/run/docker.sock"  # Docker-in-Docker
    environment:
      JAVA_OPTS: "-Xmx2048m"

  git:
    template: "gitea:latest"
    ports:
      - "3000:3000"
      - "2222:22"
    volumes:
      - "git-data:/data"
    depends_on:
      - database

  registry:
    template: "registry:2"
    ports:
      - "5000:5000"
    volumes:
      - "registry-data:/var/lib/registry"

  database:
    template: "postgres:15"
    expose:
      - "5432"
    volumes:
      - "db-data:/var/lib/postgresql/data"
    environment:
      POSTGRES_DB: "gitea"
      POSTGRES_USER: "gitea"
      POSTGRES_PASSWORD_FILE: "/run/secrets/db_password"

secrets:
  db_password:
    file: "./secrets/db_password.txt"

volumes:
  jenkins-data:
    driver: "zfs"
  git-data:
    driver: "zfs"
  registry-data:
    driver: "zfs"
  db-data:
    driver: "zfs"
```

## Performance Optimization

### Resource Allocation
- **Monitor actual usage** before setting limits
- **Over-provision memory** slightly (containers share kernel efficiently)
- **Under-provision CPU** initially (can burst when needed)
- **Use CPU limits** to prevent noisy neighbors

### Storage Performance
- **Use ZFS** for databases and frequently accessed data
- **Use tmpfs** for temporary/cache data
- **Separate logs** from application data storage
- **Consider storage location** (local SSD vs network storage)

### Network Performance
- **Use internal networks** for service-to-service communication
- **Minimize hops** between dependent services
- **Consider network latency** in health check timeouts

## Security Hardening

### Container Security
- **Always run unprivileged** unless absolutely necessary
- **Drop unnecessary capabilities** by default
- **Enable AppArmor and seccomp** filtering
- **Use minimal base images** to reduce attack surface

### Network Security
- **Isolate tiers** with separate networks
- **Use internal networks** for backend services
- **Limit exposed ports** to minimum necessary
- **Consider firewall rules** at Proxmox level

### Data Security
- **Encrypt sensitive data** at rest
- **Use secrets management** for passwords and keys
- **Limit file permissions** appropriately
- **Regular security updates** of base images

## Monitoring and Observability

### Health Monitoring
```yaml
# Application health checks
health:
  test: "curl -f http://localhost:8080/health"
  interval: "30s"
  timeout: "5s"
  retries: 3

# Database health checks
health:
  test: "pg_isready -U postgres"
  interval: "30s"

# Custom health checks
health:
  test: "/opt/app/scripts/health-check.sh"
  interval: "60s"
```

### Resource Monitoring
- Set up Proxmox monitoring for resource usage
- Use container logs for application monitoring
- Consider external monitoring tools (Prometheus, Grafana)

## Troubleshooting Common Issues

### Build Failures
1. **Check base image availability** in Proxmox template storage
2. **Verify file paths** in copy operations exist on host
3. **Test commands manually** in a temporary container
4. **Check resource limits** during build process

### Startup Issues
1. **Check dependency order** with `depends_on`
2. **Verify network connectivity** between services
3. **Check resource availability** on Proxmox node
4. **Review health check configurations**

### Performance Issues
1. **Monitor resource usage** with `pxc ps` and Proxmox interface
2. **Check storage performance** with ZFS/storage backend tools
3. **Analyze network latency** between services
4. **Review container resource limits**

### Data Issues
1. **Verify volume mounts** are correctly configured
2. **Check file permissions** and ownership
3. **Confirm backup procedures** are working
4. **Test data recovery** procedures regularly

This guide provides a foundation for configuring robust, secure, and performant LXC container applications with Proxer.
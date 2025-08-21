# Proxer 🚀

**Docker-like developer experience for Proxmox LXC containers**

Proxer (`pxc`) provides a familiar, Docker-inspired workflow for building and orchestrating LXC containers in Proxmox Virtual Environment, while maintaining the performance benefits and tight integration that LXC offers.

## ✨ Features

- **Docker-like CLI**: Familiar commands like `pxc build`, `pxc up`, `pxc ps`
- **Declarative Configuration**: Define containers with `LXCfile.yml` and orchestrate with `lxc-stack.yml`
- **Native Proxmox Integration**: Leverages `pct` commands and Proxmox features like snapshots and backups
- **Single Binary Distribution**: No runtime dependencies, just download and run
- **Template Building**: Build reusable LXC templates from declarative configurations
- **Multi-Container Orchestration**: Deploy complex applications with service dependencies
- **Resource Management**: Fine-grained control over CPU, memory, and storage
- **Security First**: Support for unprivileged containers, AppArmor, and capability management

## 🚀 Quick Start

### Installation

#### Option 1: Download Binary (Recommended)

Download the latest binary from GitHub releases:

```bash
# Linux (x86_64) - Most Proxmox hosts
curl -L https://github.com/brynnjknight/proxer/releases/latest/download/pxc-linux-amd64 -o /usr/local/bin/pxc
chmod +x /usr/local/bin/pxc

# Linux (ARM64)
curl -L https://github.com/brynnjknight/proxer/releases/latest/download/pxc-linux-arm64 -o /usr/local/bin/pxc
chmod +x /usr/local/bin/pxc

# macOS (Intel)
curl -L https://github.com/brynnjknight/proxer/releases/latest/download/pxc-darwin-amd64 -o /usr/local/bin/pxc
chmod +x /usr/local/bin/pxc

# macOS (Apple Silicon)
curl -L https://github.com/brynnjknight/proxer/releases/latest/download/pxc-darwin-arm64 -o /usr/local/bin/pxc
chmod +x /usr/local/bin/pxc
```

#### Option 2: Build from Source

```bash
git clone https://github.com/brynnjknight/proxer.git
cd proxer
go mod tidy
go build -o pxc ./cmd/pxc
sudo mv pxc /usr/local/bin/
```

#### Verify Installation

```bash
pxc version
pxc --help
```

### Your First Container

Create an `LXCfile.yml`:

```yaml
from: "debian:12"

metadata:
  name: "my-app"
  version: "1.0.0"

resources:
  cores: 2
  memory: 1024

setup:
  - run: |
      apt-get update
      apt-get install -y nginx
  
  - copy:
      source: "./html"
      dest: "/var/www/html"

  - run: systemctl enable nginx

ports:
  - container: 80
    host: 8080
```

Build and deploy:

```bash
# Build a template
pxc build -t my-app:1.0

# Test with dry-run first
pxc build --dry-run --verbose
```

## 📋 Use Cases

### Perfect For

- **Development Environments**: Quickly spin up isolated development containers
- **Application Deployment**: Deploy containerized applications with better resource efficiency than Docker
- **CI/CD Pipelines**: Build and test in clean, reproducible environments  
- **Microservices**: Deploy service meshes with native Proxmox networking
- **Legacy Application Modernization**: Containerize without Docker overhead

### Why Choose Proxer over Docker?

- **Better Resource Efficiency**: LXC containers share the kernel more efficiently
- **Proxmox Integration**: Native backup, snapshot, and migration support
- **Hardware Access**: Easier GPU passthrough and device mounting
- **Network Performance**: Direct bridge access without Docker's network overhead
- **Storage Flexibility**: ZFS integration and flexible storage backends

## 📖 Documentation

### Configuration Files

#### LXCfile.yml

Defines how to build a container template:

```yaml
# Base template
from: "ubuntu:22.04"

# Container metadata
metadata:
  name: "web-server"
  description: "Production web server"
  version: "2.1.0"

# LXC features
features:
  unprivileged: true
  nesting: false

# Resource limits
resources:
  cores: 4
  memory: 2048
  swap: 1024

# Security settings
security:
  isolation: "strict"
  apparmor: true

# Build steps
setup:
  - run: apt-get update && apt-get install -y nginx nodejs npm
  - copy:
      source: "./app"
      dest: "/opt/app"
      owner: "www-data:www-data"
  - env:
      NODE_ENV: "production"
      PORT: "3000"

# Startup configuration
startup:
  command: "systemctl start nginx"

# Network configuration  
ports:
  - container: 80
    host: 8080
  - container: 443
    host: 8443

# Health checks
health:
  test: "curl -f http://localhost || exit 1"
  interval: "30s"
  retries: 3
```

#### lxc-stack.yml

Orchestrates multi-container applications:

```yaml
version: "1.0"

services:
  web:
    build: "./web"
    ports:
      - "80:80"
    depends_on:
      - database
    environment:
      DB_HOST: "database"
    
  database:
    build: "./db"
    volumes:
      - "db-data:/var/lib/postgresql/data"
    environment:
      POSTGRES_DB: "myapp"

volumes:
  db-data:
    driver: "zfs"

networks:
  default:
    driver: "bridge"
    subnet: "172.20.0.0/24"
```

### Commands

| Command | Description | Example |
|---------|-------------|---------|
| `pxc build` | Build a template from LXCfile.yml | `pxc build -t myapp:1.0` |
| `pxc up` | Start multi-container application | `pxc up -f lxc-stack.yml` |
| `pxc down` | Stop and remove containers | `pxc down -f lxc-stack.yml` |
| `pxc ps` | List running containers | `pxc ps` |
| `pxc exec` | Execute command in container | `pxc exec web bash` |
| `pxc logs` | View container logs | `pxc logs web` |

### Advanced Features

#### Build Arguments

Pass variables to build process:

```bash
pxc build --build-arg NODE_ENV=production --build-arg VERSION=1.2.3
```

Use in LXCfile.yml:

```yaml
setup:
  - run: echo "Building version ${VERSION} for ${NODE_ENV}"
```

#### Volume Management

```yaml
volumes:
  app-data:
    driver: "zfs"
    options:
      compression: "lz4"
      recordsize: "8k"
      
  logs:
    driver: "local"
    options:
      type: "tmpfs"
      device: "tmpfs"
      o: "size=100m"
```

#### Network Configuration

```yaml
networks:
  frontend:
    driver: "bridge"
    subnet: "172.20.0.0/24"
    options:
      parent: "vmbr0"
      
  backend:
    internal: true
    subnet: "172.21.0.0/24"
```

#### Security Profiles

```yaml
security:
  isolation: "strict"          # default | strict | privileged
  capabilities:
    add: ["SYS_ADMIN"]
    drop: ["SYS_MODULE", "SYS_TIME"]
```

## 🏗️ Architecture

### Build Process

1. **Parse LXCfile.yml** → Validate configuration
2. **Create temp container** → `pct create` from base template  
3. **Execute setup steps** → Run commands, copy files, set environment
4. **Apply configuration** → Set resources, security, features
5. **Export template** → `pct export` to create reusable template
6. **Cleanup** → Remove temporary container

### Orchestration Process

1. **Parse lxc-stack.yml** → Load multi-container configuration
2. **Dependency resolution** → Determine startup order
3. **Build templates** → Build any containers that need building
4. **Create containers** → `pct create` for each service
5. **Configure networking** → Set up bridges and IP allocation
6. **Start services** → Start containers in dependency order

## 🔧 Development

### Building from Source

```bash
git clone https://github.com/brynnjknight/proxer.git
cd proxer
go mod tidy
go build -o pxc ./cmd/pxc
```

### Running Tests

```bash
go test ./...
```

### Project Structure

```
proxer/
├── cmd/pxc/              # CLI entry point
├── internal/
│   ├── cmd/              # Command implementations
│   └── models/           # Data structures
├── pkg/
│   ├── builder/          # Template building logic
│   ├── config/           # Configuration loading
│   ├── runner/           # Container orchestration
│   └── proxmox/          # Proxmox API integration
├── examples/             # Example configurations
└── docs/                 # Documentation
```

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

### Areas for Contribution

- **Additional Commands**: `pxc exec`, `pxc logs`, `pxc stats`
- **Orchestration Features**: Service discovery, health checks, scaling
- **Template Management**: Template registry, versioning, sharing
- **Proxmox Integration**: Cluster support, advanced networking
- **Testing**: Integration tests, end-to-end testing
- **Documentation**: Tutorials, best practices, troubleshooting

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **Proxmox VE Team** - For the excellent LXC implementation
- **tteck & Community Scripts** - For inspiration from Proxmox helper scripts  
- **Docker** - For pioneering the developer experience we're emulating
- **LXC/LXD Teams** - For the container technology foundation

## 🔗 Related Projects

- [proxmox-lxc-compose](https://github.com/larkinwc/proxmox-lxc-compose) - Similar goals with different approach
- [tteck/Proxmox](https://github.com/community-scripts/ProxmoxVE) - Community helper scripts
- [LXD](https://linuxcontainers.org/lxd/) - Advanced container manager

---

**Last Updated On:** August 21, 2025
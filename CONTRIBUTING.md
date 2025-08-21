# Contributing to Proxer

Thank you for your interest in contributing to Proxer! This document provides guidelines and information for contributors.

## 🚀 Getting Started

### Prerequisites

- Go 1.22 or later
- Access to a Proxmox VE environment for testing
- Basic understanding of LXC containers and Proxmox

### Development Setup

1. Fork and clone the repository:
```bash
git clone https://github.com/yourusername/proxer.git
cd proxer
```

2. Install dependencies:
```bash
go mod tidy
```

3. Build the project:
```bash
go build -o pxc ./cmd/pxc
```

4. Run tests:
```bash
go test ./...
```

## 🏗️ Project Structure

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
├── schemas/              # YAML schemas
└── docs/                 # Documentation
```

## 📝 Contributing Guidelines

### Code Style

- Follow standard Go conventions and formatting
- Use `gofmt` to format your code
- Run `go vet` to check for common issues
- Add comments to exported functions and types
- Write descriptive commit messages

### Testing

- Write unit tests for new functionality
- Test on actual Proxmox environments when possible
- Use dry-run mode for development testing
- Include both positive and negative test cases

### Pull Request Process

1. **Create an Issue**: For significant changes, create an issue first to discuss the approach
2. **Fork & Branch**: Create a feature branch from `main`
3. **Implement**: Make your changes with tests
4. **Test**: Ensure all tests pass and functionality works
5. **Document**: Update documentation if needed
6. **Submit**: Create a pull request with a clear description

### Commit Message Format

Use conventional commit format:

```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

Examples:
- `feat(build): add support for multi-stage builds`
- `fix(ps): handle containers without tags properly`
- `docs(readme): update installation instructions`

## 🎯 Areas for Contribution

### High Priority
- **Additional Commands**: `exec`, `logs`, `restart`, `stats`
- **Health Checks**: Implement health check monitoring
- **Service Discovery**: Internal DNS or service mesh integration
- **Template Registry**: Local template management and sharing

### Medium Priority
- **Advanced Networking**: Custom bridge configuration, VLANs
- **Volume Management**: ZFS integration, volume plugins
- **Scaling**: Horizontal scaling for services
- **Monitoring**: Integration with Prometheus/Grafana

### Low Priority
- **GUI Dashboard**: Web interface for management
- **Plugin System**: Extensible architecture
- **Cloud Integration**: Multi-node deployments
- **Migration Tools**: Docker Compose converter

## 🧪 Testing Guidelines

### Unit Tests
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/builder/
```

### Integration Tests
```bash
# Test with dry-run mode
./pxc build --dry-run -f examples/simple-app/LXCfile.yml
./pxc up --dry-run -f examples/full-stack/lxc-stack.yml
```

### Manual Testing on Proxmox
```bash
# Copy binary to Proxmox host
scp pxc root@proxmox-host:/usr/local/bin/

# Test basic functionality
ssh root@proxmox-host
pxc ps
pxc build -f simple-app/LXCfile.yml --dry-run
```

## 📚 Documentation

### Code Documentation
- Document all exported functions and types
- Include examples in documentation comments
- Keep documentation up to date with code changes

### User Documentation
- Update README.md for user-facing changes
- Add examples for new features
- Update help text and command descriptions

## 🐛 Bug Reports

When reporting bugs, please include:

1. **Environment**: OS, Go version, Proxmox version
2. **Steps to Reproduce**: Minimal example to reproduce the issue
3. **Expected Behavior**: What should happen
4. **Actual Behavior**: What actually happens
5. **Configuration**: Relevant LXCfile.yml or lxc-stack.yml
6. **Logs**: Command output with `--verbose` flag

## 💡 Feature Requests

For feature requests, please include:

1. **Use Case**: Why is this feature needed?
2. **Proposed Solution**: How should it work?
3. **Alternatives**: Other ways to solve the problem
4. **Examples**: Mock configuration or usage examples

## 🔒 Security

- Never commit secrets, passwords, or sensitive information
- Follow security best practices for container configurations
- Report security vulnerabilities privately via email

## 📄 License

By contributing to Proxer, you agree that your contributions will be licensed under the MIT License.

## 🤝 Community

- Be respectful and inclusive
- Help others learn and contribute
- Share knowledge and best practices
- Provide constructive feedback

## 🙏 Recognition

Contributors will be recognized in:
- GitHub contributors list
- Release notes for significant contributions
- README acknowledgments

Thank you for contributing to Proxer! 🎉

---

**Last Updated On:** August 21, 2025
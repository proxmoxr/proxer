package models

import (
	"fmt"
	"strings"
)

// LXCStack represents the structure of an lxc-stack.yml configuration
type LXCStack struct {
	// Required: Schema version for compatibility
	Version string `yaml:"version" validate:"required"`

	// Optional: Stack metadata
	Metadata *Metadata `yaml:"metadata,omitempty"`

	// Required: Service definitions (containers)
	Services map[string]Service `yaml:"services" validate:"required"`

	// Optional: Named volumes definition
	Volumes map[string]Volume `yaml:"volumes,omitempty"`

	// Optional: Network definitions
	Networks map[string]Network `yaml:"networks,omitempty"`

	// Optional: Secrets management
	Secrets map[string]Secret `yaml:"secrets,omitempty"`

	// Optional: Configuration files/templates
	Configs map[string]Config `yaml:"configs,omitempty"`

	// Optional: Global settings
	Settings *Settings `yaml:"settings,omitempty"`

	// Optional: Hooks for lifecycle events
	Hooks *Hooks `yaml:"hooks,omitempty"`

	// Optional: Development overrides
	Development *Development `yaml:"development,omitempty"`
}

// Service represents a container service definition
type Service struct {
	// Build configuration
	Build interface{} `yaml:"build,omitempty"`

	// Alternative: use pre-built template
	Template string `yaml:"template,omitempty"`

	// Container-specific overrides
	Hostname string `yaml:"hostname,omitempty"`

	// Resource limits (override LXCfile settings)
	Resources *Resources `yaml:"resources,omitempty"`

	// Environment variables
	Environment map[string]string `yaml:"environment,omitempty"`

	// Port mappings
	Ports []string `yaml:"ports,omitempty"`

	// Exposed ports (internal only)
	Expose []string `yaml:"expose,omitempty"`

	// Volume mounts
	Volumes []string `yaml:"volumes,omitempty"`

	// Service dependencies (start order)
	DependsOn []string `yaml:"depends_on,omitempty"`

	// Health check override
	Health *HealthCheck `yaml:"health,omitempty"`

	// Restart policy
	Restart string `yaml:"restart,omitempty"` // no | always | on-failure | unless-stopped

	// Security overrides
	Security *Security `yaml:"security,omitempty"`

	// Network assignment
	Networks []string `yaml:"networks,omitempty"`

	// Backup configuration
	Backup *BackupConfig `yaml:"backup,omitempty"`

	// Scale this service
	Scale int `yaml:"scale,omitempty"`

	// Labels for the service
	Labels map[string]string `yaml:"labels,omitempty"`
}

// BuildConfig represents build configuration for a service
type BuildConfig struct {
	Context    string            `yaml:"context,omitempty"`
	Dockerfile string            `yaml:"dockerfile,omitempty"`
	Args       map[string]string `yaml:"args,omitempty"`
	Target     string            `yaml:"target,omitempty"`
}

// Volume represents a named volume definition
type Volume struct {
	Driver  string            `yaml:"driver,omitempty"`
	Options map[string]string `yaml:"options,omitempty"`
}

// Network represents a network definition
type Network struct {
	Driver   string            `yaml:"driver,omitempty"`
	Name     string            `yaml:"name,omitempty"`
	Subnet   string            `yaml:"subnet,omitempty"`
	Gateway  string            `yaml:"gateway,omitempty"`
	Internal bool              `yaml:"internal,omitempty"`
	Options  map[string]string `yaml:"options,omitempty"`
}

// Secret represents a secret definition
type Secret struct {
	File     string `yaml:"file,omitempty"`
	External bool   `yaml:"external,omitempty"`
	Name     string `yaml:"name,omitempty"`
}

// Config represents a configuration file definition
type Config struct {
	File   string `yaml:"file,omitempty"`
	Target string `yaml:"target,omitempty"`
	Mode   int    `yaml:"mode,omitempty"`
	User   string `yaml:"user,omitempty"`
	Group  string `yaml:"group,omitempty"`
}

// BackupConfig represents backup configuration
type BackupConfig struct {
	Enabled   bool   `yaml:"enabled,omitempty"`
	Schedule  string `yaml:"schedule,omitempty"`
	Retention int    `yaml:"retention,omitempty"`
}

// Settings represents global stack settings
type Settings struct {
	DefaultResources *Resources     `yaml:"default_resources,omitempty"`
	DefaultSecurity  *Security      `yaml:"default_security,omitempty"`
	DefaultNetwork   string         `yaml:"default_network,omitempty"`
	DefaultBackup    *BackupConfig  `yaml:"default_backup,omitempty"`
	Proxmox          *ProxmoxConfig `yaml:"proxmox,omitempty"`
}

// ProxmoxConfig represents Proxmox-specific settings
type ProxmoxConfig struct {
	Node            string `yaml:"node,omitempty"`
	Storage         string `yaml:"storage,omitempty"`
	TemplateStorage string `yaml:"template_storage,omitempty"`
}

// Hooks represents lifecycle event hooks
type Hooks struct {
	PreStart  []string `yaml:"pre_start,omitempty"`
	PostStart []string `yaml:"post_start,omitempty"`
	PreStop   []string `yaml:"pre_stop,omitempty"`
	PostStop  []string `yaml:"post_stop,omitempty"`
}

// Development represents development overrides
type Development struct {
	Services      map[string]Service `yaml:"services,omitempty"`
	ExtraServices map[string]Service `yaml:"extra_services,omitempty"`
}

// Validate performs basic validation on the LXCStack
func (s *LXCStack) Validate() error {
	if s.Version == "" {
		return fmt.Errorf("'version' field is required")
	}

	if len(s.Services) == 0 {
		return fmt.Errorf("'services' field is required and must contain at least one service")
	}

	// Validate services
	for name, service := range s.Services {
		if err := s.validateService(name, service); err != nil {
			return fmt.Errorf("service '%s': %w", name, err)
		}
	}

	// Validate network references
	for serviceName, service := range s.Services {
		for _, networkName := range service.Networks {
			if _, exists := s.Networks[networkName]; !exists && networkName != "default" {
				return fmt.Errorf("service '%s' references undefined network '%s'", serviceName, networkName)
			}
		}
	}

	// Validate volume references
	for serviceName, service := range s.Services {
		for _, volume := range service.Volumes {
			// Parse volume string (name:path or host:container format)
			if volumeName := parseVolumeName(volume); volumeName != "" {
				if _, exists := s.Volumes[volumeName]; !exists {
					// Check if it's a host path (starts with /) - those are valid
					if volumeName[0] != '/' {
						return fmt.Errorf("service '%s' references undefined volume '%s'", serviceName, volumeName)
					}
				}
			}
		}
	}

	return nil
}

func (s *LXCStack) validateService(name string, service Service) error {
	// Must have either build config or template
	if !service.HasBuild() && service.Template == "" {
		return fmt.Errorf("must specify either 'build' or 'template'")
	}

	// Can't have both build and template
	if service.HasBuild() && service.Template != "" {
		return fmt.Errorf("cannot specify both 'build' and 'template'")
	}

	// Validate build config if present
	if service.HasBuild() {
		buildConfig := service.GetBuildConfig()
		if buildConfig == nil || buildConfig.Context == "" {
			return fmt.Errorf("build context is required")
		}
	}

	// Validate dependencies
	for _, dep := range service.DependsOn {
		if _, exists := s.Services[dep]; !exists {
			return fmt.Errorf("depends_on references undefined service '%s'", dep)
		}
	}

	// Validate restart policy
	if service.Restart != "" {
		validPolicies := []string{"no", "always", "on-failure", "unless-stopped"}
		valid := false
		for _, policy := range validPolicies {
			if service.Restart == policy {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid restart policy '%s', must be one of: %v", service.Restart, validPolicies)
		}
	}

	// Validate scale
	if service.Scale < 0 {
		return fmt.Errorf("scale cannot be negative")
	}

	return nil
}

// parseVolumeName extracts the volume name from a volume string
func parseVolumeName(volume string) string {
	// Handle different volume formats:
	// - "volume_name:/path/in/container"
	// - "/host/path:/path/in/container"
	// - "volume_name:/path:ro"
	parts := splitVolume(volume)
	if len(parts) >= 2 {
		return parts[0]
	}
	return ""
}

// splitVolume splits a volume string by colons, handling Windows paths
func splitVolume(volume string) []string {
	// Split by colon, but be careful with Windows paths like C:\path
	parts := strings.Split(volume, ":")
	
	// Handle Windows drive letters (e.g., C:\path\to\dir:/container/path)
	if len(parts) >= 3 && len(parts[0]) == 1 && 
		strings.Contains("ABCDEFGHIJKLMNOPQRSTUVWXYZ", strings.ToUpper(parts[0])) {
		// Rejoin the first two parts for Windows drive letter
		parts[0] = parts[0] + ":" + parts[1]
		parts = append(parts[:1], parts[2:]...)
	}
	
	return parts
}

// GetServiceDependencyOrder returns services in dependency order
func (s *LXCStack) GetServiceDependencyOrder() ([]string, error) {
	var order []string
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(string) error
	visit = func(serviceName string) error {
		if visiting[serviceName] {
			return fmt.Errorf("circular dependency detected involving service '%s'", serviceName)
		}
		if visited[serviceName] {
			return nil
		}

		visiting[serviceName] = true

		service := s.Services[serviceName]
		for _, dep := range service.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[serviceName] = false
		visited[serviceName] = true
		order = append(order, serviceName)

		return nil
	}

	for serviceName := range s.Services {
		if err := visit(serviceName); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// GetBuildConfig returns the build configuration for a service
func (s *Service) GetBuildConfig() *BuildConfig {
	if s.Build == nil {
		return nil
	}

	switch build := s.Build.(type) {
	case string:
		// Build field is a string (directory path)
		return &BuildConfig{
			Context: build,
		}
	case map[string]interface{}:
		// Build field is an object, convert to BuildConfig
		config := &BuildConfig{}
		if context, ok := build["context"].(string); ok {
			config.Context = context
		}
		if dockerfile, ok := build["dockerfile"].(string); ok {
			config.Dockerfile = dockerfile
		}
		if target, ok := build["target"].(string); ok {
			config.Target = target
		}
		if args, ok := build["args"].(map[string]interface{}); ok {
			config.Args = make(map[string]string)
			for k, v := range args {
				if strVal, ok := v.(string); ok {
					config.Args[k] = strVal
				}
			}
		}
		return config
	default:
		return nil
	}
}

// HasBuild returns true if the service has build configuration
func (s *Service) HasBuild() bool {
	return s.Build != nil
}

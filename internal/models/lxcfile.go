package models

import (
	"fmt"
	"time"
)

// LXCfile represents the structure of an LXCfile.yml configuration
type LXCfile struct {
	// Required: Base template/image to start from
	From string `yaml:"from" validate:"required"`

	// Optional: Container metadata
	Metadata *Metadata `yaml:"metadata,omitempty"`

	// Optional: LXC-specific features and configurations
	Features *Features `yaml:"features,omitempty"`

	// Optional: Resource limits and hardware settings
	Resources *Resources `yaml:"resources,omitempty"`

	// Optional: Security and isolation settings
	Security *Security `yaml:"security,omitempty"`

	// Required: Build steps (executed in order during template creation)
	Setup []SetupStep `yaml:"setup" validate:"required"`

	// Optional: Default command/entrypoint when container starts
	Startup *Startup `yaml:"startup,omitempty"`

	// Optional: Network configuration
	Network *NetworkConfig `yaml:"network,omitempty"`

	// Optional: Mount points and volumes
	Mounts []Mount `yaml:"mounts,omitempty"`

	// Optional: Exposed ports for networking
	Ports []Port `yaml:"ports,omitempty"`

	// Optional: Health check configuration
	Health *HealthCheck `yaml:"health,omitempty"`

	// Optional: Post-build cleanup and optimization
	Cleanup []SetupStep `yaml:"cleanup,omitempty"`

	// Optional: Labels for metadata and organization
	Labels map[string]string `yaml:"labels,omitempty"`
}

// Metadata contains container metadata information
type Metadata struct {
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
	Version     string `yaml:"version,omitempty"`
	Author      string `yaml:"author,omitempty"`
}

// Features defines LXC-specific features and configurations
type Features struct {
	Unprivileged bool     `yaml:"unprivileged,omitempty"`
	Nesting      bool     `yaml:"nesting,omitempty"`
	Keyctl       bool     `yaml:"keyctl,omitempty"`
	Fuse         bool     `yaml:"fuse,omitempty"`
	Mount        []string `yaml:"mount,omitempty"`
}

// Resources defines resource limits and hardware settings
type Resources struct {
	Cores    int `yaml:"cores,omitempty"`
	CPULimit int `yaml:"cpulimit,omitempty"`
	CPUUnits int `yaml:"cpuunits,omitempty"`
	Memory   int `yaml:"memory,omitempty"`   // Memory in MB
	Swap     int `yaml:"swap,omitempty"`     // Swap in MB
	RootFS   int `yaml:"rootfs,omitempty"`   // Root filesystem size in GB
	NetRate  int `yaml:"net_rate,omitempty"` // Network rate limit in MB/s
}

// Security defines security and isolation settings
type Security struct {
	Isolation    string        `yaml:"isolation,omitempty"` // default | strict | privileged
	AppArmor     bool          `yaml:"apparmor,omitempty"`
	Seccomp      bool          `yaml:"seccomp,omitempty"`
	Capabilities *Capabilities `yaml:"capabilities,omitempty"`
}

// Capabilities defines Linux capabilities to add/drop
type Capabilities struct {
	Add  []string `yaml:"add,omitempty"`
	Drop []string `yaml:"drop,omitempty"`
}

// SetupStep represents a single step in the build process
type SetupStep struct {
	Run     string            `yaml:"run,omitempty"`
	Copy    *CopyStep         `yaml:"copy,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	WorkDir string            `yaml:"workdir,omitempty"`
}

// CopyStep defines a file copy operation
type CopyStep struct {
	Source string `yaml:"source" validate:"required"`
	Dest   string `yaml:"dest" validate:"required"`
	Owner  string `yaml:"owner,omitempty"`
	Mode   string `yaml:"mode,omitempty"`
}

// Startup defines the default startup configuration
type Startup struct {
	Command    string `yaml:"command,omitempty"`
	User       string `yaml:"user,omitempty"`
	WorkingDir string `yaml:"working_dir,omitempty"`
}

// NetworkConfig defines network configuration
type NetworkConfig struct {
	Hostname     string   `yaml:"hostname,omitempty"`
	Domain       string   `yaml:"domain,omitempty"`
	DNS          []string `yaml:"dns,omitempty"`
	SearchDomain string   `yaml:"searchdomain,omitempty"`
}

// Mount defines a mount point or volume
type Mount struct {
	Source   string `yaml:"source,omitempty"`
	Target   string `yaml:"target" validate:"required"`
	Type     string `yaml:"type,omitempty"` // bind | volume
	ReadOnly bool   `yaml:"readonly,omitempty"`
	Backup   bool   `yaml:"backup,omitempty"`
	Size     string `yaml:"size,omitempty"` // For volume type
}

// Port defines port mapping
type Port struct {
	Container int    `yaml:"container" validate:"required"`
	Host      int    `yaml:"host,omitempty"`
	Protocol  string `yaml:"protocol,omitempty"` // tcp | udp
}

// HealthCheck defines health check configuration
type HealthCheck struct {
	Test        string        `yaml:"test" validate:"required"`
	Interval    time.Duration `yaml:"interval,omitempty"`
	Timeout     time.Duration `yaml:"timeout,omitempty"`
	Retries     int           `yaml:"retries,omitempty"`
	StartPeriod time.Duration `yaml:"start_period,omitempty"`
}

// Validate performs basic validation on the LXCfile
func (l *LXCfile) Validate() error {
	if l.From == "" {
		return fmt.Errorf("'from' field is required")
	}

	if len(l.Setup) == 0 {
		return fmt.Errorf("'setup' field is required and must contain at least one step")
	}

	// Validate setup steps
	for i, step := range l.Setup {
		if step.Run == "" && step.Copy == nil && step.Env == nil && step.WorkDir == "" {
			return fmt.Errorf("setup step %d must have at least one action (run, copy, env, or workdir)", i+1)
		}

		if step.Copy != nil {
			if step.Copy.Source == "" {
				return fmt.Errorf("setup step %d: copy source is required", i+1)
			}
			if step.Copy.Dest == "" {
				return fmt.Errorf("setup step %d: copy dest is required", i+1)
			}
		}
	}

	// Validate mounts
	for i, mount := range l.Mounts {
		if mount.Target == "" {
			return fmt.Errorf("mount %d: target is required", i+1)
		}
		if mount.Type == "bind" && mount.Source == "" {
			return fmt.Errorf("mount %d: source is required for bind mount", i+1)
		}
	}

	// Validate ports
	for i, port := range l.Ports {
		if port.Container <= 0 || port.Container > 65535 {
			return fmt.Errorf("port %d: container port must be between 1 and 65535", i+1)
		}
		if port.Host != 0 && (port.Host <= 0 || port.Host > 65535) {
			return fmt.Errorf("port %d: host port must be between 1 and 65535", i+1)
		}
	}

	// Validate health check
	if l.Health != nil && l.Health.Test == "" {
		return fmt.Errorf("health check test command is required")
	}

	return nil
}

// GetTemplateName generates a template name based on metadata
func (l *LXCfile) GetTemplateName() string {
	if l.Metadata != nil && l.Metadata.Name != "" {
		name := l.Metadata.Name
		if l.Metadata.Version != "" {
			name += ":" + l.Metadata.Version
		}
		return name
	}
	return "custom-template"
}

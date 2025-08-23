package proxmox

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Client represents a Proxmox client for interacting with LXC containers
type Client struct {
	node    string
	verbose bool
	dryRun  bool
}

// ContainerInfo represents information about an LXC container
type ContainerInfo struct {
	VMID        int               `json:"vmid"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Template    string            `json:"template,omitempty"`
	Lock        string            `json:"lock,omitempty"`
	CPUs        float64           `json:"cpus,omitempty"`
	Memory      int64             `json:"maxmem,omitempty"`
	Disk        int64             `json:"maxdisk,omitempty"`
	Uptime      int64             `json:"uptime,omitempty"`
	PID         int               `json:"pid,omitempty"`
	Tags        string            `json:"tags,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedTime time.Time         `json:"created,omitempty"`
}

// ContainerConfig represents detailed container configuration
type ContainerConfig struct {
	VMID         int               `json:"vmid"`
	Hostname     string            `json:"hostname,omitempty"`
	Memory       int               `json:"memory,omitempty"`
	Swap         int               `json:"swap,omitempty"`
	Cores        int               `json:"cores,omitempty"`
	CPULimit     int               `json:"cpulimit,omitempty"`
	Storage      string            `json:"storage,omitempty"`
	RootFS       string            `json:"rootfs,omitempty"`
	Net0         string            `json:"net0,omitempty"`
	Features     string            `json:"features,omitempty"`
	Unprivileged bool              `json:"unprivileged,omitempty"`
	Environment  map[string]string `json:"env,omitempty"`
	MountPoints  map[string]string `json:"mp,omitempty"`
}

// NewClient creates a new Proxmox client
func NewClient(node string, verbose, dryRun bool) *Client {
	if node == "" {
		node = "localhost"
	}
	return &Client{
		node:    node,
		verbose: verbose,
		dryRun:  dryRun,
	}
}

// ListContainers returns a list of all LXC containers
func (c *Client) ListContainers() ([]ContainerInfo, error) {
	if c.dryRun {
		// Return mock data for dry run
		return []ContainerInfo{
			{
				VMID:   100,
				Name:   "web-server",
				Status: "running",
				CPUs:   2.0,
				Memory: 1024 * 1024 * 1024, // 1GB in bytes
				Uptime: 3600,               // 1 hour
				Tags:   "pxc,webapp",
			},
			{
				VMID:   101,
				Name:   "database",
				Status: "running",
				CPUs:   1.0,
				Memory: 512 * 1024 * 1024, // 512MB in bytes
				Uptime: 7200,              // 2 hours
				Tags:   "pxc,database",
			},
			{
				VMID:   102,
				Name:   "cache",
				Status: "stopped",
				CPUs:   1.0,
				Memory: 256 * 1024 * 1024, // 256MB in bytes
				Tags:   "pxc,cache",
			},
		}, nil
	}

	// Execute pct list command
	cmd := exec.Command("pct", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	return c.parseContainerList(string(output))
}

// GetContainer returns detailed information about a specific container
func (c *Client) GetContainer(vmid int) (*ContainerInfo, error) {
	if c.dryRun {
		// Return mock data for dry run
		return &ContainerInfo{
			VMID:   vmid,
			Name:   fmt.Sprintf("container-%d", vmid),
			Status: "running",
			CPUs:   2.0,
			Memory: 1024 * 1024 * 1024,
			Uptime: 3600,
			Tags:   "pxc",
		}, nil
	}

	containers, err := c.ListContainers()
	if err != nil {
		return nil, err
	}

	for _, container := range containers {
		if container.VMID == vmid {
			return &container, nil
		}
	}

	return nil, fmt.Errorf("container %d not found", vmid)
}

// GetContainerConfig returns the configuration of a container
func (c *Client) GetContainerConfig(vmid int) (*ContainerConfig, error) {
	if c.dryRun {
		return &ContainerConfig{
			VMID:     vmid,
			Hostname: fmt.Sprintf("container-%d", vmid),
			Memory:   1024,
			Cores:    2,
		}, nil
	}

	cmd := exec.Command("pct", "config", strconv.Itoa(vmid))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get container config: %w", err)
	}

	return c.parseContainerConfig(vmid, string(output))
}

// CreateContainer creates a new LXC container
func (c *Client) CreateContainer(vmid int, template string, config *ContainerConfig) error {
	if c.dryRun {
		if c.verbose {
			fmt.Printf("DRY RUN: Would create container %d from template %s\n", vmid, template)
		}
		return nil
	}

	// Detect if template is a container ID (numeric) or file path
	if _, err := strconv.Atoi(template); err == nil {
		// Template is a container ID, use clone
		return c.cloneContainer(vmid, template, config)
	}

	// Template is a file path, use create
	args := []string{"create", strconv.Itoa(vmid), template}

	if config.Hostname != "" {
		args = append(args, "--hostname", config.Hostname)
	}
	if config.Memory > 0 {
		args = append(args, "--memory", strconv.Itoa(config.Memory))
	}
	if config.Cores > 0 {
		args = append(args, "--cores", strconv.Itoa(config.Cores))
	}
	if config.Storage != "" {
		args = append(args, "--storage", config.Storage)
	}

	// Add default network configuration if not specified
	if config.Net0 == "" {
		config.Net0 = "name=eth0,bridge=vmbr0,ip=dhcp,type=veth"
	}
	if config.Net0 != "" {
		args = append(args, "--net0", config.Net0)
	}

	return c.runPCTCommand(args...)
}

// cloneContainer clones a container from a template container
func (c *Client) cloneContainer(vmid int, templateID string, config *ContainerConfig) error {
	// First, clone the template
	args := []string{"clone", templateID, strconv.Itoa(vmid)}

	if config.Hostname != "" {
		args = append(args, "--hostname", config.Hostname)
	}

	if err := c.runPCTCommand(args...); err != nil {
		return err
	}

	// Then configure the cloned container with additional settings
	return c.configureClonedContainer(vmid, config)
}

// configureClonedContainer configures a cloned container with additional settings
func (c *Client) configureClonedContainer(vmid int, config *ContainerConfig) error {
	var args []string

	if config.Memory > 0 {
		args = append(args, "-memory", strconv.Itoa(config.Memory))
	}
	if config.Cores > 0 {
		args = append(args, "-cores", strconv.Itoa(config.Cores))
	}

	// Add default network configuration if not specified
	if config.Net0 == "" {
		// Set up default bridged network with DHCP
		config.Net0 = "name=eth0,bridge=vmbr0,ip=dhcp,type=veth"
	}
	if config.Net0 != "" {
		args = append(args, "-net0", config.Net0)
	}

	// Apply configuration if we have settings to apply
	if len(args) > 0 {
		setArgs := append([]string{"set", strconv.Itoa(vmid)}, args...)
		return c.runPCTCommand(setArgs...)
	}

	return nil
}

// StartContainer starts a container
func (c *Client) StartContainer(vmid int) error {
	if c.dryRun {
		if c.verbose {
			fmt.Printf("DRY RUN: Would start container %d\n", vmid)
		}
		return nil
	}

	return c.runPCTCommand("start", strconv.Itoa(vmid))
}

// StopContainer stops a container
func (c *Client) StopContainer(vmid int) error {
	if c.dryRun {
		if c.verbose {
			fmt.Printf("DRY RUN: Would stop container %d\n", vmid)
		}
		return nil
	}

	return c.runPCTCommand("stop", strconv.Itoa(vmid))
}

// DestroyContainer destroys a container
func (c *Client) DestroyContainer(vmid int) error {
	if c.dryRun {
		if c.verbose {
			fmt.Printf("DRY RUN: Would destroy container %d\n", vmid)
		}
		return nil
	}

	return c.runPCTCommand("destroy", strconv.Itoa(vmid))
}

// ExecCommand executes a command in a container
func (c *Client) ExecCommand(vmid int, command []string) error {
	if c.dryRun {
		if c.verbose {
			fmt.Printf("DRY RUN: Would execute in container %d: %s\n", vmid, strings.Join(command, " "))
		}
		return nil
	}

	args := append([]string{"exec", strconv.Itoa(vmid), "--"}, command...)
	return c.runPCTCommand(args...)
}

// GetContainerLogs returns logs from a container
func (c *Client) GetContainerLogs(vmid int, lines int) (string, error) {
	if c.dryRun {
		return fmt.Sprintf("DRY RUN: Mock logs for container %d", vmid), nil
	}

	var args []string
	if lines > 0 {
		args = []string{"exec", strconv.Itoa(vmid), "--", "journalctl", "-n", strconv.Itoa(lines)}
	} else {
		args = []string{"exec", strconv.Itoa(vmid), "--", "journalctl"}
	}

	cmd := exec.Command("pct", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}

	return string(output), nil
}

// FilterPXCContainers returns only containers managed by pxc (tagged with "pxc")
func (c *Client) FilterPXCContainers(containers []ContainerInfo) []ContainerInfo {
	var pxcContainers []ContainerInfo
	for _, container := range containers {
		if strings.Contains(container.Tags, "pxc") {
			pxcContainers = append(pxcContainers, container)
		}
	}
	return pxcContainers
}

// parseContainerList parses the output of pct list command
func (c *Client) parseContainerList(output string) ([]ContainerInfo, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return []ContainerInfo{}, nil // No containers
	}

	var containers []ContainerInfo

	// Skip header line
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		vmid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		container := ContainerInfo{
			VMID:   vmid,
			Status: fields[1],
			Name:   fields[2],
		}

		// Parse additional fields if available
		if len(fields) > 3 {
			container.Lock = fields[3]
		}

		containers = append(containers, container)
	}

	return containers, nil
}

// parseContainerConfig parses the output of pct config command
func (c *Client) parseContainerConfig(vmid int, output string) (*ContainerConfig, error) {
	config := &ContainerConfig{
		VMID:        vmid,
		Environment: make(map[string]string),
		MountPoints: make(map[string]string),
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "hostname":
			config.Hostname = value
		case "memory":
			if mem, err := strconv.Atoi(value); err == nil {
				config.Memory = mem
			}
		case "swap":
			if swap, err := strconv.Atoi(value); err == nil {
				config.Swap = swap
			}
		case "cores":
			if cores, err := strconv.Atoi(value); err == nil {
				config.Cores = cores
			}
		case "cpulimit":
			if limit, err := strconv.Atoi(value); err == nil {
				config.CPULimit = limit
			}
		case "rootfs":
			config.RootFS = value
		case "net0":
			config.Net0 = value
		case "features":
			config.Features = value
		case "unprivileged":
			config.Unprivileged = value == "1"
		default:
			// Handle mount points (mp0, mp1, etc.)
			if strings.HasPrefix(key, "mp") {
				config.MountPoints[key] = value
			}
		}
	}

	return config, nil
}

// runPCTCommand executes a pct command
func (c *Client) runPCTCommand(args ...string) error {
	cmd := exec.Command("pct", args...)

	if c.verbose {
		fmt.Printf("Executing: pct %s\n", strings.Join(args, " "))
	}

	return cmd.Run()
}

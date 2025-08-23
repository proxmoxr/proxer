package runner

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/brynnjknight/proxer/internal/models"
	"github.com/brynnjknight/proxer/pkg/builder"
	"github.com/brynnjknight/proxer/pkg/config"
	"github.com/brynnjknight/proxer/pkg/proxmox"
)

// Orchestrator manages multi-container applications
type Orchestrator struct {
	client          *proxmox.Client
	builder         *builder.Builder
	verbose         bool
	dryRun          bool
	projectName     string
	baseDir         string
	storage         string
	templateStorage string
}

// Config holds orchestrator configuration
type Config struct {
	Verbose         bool
	DryRun          bool
	ProjectName     string
	BaseDir         string
	ProxmoxNode     string
	Storage         string
	TemplateStorage string
}

// DeploymentResult contains the results of a deployment operation
type DeploymentResult struct {
	Services       []ServiceResult
	Networks       []NetworkResult
	Volumes        []VolumeResult
	DeploymentTime time.Duration
}

// ServiceResult contains the results for a single service
type ServiceResult struct {
	Name        string
	ContainerID int
	Status      string
	BuildTime   time.Duration
	StartTime   time.Duration
	Error       error
}

// NetworkResult contains network creation results
type NetworkResult struct {
	Name   string
	Status string
	Error  error
}

// VolumeResult contains volume creation results
type VolumeResult struct {
	Name   string
	Status string
	Error  error
}

// New creates a new orchestrator instance
func New(config *Config) *Orchestrator {
	if config == nil {
		config = &Config{}
	}

	if config.ProjectName == "" {
		config.ProjectName = "pxc-project"
	}

	if config.BaseDir == "" {
		config.BaseDir = "."
	}

	return &Orchestrator{
		client: proxmox.NewClient("", config.Verbose, config.DryRun),
		builder: builder.New(&builder.Config{
			Verbose:         config.Verbose,
			DryRun:          config.DryRun,
			ProxmoxNode:     config.ProxmoxNode,
			Storage:         config.Storage,
			TemplateStorage: config.TemplateStorage,
		}),
		verbose:         config.Verbose,
		dryRun:          config.DryRun,
		projectName:     config.ProjectName,
		baseDir:         config.BaseDir,
		storage:         config.Storage,
		templateStorage: config.TemplateStorage,
	}
}

// Up deploys a multi-container application
func (o *Orchestrator) Up(stackFile string) (*DeploymentResult, error) {
	startTime := time.Now()

	// Load stack configuration
	o.log("Loading stack configuration: %s", stackFile)
	stack, err := config.LoadLXCStack(stackFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load stack: %w", err)
	}

	// Validate stack
	if err := stack.Validate(); err != nil {
		return nil, fmt.Errorf("invalid stack configuration: %w", err)
	}

	result := &DeploymentResult{
		Services: make([]ServiceResult, 0, len(stack.Services)),
		Networks: make([]NetworkResult, 0, len(stack.Networks)),
		Volumes:  make([]VolumeResult, 0, len(stack.Volumes)),
	}

	o.log("Deploying stack: %s", o.getStackName(stack))

	// Create networks
	if err := o.createNetworks(stack, result); err != nil {
		return result, fmt.Errorf("failed to create networks: %w", err)
	}

	// Create volumes
	if err := o.createVolumes(stack, result); err != nil {
		return result, fmt.Errorf("failed to create volumes: %w", err)
	}

	// Get service dependency order
	serviceOrder, err := stack.GetServiceDependencyOrder()
	if err != nil {
		return result, fmt.Errorf("failed to resolve service dependencies: %w", err)
	}

	o.log("Service startup order: %s", strings.Join(serviceOrder, " -> "))

	// Deploy services in dependency order
	for _, serviceName := range serviceOrder {
		service := stack.Services[serviceName]
		serviceResult := o.deployService(serviceName, service, stack)
		result.Services = append(result.Services, serviceResult)

		if serviceResult.Error != nil {
			return result, fmt.Errorf("failed to deploy service %s: %w", serviceName, serviceResult.Error)
		}
	}

	// Execute post-start hooks
	if stack.Hooks != nil && len(stack.Hooks.PostStart) > 0 {
		o.log("Executing post-start hooks")
		if err := o.executeHooks(stack.Hooks.PostStart); err != nil {
			o.logWarning("Post-start hooks failed: %v", err)
		}
	}

	result.DeploymentTime = time.Since(startTime)
	o.logSuccess("Stack deployed successfully in %v", result.DeploymentTime)

	return result, nil
}

// Down stops and removes a multi-container application
func (o *Orchestrator) Down(stackFile string, removeVolumes bool) error {
	// Load stack configuration
	stack, err := config.LoadLXCStack(stackFile)
	if err != nil {
		return fmt.Errorf("failed to load stack: %w", err)
	}

	o.log("Stopping stack: %s", o.getStackName(stack))

	// Execute pre-stop hooks
	if stack.Hooks != nil && len(stack.Hooks.PreStop) > 0 {
		o.log("Executing pre-stop hooks")
		if err := o.executeHooks(stack.Hooks.PreStop); err != nil {
			o.logWarning("Pre-stop hooks failed: %v", err)
		}
	}

	// Get service order (reverse for shutdown)
	serviceOrder, err := stack.GetServiceDependencyOrder()
	if err != nil {
		return fmt.Errorf("failed to resolve service dependencies: %w", err)
	}

	// Reverse the order for shutdown
	for i := len(serviceOrder)/2 - 1; i >= 0; i-- {
		opp := len(serviceOrder) - 1 - i
		serviceOrder[i], serviceOrder[opp] = serviceOrder[opp], serviceOrder[i]
	}

	o.log("Service shutdown order: %s", strings.Join(serviceOrder, " -> "))

	// Stop and remove services
	for _, serviceName := range serviceOrder {
		if err := o.removeService(serviceName); err != nil {
			o.logWarning("Failed to remove service %s: %v", serviceName, err)
		}
	}

	// Remove volumes if requested
	if removeVolumes {
		if err := o.removeVolumes(stack); err != nil {
			o.logWarning("Failed to remove volumes: %v", err)
		}
	}

	// Execute post-stop hooks
	if stack.Hooks != nil && len(stack.Hooks.PostStop) > 0 {
		o.log("Executing post-stop hooks")
		if err := o.executeHooks(stack.Hooks.PostStop); err != nil {
			o.logWarning("Post-stop hooks failed: %v", err)
		}
	}

	o.logSuccess("Stack stopped successfully")
	return nil
}

// deployService deploys a single service
func (o *Orchestrator) deployService(name string, service models.Service, stack *models.LXCStack) ServiceResult {
	result := ServiceResult{
		Name: name,
	}

	o.log("Deploying service: %s", name)

	// Build or get template
	templateName, err := o.ensureTemplate(name, service)
	if err != nil {
		result.Error = err
		return result
	}

	// Generate container ID
	containerID, err := o.generateContainerID(name)
	if err != nil {
		result.Error = err
		return result
	}
	result.ContainerID = containerID

	// Create container configuration
	containerConfig := o.buildContainerConfig(service, stack)
	containerConfig.Hostname = o.getContainerHostname(name, service)

	// Create container
	if err := o.client.CreateContainer(containerID, templateName, containerConfig); err != nil {
		result.Error = fmt.Errorf("failed to create container: %w", err)
		return result
	}

	// Configure container (set additional properties)
	if err := o.configureContainer(containerID, service); err != nil {
		result.Error = fmt.Errorf("failed to configure container: %w", err)
		return result
	}

	// Start container
	startTime := time.Now()
	if err := o.client.StartContainer(containerID); err != nil {
		result.Error = fmt.Errorf("failed to start container: %w", err)
		return result
	}
	result.StartTime = time.Since(startTime)

	// Wait for health check if defined
	if service.Health != nil {
		if err := o.waitForHealthCheck(containerID, service.Health); err != nil {
			o.logWarning("Health check failed for service %s: %v", name, err)
		}
	}

	result.Status = "running"
	o.logSuccess("Service %s deployed successfully (container %d)", name, containerID)

	return result
}

// ensureTemplate builds or retrieves the template for a service
func (o *Orchestrator) ensureTemplate(serviceName string, service models.Service) (string, error) {
	if service.Template != "" {
		// Use existing template
		return service.Template, nil
	}

	buildConfig := service.GetBuildConfig()
	if buildConfig == nil {
		return "", fmt.Errorf("service %s must specify either 'template' or 'build'", serviceName)
	}

	// Build template from LXCfile
	lxcfilePath := filepath.Join(buildConfig.Context, buildConfig.Dockerfile)
	if buildConfig.Dockerfile == "" {
		lxcfilePath = filepath.Join(buildConfig.Context, "LXCfile.yml")
	}

	// Load LXCfile
	lxcfile, err := config.LoadLXCfile(lxcfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to load LXCfile for service %s: %w", serviceName, err)
	}

	// Generate template name
	templateName := fmt.Sprintf("%s-%s:latest", o.projectName, serviceName)

	// Build template
	o.log("Building template for service %s", serviceName)
	buildStart := time.Now()
	result, err := o.builder.BuildTemplate(lxcfile, templateName, buildConfig.Args)
	if err != nil {
		return "", fmt.Errorf("failed to build template for service %s: %w", serviceName, err)
	}

	o.log("Template built for service %s in %v", serviceName, time.Since(buildStart))
	// Return the container ID (stored in TemplatePath) instead of the templateName
	return result.TemplatePath, nil
}

// generateContainerID generates a unique container ID for a service
func (o *Orchestrator) generateContainerID(serviceName string) (int, error) {
	// Use a hash-based approach for consistent IDs
	// In production, we might want to track these in a state file
	baseID := 200 // Start from 200 to avoid conflicts with manual containers

	// Simple hash of project name + service name
	hash := 0
	for _, char := range o.projectName + serviceName {
		hash = (hash*31 + int(char)) % 1000
	}

	return baseID + hash, nil
}

// buildContainerConfig creates container configuration from service definition
func (o *Orchestrator) buildContainerConfig(service models.Service, stack *models.LXCStack) *proxmox.ContainerConfig {
	config := &proxmox.ContainerConfig{
		Environment: make(map[string]string),
		MountPoints: make(map[string]string),
		Storage:     o.storage, // Set storage from orchestrator config
	}

	// Apply resource limits
	if service.Resources != nil {
		if service.Resources.Memory > 0 {
			config.Memory = service.Resources.Memory
		}
		if service.Resources.Cores > 0 {
			config.Cores = service.Resources.Cores
		}
		if service.Resources.Swap > 0 {
			config.Swap = service.Resources.Swap
		}
	}

	// Apply default resources if not specified
	if config.Memory == 0 && stack.Settings != nil && stack.Settings.DefaultResources != nil {
		config.Memory = stack.Settings.DefaultResources.Memory
	}
	if config.Cores == 0 && stack.Settings != nil && stack.Settings.DefaultResources != nil {
		config.Cores = stack.Settings.DefaultResources.Cores
	}

	// Set environment variables
	for key, value := range service.Environment {
		config.Environment[key] = value
	}

	// Override with stack-specific storage if set
	if stack.Settings != nil && stack.Settings.Proxmox != nil && stack.Settings.Proxmox.Storage != "" {
		config.Storage = stack.Settings.Proxmox.Storage
	}

	return config
}

// Additional helper methods would go here...

func (o *Orchestrator) getStackName(stack *models.LXCStack) string {
	if stack.Metadata != nil && stack.Metadata.Name != "" {
		return stack.Metadata.Name
	}
	return o.projectName
}

func (o *Orchestrator) getContainerHostname(serviceName string, service models.Service) string {
	if service.Hostname != "" {
		return service.Hostname
	}
	return fmt.Sprintf("%s-%s", o.projectName, serviceName)
}

// Placeholder implementations for remaining methods
func (o *Orchestrator) createNetworks(stack *models.LXCStack, result *DeploymentResult) error {
	// TODO: Implement network creation
	if o.verbose {
		o.log("Creating networks (placeholder)")
	}
	return nil
}

func (o *Orchestrator) createVolumes(stack *models.LXCStack, result *DeploymentResult) error {
	// TODO: Implement volume creation
	if o.verbose {
		o.log("Creating volumes (placeholder)")
	}
	return nil
}

func (o *Orchestrator) configureContainer(containerID int, service models.Service) error {
	// TODO: Implement additional container configuration
	return nil
}

func (o *Orchestrator) waitForHealthCheck(containerID int, health *models.HealthCheck) error {
	// TODO: Implement health check waiting
	return nil
}

func (o *Orchestrator) executeHooks(hooks []string) error {
	// TODO: Implement hook execution
	return nil
}

func (o *Orchestrator) removeService(serviceName string) error {
	// TODO: Implement service removal
	o.log("Removing service: %s", serviceName)
	return nil
}

func (o *Orchestrator) removeVolumes(stack *models.LXCStack) error {
	// TODO: Implement volume removal
	return nil
}

// Logging functions
func (o *Orchestrator) log(format string, args ...interface{}) {
	fmt.Printf(color.BlueString("ℹ ")+format+"\n", args...)
}

func (o *Orchestrator) logSuccess(format string, args ...interface{}) {
	fmt.Printf(color.GreenString("✓ ")+format+"\n", args...)
}

func (o *Orchestrator) logWarning(format string, args ...interface{}) {
	fmt.Printf(color.YellowString("⚠ ")+format+"\n", args...)
}

package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/brynnjknight/proxer/internal/models"
)

// Config holds configuration for the builder
type Config struct {
	Verbose bool
	DryRun  bool

	// Proxmox settings
	ProxmoxNode string
	Storage     string

	// Build settings
	TempContainerPrefix string
	TemplateStorage     string
}

// Builder handles the building of LXC templates
type Builder struct {
	config *Config
}

// BuildResult contains the results of a build operation
type BuildResult struct {
	TemplateName  string
	TemplatePath  string
	ContainerID   int
	BuildDuration time.Duration
	ExecutedSteps []string
}

// New creates a new Builder instance
func New(config *Config) *Builder {
	if config == nil {
		config = &Config{}
	}

	// Set defaults
	if config.TempContainerPrefix == "" {
		config.TempContainerPrefix = "pxc-build-"
	}
	if config.ProxmoxNode == "" {
		config.ProxmoxNode = "localhost"
	}
	if config.Storage == "" {
		config.Storage = "local-lvm"
	}
	if config.TemplateStorage == "" {
		config.TemplateStorage = "local"
	}

	return &Builder{config: config}
}

// BuildTemplate builds an LXC template from an LXCfile configuration
func (b *Builder) BuildTemplate(lxcfile *models.LXCfile, templateName string, buildArgs map[string]string) (*BuildResult, error) {
	startTime := time.Now()

	result := &BuildResult{
		TemplateName:  templateName,
		ExecutedSteps: []string{},
	}

	// Generate a unique container ID for the build
	containerID, err := b.generateContainerID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate container ID: %w", err)
	}
	result.ContainerID = containerID

	if b.config.Verbose {
		b.log("Using temporary container ID: %d", containerID)
	}

	// Create temporary container from base template
	if createErr := b.createTempContainer(containerID, lxcfile.From); createErr != nil {
		return nil, fmt.Errorf("failed to create temporary container: %w", createErr)
	}

	// Track if we should cleanup the container (not if it becomes a template)
	shouldCleanup := true
	defer func() {
		if shouldCleanup {
			if cleanupErr := b.cleanupTempContainer(containerID); cleanupErr != nil {
				b.logError("Failed to cleanup temporary container %d: %v", containerID, cleanupErr)
			}
		}
	}()

	// Start the container for configuration
	if startErr := b.startContainer(containerID); startErr != nil {
		return nil, fmt.Errorf("failed to start temporary container: %w", startErr)
	}

	// Wait for container to be ready
	if waitErr := b.waitForContainer(containerID); waitErr != nil {
		return nil, fmt.Errorf("container failed to become ready: %w", waitErr)
	}

	// Execute setup steps
	for i, step := range lxcfile.Setup {
		stepName := fmt.Sprintf("Step %d", i+1)

		if err := b.executeSetupStep(containerID, step, stepName, buildArgs); err != nil {
			return nil, fmt.Errorf("failed to execute %s: %w", stepName, err)
		}

		result.ExecutedSteps = append(result.ExecutedSteps, stepName)
	}

	// Apply resource and security configurations
	if err := b.applyContainerConfig(containerID, lxcfile); err != nil {
		return nil, fmt.Errorf("failed to apply container configuration: %w", err)
	}

	// Execute cleanup steps if any
	for i, step := range lxcfile.Cleanup {
		stepName := fmt.Sprintf("Cleanup %d", i+1)

		if err := b.executeSetupStep(containerID, step, stepName, buildArgs); err != nil {
			b.logWarning("Cleanup step failed (continuing): %v", err)
		} else {
			result.ExecutedSteps = append(result.ExecutedSteps, stepName)
		}
	}

	// Stop the container before export
	if err := b.stopContainer(containerID); err != nil {
		return nil, fmt.Errorf("failed to stop container: %w", err)
	}

	// Export the configured container as a template
	templatePath, err := b.exportTemplate(containerID, templateName)
	if err != nil {
		return nil, fmt.Errorf("failed to export template: %w", err)
	}
	result.TemplatePath = templatePath

	// Don't cleanup the container after it becomes a template
	shouldCleanup = false

	result.BuildDuration = time.Since(startTime)
	return result, nil
}

// generateContainerID generates a unique container ID for the build process
func (b *Builder) generateContainerID() (int, error) {
	// Use timestamp-based ID to avoid conflicts
	// In production, we might want to check with pct list first
	return int(time.Now().Unix()%100000 + 10000), nil
}

// createTempContainer creates a temporary LXC container from a base template
func (b *Builder) createTempContainer(containerID int, baseTemplate string) error {
	b.log("Creating temporary container %d from template: %s", containerID, baseTemplate)

	if b.config.DryRun {
		b.log("DRY RUN: Would create container %d", containerID)
		return nil
	}

	// Basic pct create command
	args := []string{
		"create", strconv.Itoa(containerID),
		baseTemplate,
		"--hostname", fmt.Sprintf("%s%d", b.config.TempContainerPrefix, containerID),
		"--memory", "512", // Default memory for build
		"--cores", "1", // Default cores for build
		"--unprivileged", "1", // Use unprivileged by default
	}

	if b.config.Storage != "" {
		args = append(args, "--storage", b.config.Storage)
	}

	return b.runPCTCommand(args...)
}

// startContainer starts the temporary container
func (b *Builder) startContainer(containerID int) error {
	b.log("Starting container %d", containerID)

	if b.config.DryRun {
		return nil
	}

	return b.runPCTCommand("start", strconv.Itoa(containerID))
}

// stopContainer stops the container
func (b *Builder) stopContainer(containerID int) error {
	b.log("Stopping container %d", containerID)

	if b.config.DryRun {
		return nil
	}

	return b.runPCTCommand("stop", strconv.Itoa(containerID))
}

// waitForContainer waits for the container to be ready for commands
func (b *Builder) waitForContainer(containerID int) error {
	if b.config.DryRun {
		return nil
	}

	b.log("Waiting for container %d to be ready...", containerID)

	// Wait up to 60 seconds for container to be ready
	for i := 0; i < 60; i++ {
		// Try to execute a simple command
		cmd := exec.Command("pct", "exec", strconv.Itoa(containerID), "--", "echo", "ready")
		if err := cmd.Run(); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("container %d did not become ready within 60 seconds", containerID)
}

// executeSetupStep executes a single setup step
func (b *Builder) executeSetupStep(containerID int, step models.SetupStep, stepName string, buildArgs map[string]string) error {
	if step.Run != "" {
		return b.executeRunStep(containerID, step.Run, stepName, buildArgs)
	}

	if step.Copy != nil {
		return b.executeCopyStep(containerID, *step.Copy, stepName)
	}

	if step.Env != nil {
		return b.executeEnvStep(containerID, step.Env, stepName)
	}

	return fmt.Errorf("setup step has no actions")
}

// executeRunStep executes a run command in the container
func (b *Builder) executeRunStep(containerID int, command, stepName string, buildArgs map[string]string) error {
	b.log("%s: Running command", stepName)
	if b.config.Verbose {
		b.log("Command: %s", command)
	}

	if b.config.DryRun {
		return nil
	}

	// Substitute build args in the command
	expandedCommand := b.expandBuildArgs(command, buildArgs)

	// Execute the command in the container
	cmd := exec.Command("pct", "exec", strconv.Itoa(containerID), "--", "sh", "-c", expandedCommand)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// executeCopyStep copies files from host to container
func (b *Builder) executeCopyStep(containerID int, copyStep models.CopyStep, stepName string) error {
	b.log("%s: Copying %s -> %s", stepName, copyStep.Source, copyStep.Dest)

	if b.config.DryRun {
		return nil
	}

	// Verify source exists
	if _, err := os.Stat(copyStep.Source); os.IsNotExist(err) {
		return fmt.Errorf("source file/directory does not exist: %s", copyStep.Source)
	}

	// Create destination directory in container if needed
	destDir := filepath.Dir(copyStep.Dest)
	if destDir != "/" && destDir != "." {
		cmd := exec.Command("pct", "exec", strconv.Itoa(containerID), "--", "mkdir", "-p", destDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	// Copy the file/directory
	cmd := exec.Command("pct", "push", strconv.Itoa(containerID), copyStep.Source, copyStep.Dest)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy files: %w", err)
	}

	// Set ownership and permissions if specified
	if copyStep.Owner != "" {
		cmd = exec.Command("pct", "exec", strconv.Itoa(containerID), "--", "chown", "-R", copyStep.Owner, copyStep.Dest)
		if err := cmd.Run(); err != nil {
			b.logWarning("Failed to set ownership: %v", err)
		}
	}

	if copyStep.Mode != "" {
		cmd = exec.Command("pct", "exec", strconv.Itoa(containerID), "--", "chmod", "-R", copyStep.Mode, copyStep.Dest)
		if err := cmd.Run(); err != nil {
			b.logWarning("Failed to set permissions: %v", err)
		}
	}

	return nil
}

// executeEnvStep sets environment variables (writes to /etc/environment)
func (b *Builder) executeEnvStep(containerID int, env map[string]string, stepName string) error {
	b.log("%s: Setting %d environment variables", stepName, len(env))

	if b.config.DryRun {
		return nil
	}

	// Write environment variables to /etc/environment
	for key, value := range env {
		envLine := fmt.Sprintf("%s=%s", key, value)
		cmd := exec.Command("pct", "exec", strconv.Itoa(containerID), "--", "sh", "-c",
			fmt.Sprintf("echo '%s' >> /etc/environment", envLine))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, err)
		}
	}

	return nil
}

// applyContainerConfig applies resource and security configurations
func (b *Builder) applyContainerConfig(containerID int, lxcfile *models.LXCfile) error {
	b.log("Applying container configuration")

	if b.config.DryRun {
		return nil
	}

	var args []string

	// Apply resource limits
	if lxcfile.Resources != nil {
		if lxcfile.Resources.Cores > 0 {
			args = append(args, "-cores", strconv.Itoa(lxcfile.Resources.Cores))
		}
		if lxcfile.Resources.Memory > 0 {
			args = append(args, "-memory", strconv.Itoa(lxcfile.Resources.Memory))
		}
		if lxcfile.Resources.Swap > 0 {
			args = append(args, "-swap", strconv.Itoa(lxcfile.Resources.Swap))
		}
	}

	// Apply features
	if lxcfile.Features != nil {
		features := []string{}
		if lxcfile.Features.Nesting {
			features = append(features, "nesting=1")
		}
		if lxcfile.Features.Keyctl {
			features = append(features, "keyctl=1")
		}
		if lxcfile.Features.Fuse {
			features = append(features, "fuse=1")
		}

		if len(features) > 0 {
			args = append(args, "-features", strings.Join(features, ","))
		}
	}

	// Apply configuration if we have any settings to apply
	if len(args) > 0 {
		pctArgs := append([]string{"set", strconv.Itoa(containerID)}, args...)
		return b.runPCTCommand(pctArgs...)
	}

	return nil
}

// exportTemplate converts the configured container to a template
func (b *Builder) exportTemplate(containerID int, templateName string) (string, error) {
	b.log("Converting container to template: %s", templateName)

	if b.config.DryRun {
		// Return template name for dry run (format expected by pct create)
		return templateName, nil
	}

	// Convert container to template using pct template command
	args := []string{"template", strconv.Itoa(containerID)}
	if err := b.runPCTCommand(args...); err != nil {
		return "", err
	}

	// Return the container ID as the template reference
	// Proxmox templates are referenced by container ID, not file path
	return strconv.Itoa(containerID), nil
}

// cleanupTempContainer removes the temporary container
func (b *Builder) cleanupTempContainer(containerID int) error {
	if b.config.DryRun {
		return nil
	}

	b.log("Cleaning up temporary container %d", containerID)

	// Try to stop first (in case it's still running)
	_ = b.runPCTCommand("stop", strconv.Itoa(containerID))

	// Destroy the container
	return b.runPCTCommand("destroy", strconv.Itoa(containerID))
}

// runPCTCommand executes a pct command
func (b *Builder) runPCTCommand(args ...string) error {
	cmd := exec.Command("pct", args...)

	if b.config.Verbose {
		b.log("Executing: pct %s", strings.Join(args, " "))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// expandBuildArgs expands build arguments in a command string
func (b *Builder) expandBuildArgs(command string, buildArgs map[string]string) string {
	result := command
	for key, value := range buildArgs {
		result = strings.ReplaceAll(result, "${"+key+"}", value)
		result = strings.ReplaceAll(result, "$"+key, value)
	}
	return result
}

// Logging functions
func (b *Builder) log(format string, args ...interface{}) {
	fmt.Printf(color.BlueString("ℹ ")+format+"\n", args...)
}

func (b *Builder) logWarning(format string, args ...interface{}) {
	fmt.Printf(color.YellowString("⚠ ")+format+"\n", args...)
}

func (b *Builder) logError(format string, args ...interface{}) {
	fmt.Printf(color.RedString("✗ ")+format+"\n", args...)
}

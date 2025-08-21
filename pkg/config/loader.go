package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/brynnjknight/proxer/internal/models"
)

// LoadLXCfile loads and parses an LXCfile.yml configuration
func LoadLXCfile(filename string) (*models.LXCfile, error) {
	// Read the file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read LXCfile: %w", err)
	}

	// Parse YAML
	var lxcfile models.LXCfile
	if err := yaml.Unmarshal(data, &lxcfile); err != nil {
		return nil, fmt.Errorf("failed to parse LXCfile YAML: %w", err)
	}

	// Resolve relative paths in copy steps relative to the LXCfile location
	baseDir := filepath.Dir(filename)
	if err := resolveRelativePaths(&lxcfile, baseDir); err != nil {
		return nil, fmt.Errorf("failed to resolve paths: %w", err)
	}

	return &lxcfile, nil
}

// LoadLXCStack loads and parses an lxc-stack.yml configuration
func LoadLXCStack(filename string) (*models.LXCStack, error) {
	// Read the file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read lxc-stack file: %w", err)
	}

	// Parse YAML
	var stack models.LXCStack
	if err := yaml.Unmarshal(data, &stack); err != nil {
		return nil, fmt.Errorf("failed to parse lxc-stack YAML: %w", err)
	}

	// Resolve relative paths
	baseDir := filepath.Dir(filename)
	if err := resolveStackPaths(&stack, baseDir); err != nil {
		return nil, fmt.Errorf("failed to resolve stack paths: %w", err)
	}

	return &stack, nil
}

// resolveRelativePaths converts relative paths in the LXCfile to absolute paths
func resolveRelativePaths(lxcfile *models.LXCfile, baseDir string) error {
	// Resolve paths in setup steps
	for i := range lxcfile.Setup {
		step := &lxcfile.Setup[i]
		if step.Copy != nil && !filepath.IsAbs(step.Copy.Source) {
			// Don't modify paths that look like container paths (start with /)
			if !strings.HasPrefix(step.Copy.Source, "/") {
				step.Copy.Source = filepath.Join(baseDir, step.Copy.Source)
			}
		}
	}

	// Resolve paths in mounts (host side)
	for i := range lxcfile.Mounts {
		mount := &lxcfile.Mounts[i]
		if mount.Source != "" && !filepath.IsAbs(mount.Source) {
			// Only resolve if it doesn't look like a container path
			if !strings.HasPrefix(mount.Source, "/") {
				mount.Source = filepath.Join(baseDir, mount.Source)
			}
		}
	}

	return nil
}

// resolveStackPaths resolves relative paths in stack configurations
func resolveStackPaths(stack *models.LXCStack, baseDir string) error {
	for serviceName, service := range stack.Services {
		// Resolve build context paths
		if service.HasBuild() {
			buildConfig := service.GetBuildConfig()
			if buildConfig != nil && buildConfig.Context != "" && !filepath.IsAbs(buildConfig.Context) {
				// Update the build context - this is a bit tricky with interface{}
				switch build := service.Build.(type) {
				case string:
					// If it's a string, replace with absolute path
					service.Build = filepath.Join(baseDir, build)
				case map[string]interface{}:
					// If it's an object, update the context field
					build["context"] = filepath.Join(baseDir, buildConfig.Context)
				}
			}
		}

		// Resolve volume paths
		for i := range service.Volumes {
			volume := &service.Volumes[i]
			parts := strings.Split(*volume, ":")
			if len(parts) >= 2 && !filepath.IsAbs(parts[0]) {
				// Only resolve host paths (first part), not container paths
				if !strings.HasPrefix(parts[0], "/") {
					parts[0] = filepath.Join(baseDir, parts[0])
					*volume = strings.Join(parts, ":")
				}
			}
		}

		// Update the service back to the map
		stack.Services[serviceName] = service
	}

	return nil
}

// ValidateConfigExists checks if a configuration file exists and returns a helpful error
func ValidateConfigExists(filename string) error {
	if filename == "" {
		return fmt.Errorf("configuration file not specified")
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// Provide helpful suggestions
		dir := filepath.Dir(filename)
		base := filepath.Base(filename)

		// Look for similar files
		files, _ := os.ReadDir(dir)
		var suggestions []string

		for _, file := range files {
			if !file.IsDir() {
				name := file.Name()
				if strings.Contains(strings.ToLower(name), "lxc") ||
					strings.Contains(strings.ToLower(name), "proxer") ||
					strings.HasSuffix(name, ".yml") ||
					strings.HasSuffix(name, ".yaml") {
					suggestions = append(suggestions, name)
				}
			}
		}

		errMsg := fmt.Sprintf("configuration file not found: %s", filename)
		if len(suggestions) > 0 {
			errMsg += "\n\nDid you mean one of these files?\n"
			for _, suggestion := range suggestions {
				errMsg += fmt.Sprintf("  %s\n", filepath.Join(dir, suggestion))
			}
		} else {
			errMsg += fmt.Sprintf("\n\nTo get started, create an %s file or run 'pxc init'", base)
		}

		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// GetDefaultLXCfile returns the default LXCfile name to look for
func GetDefaultLXCfile() string {
	candidates := []string{
		"LXCfile.yml",
		"LXCfile.yaml",
		"lxcfile.yml",
		"lxcfile.yaml",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return "LXCfile.yml" // Default fallback
}

// GetDefaultStackfile returns the default stack file name to look for
func GetDefaultStackfile() string {
	candidates := []string{
		"lxc-stack.yml",
		"lxc-stack.yaml",
		"stack.yml",
		"stack.yaml",
		"docker-compose.yml", // For migration convenience
		"docker-compose.yaml",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return "lxc-stack.yml" // Default fallback
}

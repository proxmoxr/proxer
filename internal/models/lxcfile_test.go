package models

import (
	"testing"
	"time"
)

func TestLXCfileValidation(t *testing.T) {
	tests := []struct {
		name     string
		lxcfile  LXCfile
		wantErr  bool
		errorMsg string
	}{
		{
			name: "valid minimal LXCfile",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing from field",
			lxcfile: LXCfile{
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
			},
			wantErr:  true,
			errorMsg: "'from' field is required",
		},
		{
			name: "missing setup field",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
			},
			wantErr:  true,
			errorMsg: "'setup' field is required and must contain at least one step",
		},
		{
			name: "empty setup step",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{}, // empty step
				},
			},
			wantErr:  true,
			errorMsg: "setup step 1 must have at least one action (run, copy, env, or workdir)",
		},
		{
			name: "valid setup with copy",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{
						Copy: &CopyStep{
							Source: "./app",
							Dest:   "/opt/app",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "copy step missing source",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{
						Copy: &CopyStep{
							Dest: "/opt/app",
						},
					},
				},
			},
			wantErr:  true,
			errorMsg: "setup step 1: copy source is required",
		},
		{
			name: "copy step missing dest",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{
						Copy: &CopyStep{
							Source: "./app",
						},
					},
				},
			},
			wantErr:  true,
			errorMsg: "setup step 1: copy dest is required",
		},
		{
			name: "valid port configuration",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
				Ports: []Port{
					{Container: 80, Host: 8080, Protocol: "tcp"},
					{Container: 443, Protocol: "tcp"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid container port",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
				Ports: []Port{
					{Container: 0, Host: 8080},
				},
			},
			wantErr:  true,
			errorMsg: "port 1: container port must be between 1 and 65535",
		},
		{
			name: "invalid host port",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
				Ports: []Port{
					{Container: 80, Host: 70000},
				},
			},
			wantErr:  true,
			errorMsg: "port 1: host port must be between 1 and 65535",
		},
		{
			name: "valid mount configuration",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
				Mounts: []Mount{
					{
						Source: "/host/data",
						Target: "/container/data",
						Type:   "bind",
					},
					{
						Target: "/container/logs",
						Type:   "volume",
						Size:   "1G",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "mount missing target",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
				Mounts: []Mount{
					{
						Source: "/host/data",
						Type:   "bind",
					},
				},
			},
			wantErr:  true,
			errorMsg: "mount 1: target is required",
		},
		{
			name: "bind mount missing source",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
				Mounts: []Mount{
					{
						Target: "/container/data",
						Type:   "bind",
					},
				},
			},
			wantErr:  true,
			errorMsg: "mount 1: source is required for bind mount",
		},
		{
			name: "valid health check",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
				Health: &HealthCheck{
					Test:        "curl -f http://localhost || exit 1",
					Interval:    30 * time.Second,
					Timeout:     5 * time.Second,
					Retries:     3,
					StartPeriod: 60 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "health check missing test",
			lxcfile: LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
				Health: &HealthCheck{
					Interval: 30 * time.Second,
				},
			},
			wantErr:  true,
			errorMsg: "health check test command is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.lxcfile.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestGetTemplateName(t *testing.T) {
	tests := []struct {
		name     string
		lxcfile  LXCfile
		expected string
	}{
		{
			name: "with name and version",
			lxcfile: LXCfile{
				Metadata: &Metadata{
					Name:    "webapp",
					Version: "1.0.0",
				},
			},
			expected: "webapp:1.0.0",
		},
		{
			name: "with name only",
			lxcfile: LXCfile{
				Metadata: &Metadata{
					Name: "webapp",
				},
			},
			expected: "webapp",
		},
		{
			name:     "no metadata",
			lxcfile:  LXCfile{},
			expected: "custom-template",
		},
		{
			name: "empty metadata",
			lxcfile: LXCfile{
				Metadata: &Metadata{},
			},
			expected: "custom-template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.lxcfile.GetTemplateName()
			if result != tt.expected {
				t.Errorf("GetTemplateName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSetupStepValidation(t *testing.T) {
	tests := []struct {
		name    string
		step    SetupStep
		valid   bool
		message string
	}{
		{
			name: "valid run step",
			step: SetupStep{
				Run: "apt-get update",
			},
			valid: true,
		},
		{
			name: "valid copy step",
			step: SetupStep{
				Copy: &CopyStep{
					Source: "./app",
					Dest:   "/opt/app",
				},
			},
			valid: true,
		},
		{
			name: "valid env step",
			step: SetupStep{
				Env: map[string]string{
					"NODE_ENV": "production",
				},
			},
			valid: true,
		},
		{
			name: "valid workdir step",
			step: SetupStep{
				WorkDir: "/opt/app",
			},
			valid: true,
		},
		{
			name: "multiple actions in one step",
			step: SetupStep{
				Run: "apt-get update",
				Env: map[string]string{
					"NODE_ENV": "production",
				},
			},
			valid: true,
		},
		{
			name:    "empty step",
			step:    SetupStep{},
			valid:   false,
			message: "should have at least one action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal valid LXCfile with this step
			lxcfile := LXCfile{
				From:  "ubuntu:22.04",
				Setup: []SetupStep{tt.step},
			}

			err := lxcfile.Validate()
			if tt.valid && err != nil {
				t.Errorf("Expected step to be valid, but got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Errorf("Expected step to be invalid, but validation passed")
			}
		})
	}
}

func TestPortValidation(t *testing.T) {
	tests := []struct {
		name     string
		port     Port
		wantErr  bool
		errorMsg string
	}{
		{
			name: "valid port with host mapping",
			port: Port{Container: 80, Host: 8080, Protocol: "tcp"},
		},
		{
			name: "valid port without host mapping",
			port: Port{Container: 80, Protocol: "tcp"},
		},
		{
			name:     "container port too low",
			port:     Port{Container: 0, Host: 8080},
			wantErr:  true,
			errorMsg: "container port must be between 1 and 65535",
		},
		{
			name:     "container port too high",
			port:     Port{Container: 70000, Host: 8080},
			wantErr:  true,
			errorMsg: "container port must be between 1 and 65535",
		},
		{
			name:     "host port too low",
			port:     Port{Container: 80, Host: -1},
			wantErr:  true,
			errorMsg: "host port must be between 1 and 65535",
		},
		{
			name:     "host port too high",
			port:     Port{Container: 80, Host: 70000},
			wantErr:  true,
			errorMsg: "host port must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lxcfile := LXCfile{
				From:  "ubuntu:22.04",
				Setup: []SetupStep{{Run: "echo test"}},
				Ports: []Port{tt.port},
			}

			err := lxcfile.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected validation error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, but got: %v", err)
				}
			}
		})
	}
}

func TestMountValidation(t *testing.T) {
	tests := []struct {
		name     string
		mount    Mount
		wantErr  bool
		errorMsg string
	}{
		{
			name: "valid bind mount",
			mount: Mount{
				Source:   "/host/data",
				Target:   "/container/data",
				Type:     "bind",
				ReadOnly: false,
			},
		},
		{
			name: "valid volume mount",
			mount: Mount{
				Target: "/container/logs",
				Type:   "volume",
				Size:   "1G",
			},
		},
		{
			name:     "missing target",
			mount:    Mount{Source: "/host/data", Type: "bind"},
			wantErr:  true,
			errorMsg: "target is required",
		},
		{
			name:     "bind mount missing source",
			mount:    Mount{Target: "/container/data", Type: "bind"},
			wantErr:  true,
			errorMsg: "source is required for bind mount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lxcfile := LXCfile{
				From:   "ubuntu:22.04",
				Setup:  []SetupStep{{Run: "echo test"}},
				Mounts: []Mount{tt.mount},
			}

			err := lxcfile.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected validation error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, but got: %v", err)
				}
			}
		})
	}
}
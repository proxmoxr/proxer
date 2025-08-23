package models

import (
	"testing"
	"time"
)

func BenchmarkLXCfileValidation(b *testing.B) {
	benchmarks := []struct {
		name    string
		lxcfile *LXCfile
	}{
		{
			name: "minimal valid",
			lxcfile: &LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
			},
		},
		{
			name: "simple with resources",
			lxcfile: &LXCfile{
				From: "ubuntu:22.04",
				Setup: []SetupStep{
					{Run: "apt-get update"},
				},
				Resources: &Resources{
					Cores:  2,
					Memory: 1024,
				},
			},
		},
		{
			name: "complex configuration",
			lxcfile: &LXCfile{
				From: "ubuntu:22.04",
				Metadata: &Metadata{
					Name:        "webapp",
					Description: "Web application",
					Version:     "1.0.0",
					Author:      "test@example.com",
				},
				Setup: []SetupStep{
					{Run: "apt-get update && apt-get install -y nodejs npm"},
					{Copy: &CopyStep{Source: "./app", Dest: "/opt/app"}},
					{Env: map[string]string{"NODE_ENV": "production"}},
				},
				Resources: &Resources{
					Cores:  4,
					Memory: 2048,
					Swap:   1024,
				},
				Features: &Features{
					Unprivileged: true,
					Nesting:      true,
					Keyctl:       false,
					Fuse:         true,
				},
				Security: &Security{
					Isolation: "strict",
					AppArmor:  true,
					Seccomp:   true,
				},
				Ports: []Port{
					{Container: 3000, Host: 8080, Protocol: "tcp"},
					{Container: 80, Host: 8081, Protocol: "tcp"},
				},
				Mounts: []Mount{
					{Source: "/host/data", Target: "/app/data", Type: "bind"},
					{Target: "/tmp/cache", Type: "volume", Size: "1G"},
				},
				Health: &HealthCheck{
					Test:        "curl -f http://localhost:3000 || exit 1",
					Interval:    30 * time.Second,
					Timeout:     5 * time.Second,
					Retries:     3,
					StartPeriod: 60 * time.Second,
				},
			},
		},
		{
			name: "large setup steps",
			lxcfile: generateLargeLXCfile(100),
		},
		{
			name: "many ports",
			lxcfile: generateLXCfileWithManyPorts(50),
		},
		{
			name: "many mounts",
			lxcfile: generateLXCfileWithManyMounts(50),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := bm.lxcfile.Validate()
				if err != nil {
					b.Fatalf("Validation failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkStackValidation(b *testing.B) {
	benchmarks := []struct {
		name  string
		stack *LXCStack
	}{
		{
			name: "minimal valid",
			stack: &LXCStack{
				Version: "1.0",
				Services: map[string]Service{
					"web": {Template: "nginx:latest"},
				},
			},
		},
		{
			name: "complex with dependencies",
			stack: &LXCStack{
				Version: "1.0",
				Services: map[string]Service{
					"database": {Template: "postgres:15"},
					"cache":    {Template: "redis:latest"},
					"api": {
						Build:     "./api",
						DependsOn: []string{"database", "cache"},
					},
					"web": {
						Build:     "./web",
						DependsOn: []string{"api"},
					},
				},
				Networks: map[string]Network{
					"frontend": {Driver: "bridge"},
					"backend":  {Driver: "bridge"},
				},
				Volumes: map[string]Volume{
					"db-data": {Driver: "local"},
				},
			},
		},
		{
			name: "large stack",
			stack: generateLargeStack(100),
		},
		{
			name: "complex dependencies",
			stack: generateStackWithComplexDependencies(20),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := bm.stack.Validate()
				if err != nil {
					b.Fatalf("Validation failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkGetServiceDependencyOrder(b *testing.B) {
	stacks := []struct {
		name  string
		stack *LXCStack
	}{
		{
			name: "simple chain",
			stack: &LXCStack{
				Services: map[string]Service{
					"database": {Template: "postgres:15"},
					"api":      {Template: "node:latest", DependsOn: []string{"database"}},
					"web":      {Template: "nginx:latest", DependsOn: []string{"api"}},
				},
			},
		},
		{
			name: "complex dependencies",
			stack: generateStackWithComplexDependencies(10),
		},
		{
			name: "large dependency graph",
			stack: generateStackWithComplexDependencies(50),
		},
	}

	for _, bm := range stacks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := bm.stack.GetServiceDependencyOrder()
				if err != nil {
					b.Fatalf("GetServiceDependencyOrder failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkServiceBuildConfigParsing(b *testing.B) {
	services := []struct {
		name    string
		service Service
	}{
		{
			name: "string build",
			service: Service{
				Build: "./web",
			},
		},
		{
			name: "object build",
			service: Service{
				Build: map[string]interface{}{
					"context":    "./web",
					"dockerfile": "LXCfile.yml",
					"args": map[string]interface{}{
						"NODE_ENV": "production",
						"VERSION":  "1.0.0",
					},
				},
			},
		},
		{
			name: "complex object build",
			service: Service{
				Build: map[string]interface{}{
					"context":    "./web",
					"dockerfile": "LXCfile.yml",
					"target":     "production",
					"args": map[string]interface{}{
						"NODE_ENV":     "production",
						"VERSION":      "1.0.0",
						"BUILD_DATE":   "2023-01-01",
						"GIT_COMMIT":   "abc123",
						"DATABASE_URL": "postgresql://localhost/app",
					},
					"cache_from": []interface{}{"app:cache"},
					"ssh":        "default",
				},
			},
		},
	}

	for _, bm := range services {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = bm.service.GetBuildConfig()
			}
		})
	}
}

func BenchmarkGetTemplateName(b *testing.B) {
	lxcfiles := []struct {
		name    string
		lxcfile *LXCfile
	}{
		{
			name: "no metadata",
			lxcfile: &LXCfile{
				From: "ubuntu:22.04",
			},
		},
		{
			name: "with name only",
			lxcfile: &LXCfile{
				From: "ubuntu:22.04",
				Metadata: &Metadata{
					Name: "webapp",
				},
			},
		},
		{
			name: "with name and version",
			lxcfile: &LXCfile{
				From: "ubuntu:22.04",
				Metadata: &Metadata{
					Name:    "webapp",
					Version: "1.0.0",
				},
			},
		},
	}

	for _, bm := range lxcfiles {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = bm.lxcfile.GetTemplateName()
			}
		})
	}
}

// Helper functions to generate test data

func generateLargeLXCfile(numSteps int) *LXCfile {
	setup := make([]SetupStep, numSteps)
	for i := 0; i < numSteps; i++ {
		setup[i] = SetupStep{
			Run: "echo 'Step " + string(rune('0'+i%10)) + "'",
		}
	}

	return &LXCfile{
		From:  "ubuntu:22.04",
		Setup: setup,
	}
}

func generateLXCfileWithManyPorts(numPorts int) *LXCfile {
	ports := make([]Port, numPorts)
	for i := 0; i < numPorts; i++ {
		ports[i] = Port{
			Container: 3000 + i,
			Host:      8000 + i,
			Protocol:  "tcp",
		}
	}

	return &LXCfile{
		From: "ubuntu:22.04",
		Setup: []SetupStep{
			{Run: "apt-get update"},
		},
		Ports: ports,
	}
}

func generateLXCfileWithManyMounts(numMounts int) *LXCfile {
	mounts := make([]Mount, numMounts)
	for i := 0; i < numMounts; i++ {
		mounts[i] = Mount{
			Source: "/host/data" + string(rune('0'+i%10)),
			Target: "/container/data" + string(rune('0'+i%10)),
			Type:   "bind",
		}
	}

	return &LXCfile{
		From: "ubuntu:22.04",
		Setup: []SetupStep{
			{Run: "apt-get update"},
		},
		Mounts: mounts,
	}
}

func generateLargeStack(numServices int) *LXCStack {
	services := make(map[string]Service)
	for i := 0; i < numServices; i++ {
		serviceName := "service" + string(rune('0'+i%10)) + string(rune('0'+(i/10)%10))
		services[serviceName] = Service{
			Template: "alpine:latest",
			Environment: map[string]string{
				"SERVICE_ID": string(rune('0' + i%10)),
			},
		}
	}

	return &LXCStack{
		Version:  "1.0",
		Services: services,
	}
}

func generateStackWithComplexDependencies(numServices int) *LXCStack {
	services := make(map[string]Service)
	
	// Create base service
	services["base"] = Service{Template: "alpine:latest"}
	
	// Create services with dependencies
	for i := 1; i < numServices; i++ {
		serviceName := "service" + string(rune('0'+i%10)) + string(rune('0'+(i/10)%10))
		
		var deps []string
		if i == 1 {
			deps = []string{"base"}
		} else {
			// Depend on previous service and base
			prevService := "service" + string(rune('0'+(i-1)%10)) + string(rune('0'+((i-1)/10)%10))
			deps = []string{prevService}
			
			// Add additional dependencies for complexity
			if i%3 == 0 && i > 3 {
				deps = append(deps, "base")
			}
		}
		
		services[serviceName] = Service{
			Template:  "alpine:latest",
			DependsOn: deps,
		}
	}

	return &LXCStack{
		Version:  "1.0",
		Services: services,
	}
}

func BenchmarkValidationWithDifferentComplexities(b *testing.B) {
	// Benchmark validation performance with different configuration complexities
	complexities := []struct {
		name   string
		factor int
	}{
		{"small", 5},
		{"medium", 20},
		{"large", 50},
		{"xlarge", 100},
	}

	for _, complexity := range complexities {
		b.Run("lxcfile_"+complexity.name, func(b *testing.B) {
			lxcfile := generateLargeLXCfile(complexity.factor)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := lxcfile.Validate()
				if err != nil {
					b.Fatalf("Validation failed: %v", err)
				}
			}
		})

		b.Run("stack_"+complexity.name, func(b *testing.B) {
			stack := generateLargeStack(complexity.factor)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := stack.Validate()
				if err != nil {
					b.Fatalf("Validation failed: %v", err)
				}
			}
		})
	}
}
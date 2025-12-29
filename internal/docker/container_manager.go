package docker

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// ContainerInfo holds information about a running container
type ContainerInfo struct {
	ContainerID string
	ImageTag    string
	HostPort    int
	ContainerPort int
	StartedAt   time.Time
}

// ContainerManager manages Docker containers for deployments
type ContainerManager struct {
	client      DockerClient
	containers  map[string]*ContainerInfo // key: deployment key (user+path)
	mu          sync.RWMutex
	portAlloc   *PortAllocator
}

// DockerClient wraps the Docker client interface we need
type DockerClient interface {
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform interface{}, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
}

// PortAllocator manages port allocation
type PortAllocator struct {
	mu    sync.Mutex
	ports map[int]bool
	start int
	end   int
}

func NewPortAllocator(start, end int) *PortAllocator {
	return &PortAllocator{
		ports: make(map[int]bool),
		start: start,
		end:   end,
	}
}

func (pa *PortAllocator) Allocate() (int, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	for port := pa.start; port <= pa.end; port++ {
		if !pa.ports[port] {
			// Check if port is actually available
			if isPortAvailable(port) {
				pa.ports[port] = true
				return port, nil
			}
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", pa.start, pa.end)
}

func (pa *PortAllocator) Release(port int) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	delete(pa.ports, port)
}

func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// sanitizeContainerName replaces invalid characters in container names
// Docker only allows [a-zA-Z0-9][a-zA-Z0-9_.-]
func sanitizeContainerName(name string) string {
	// Replace forward slashes and other invalid chars with dashes
	result := make([]byte, 0, len(name))
	for i, b := range []byte(name) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '.' || b == '-' {
			result = append(result, b)
		} else {
			// Replace invalid chars with dash, but avoid leading/trailing dashes
			if i > 0 && i < len(name)-1 {
				// Only add dash if previous char wasn't a dash
				if len(result) > 0 && result[len(result)-1] != '-' {
					result = append(result, '-')
				}
			}
		}
	}
	// Ensure it starts with alphanumeric
	if len(result) > 0 && (result[0] < 'a' || result[0] > 'z') && (result[0] < 'A' || result[0] > 'Z') && (result[0] < '0' || result[0] > '9') {
		result = append([]byte("z"), result...)
	}
	return string(result)
}

func NewContainerManager(client DockerClient) *ContainerManager {
	return &ContainerManager{
		client:     client,
		containers: make(map[string]*ContainerInfo),
		portAlloc:  NewPortAllocator(9000, 9999), // Use ports 9000-9999
	}
}

// StartContainerForDeployment starts a container for a deployment and returns the host port
func (cm *ContainerManager) StartContainerForDeployment(ctx context.Context, deploymentKey, imageTag string, containerPort int) (*ContainerInfo, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if container already exists
	if info, ok := cm.containers[deploymentKey]; ok {
		// Verify container is still running
		_, err := cm.client.ContainerInspect(ctx, info.ContainerID)
		if err == nil {
			log.Printf("Container %s already running for deployment %s", info.ContainerID[:12], deploymentKey)
			return info, nil
		}
		// Container stopped, remove from map and release port
		delete(cm.containers, deploymentKey)
		cm.portAlloc.Release(info.HostPort)
	}

	// Check for existing container with the same name and remove it
	containerName := sanitizeContainerName(fmt.Sprintf("zerverless-%s", deploymentKey))
	
	// List all containers and find ones with matching name
	containers, err := cm.client.ContainerList(ctx, container.ListOptions{
		All: true, // Include stopped containers
	})
	if err == nil {
		for _, c := range containers {
			// Check if any name matches (containers can have multiple names)
			for _, name := range c.Names {
				if name == "/"+containerName || name == containerName {
					log.Printf("Removing existing container %s (%s) for deployment %s", c.ID[:12], containerName, deploymentKey)
					// Stop if running
					timeout := 10
					cm.client.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})
					// Remove container
					cm.client.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
					break
				}
			}
		}
	}

	// Allocate a host port
	hostPort, err := cm.portAlloc.Allocate()
	if err != nil {
		return nil, fmt.Errorf("allocate port: %w", err)
	}

	// Create port binding
	portBinding := nat.PortBinding{
		HostIP:   "127.0.0.1",
		HostPort: fmt.Sprintf("%d", hostPort),
	}
	portMap := nat.PortMap{
		nat.Port(fmt.Sprintf("%d/tcp", containerPort)): []nat.PortBinding{portBinding},
	}

	// Container config
	containerConfig := &container.Config{
		Image: imageTag,
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%d/tcp", containerPort)): struct{}{},
		},
	}

	// Host config with port mapping
	hostConfig := &container.HostConfig{
		PortBindings: portMap,
		Resources: container.Resources{
			Memory:     256 * 1024 * 1024, // 256MB default
			MemorySwap: -1,                 // Disable swap
		},
		AutoRemove:   false,
		NetworkMode:  "bridge", // Use bridge network for port mapping
		ReadonlyRootfs: false,   // Allow writes for web servers
		// Don't drop all capabilities - web servers need some (like CHOWN for nginx)
		// CapDrop:      []string{"ALL"}, // Commented out to allow web servers to work
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		// Add tmpfs mounts for directories that need to be writable
		Tmpfs: map[string]string{
			"/var/cache/nginx": "",
			"/var/run":          "",
			"/tmp":              "",
		},
	}

	// Create container with sanitized name (already computed above)
	createResp, err := cm.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		cm.portAlloc.Release(hostPort)
		return nil, fmt.Errorf("create container: %w", err)
	}

	containerID := createResp.ID

	// Start container
	if err := cm.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		cm.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		cm.portAlloc.Release(hostPort)
		return nil, fmt.Errorf("start container: %w", err)
	}

	// Wait for container to be ready (check if port is accessible)
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		time.Sleep(500 * time.Millisecond)
		// Check if container is still running
		inspected, err := cm.client.ContainerInspect(ctx, containerID)
		if err != nil {
			cm.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
			cm.portAlloc.Release(hostPort)
			return nil, fmt.Errorf("container inspection failed: %w", err)
		}
		if !inspected.State.Running {
			cm.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
			cm.portAlloc.Release(hostPort)
			return nil, fmt.Errorf("container exited immediately with code %d", inspected.State.ExitCode)
		}
		// Try to connect to the port to verify it's ready
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", hostPort), 1*time.Second)
		if err == nil {
			conn.Close()
			break // Port is accessible
		}
		if i == maxRetries-1 {
			// Container is running but port not accessible yet, continue anyway
			log.Printf("Warning: Container %s started but port %d not yet accessible", containerID[:12], hostPort)
		}
	}

	info := &ContainerInfo{
		ContainerID:   containerID,
		ImageTag:      imageTag,
		HostPort:      hostPort,
		ContainerPort: containerPort,
		StartedAt:     time.Now(),
	}

	cm.containers[deploymentKey] = info
	log.Printf("Started container %s for deployment %s on port %d", containerID[:12], deploymentKey, hostPort)

	return info, nil
}

// GetContainerInfo returns container info for a deployment
func (cm *ContainerManager) GetContainerInfo(deploymentKey string) (*ContainerInfo, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	info, ok := cm.containers[deploymentKey]
	return info, ok
}

// StopContainerForDeployment stops and removes a container for a deployment
func (cm *ContainerManager) StopContainerForDeployment(ctx context.Context, deploymentKey string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	info, ok := cm.containers[deploymentKey]
	if !ok {
		return nil // Already stopped
	}

	// Stop container
	timeout := 10
	if err := cm.client.ContainerStop(ctx, info.ContainerID, container.StopOptions{Timeout: &timeout}); err != nil {
		log.Printf("Error stopping container %s: %v", info.ContainerID[:12], err)
	}

	// Remove container
	if err := cm.client.ContainerRemove(ctx, info.ContainerID, container.RemoveOptions{Force: true}); err != nil {
		log.Printf("Error removing container %s: %v", info.ContainerID[:12], err)
	}

	// Release port
	cm.portAlloc.Release(info.HostPort)

	// Remove from map
	delete(cm.containers, deploymentKey)

	log.Printf("Stopped and removed container %s for deployment %s", info.ContainerID[:12], deploymentKey)
	return nil
}

// ListContainers returns all running containers
func (cm *ContainerManager) ListContainers() map[string]*ContainerInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make(map[string]*ContainerInfo)
	for k, v := range cm.containers {
		result[k] = v
	}
	return result
}


package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
)

type Runtime struct {
	client *client.Client
}

func (r *Runtime) Close() error {
	return r.client.Close()
}

func NewRuntime() (*Runtime, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	return &Runtime{client: cli}, nil
}

type BuildResult struct {
	ImageTag string `json:"image_tag"`
	BuildLog string `json:"build_log,omitempty"`
	Size     int64  `json:"size"`
}

func (r *Runtime) BuildImage(ctx context.Context, repoPath, dockerfilePath, contextPath, imageTag string, timeout time.Duration) (*BuildResult, error) {
	// Resolve paths relative to repository root
	fullContextPath := filepath.Join(repoPath, contextPath)
	fullDockerfilePath := filepath.Join(repoPath, dockerfilePath)

	// Create build context tar
	buildCtx, err := archive.TarWithOptions(fullContextPath, &archive.TarOptions{
		ExcludePatterns: []string{".git"},
	})
	if err != nil {
		return nil, fmt.Errorf("create build context: %w", err)
	}
	defer buildCtx.Close()

	// Build options
	dockerfileRelPath, err := filepath.Rel(fullContextPath, fullDockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("resolve dockerfile path: %w", err)
	}

	buildOptions := types.ImageBuildOptions{
		Dockerfile: dockerfileRelPath,
		Tags:       []string{imageTag},
		Remove:     true,
		NoCache:    false,
		PullParent: true,
	}

	// Build with timeout
	buildCtxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	buildResponse, err := r.client.ImageBuild(buildCtxWithTimeout, buildCtx, buildOptions)
	if err != nil {
		return nil, fmt.Errorf("docker build: %w", err)
	}
	defer buildResponse.Body.Close()

	// Stream build output
	var buildLog strings.Builder
	decoder := json.NewDecoder(buildResponse.Body)
	for {
		var stream map[string]any
		if err := decoder.Decode(&stream); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("read build output: %w", err)
		}
		if streamStr, ok := stream["stream"].(string); ok {
			buildLog.WriteString(streamStr)
		}
		if errorStr, ok := stream["error"].(string); ok {
			buildLog.WriteString("ERROR: " + errorStr + "\n")
		}
	}

	// Get image info
	imageInfo, _, err := r.client.ImageInspectWithRaw(ctx, imageTag)
	if err != nil {
		return nil, fmt.Errorf("inspect image: %w", err)
	}

	return &BuildResult{
		ImageTag: imageTag,
		BuildLog: buildLog.String(),
		Size:     imageInfo.Size,
	}, nil
}

type RunResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

func (r *Runtime) RunContainer(ctx context.Context, imageTag string, command []string, env map[string]string, timeout time.Duration) (*RunResult, error) {
	// Build environment variables
	var envVars []string
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Container config
	containerConfig := &container.Config{
		Image: imageTag,
		Cmd:   command,
		Env:   envVars,
	}

	// Host config with security settings
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:     256 * 1024 * 1024, // 256MB default
			MemorySwap: -1,                 // Disable swap
		},
		AutoRemove:   false, // Don't auto-remove so we can get logs
		NetworkMode:  "none", // Isolated network
		ReadonlyRootfs: true,
		CapDrop:      []string{"ALL"},
	}

	// Create container
	createResp, err := r.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	containerID := createResp.ID

	// Start container
	if err := r.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("start container: %w", err)
	}

	// Get logs stream before waiting (attach to container)
	logs, err := r.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
	})
	if err != nil {
		// If we can't get logs, still wait for container
		logs = nil
	}

	// Wait for completion or timeout
	statusCh, errCh := r.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	var exitCode int
	select {
	case err := <-errCh:
		if logs != nil {
			logs.Close()
		}
		r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("wait container: %w", err)
	case status := <-statusCh:
		exitCode = int(status.StatusCode)
	case <-time.After(timeout):
		// Timeout - stop and remove container
		if logs != nil {
			logs.Close()
		}
		timeoutSeconds := 10
		r.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSeconds})
		r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("execution timeout")
	}

	// Read logs if we got the stream
	var output string
	if logs != nil {
		logBytes, _ := io.ReadAll(logs)
		output = string(logBytes)
		logs.Close()
	} else {
		// Try to get logs after container stopped
		logs2, err := r.client.ContainerLogs(ctx, containerID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
		})
		if err == nil {
			logBytes, _ := io.ReadAll(logs2)
			output = string(logBytes)
			logs2.Close()
		}
	}

	// Cleanup
	r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

	if exitCode != 0 {
		return &RunResult{
			Output:   output,
			ExitCode: exitCode,
			Error:    fmt.Sprintf("container exited with code %d", exitCode),
		}, nil
	}

	return &RunResult{
		Output:   output,
		ExitCode: exitCode,
	}, nil
}


package iac

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// AppManifest defines the Infrastructure-as-Code specification for a vibecoded app.
type AppManifest struct {
	ArtifactID   string            `json:"artifactId"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Image        string            `json:"image"`
	Env          map[string]string `json:"env"`
	Port         int               `json:"port"`
	HostPort     int               `json:"hostPort"`
	MemoryMB     int               `json:"memoryMb"`
	CPULimit     string            `json:"cpuLimit"`
	StoragePath  string            `json:"storagePath"`
	IsP2PEnabled bool              `json:"isP2pEnabled"`
}

// ContainerStatus holds runtime health details of a deployed app.
type ContainerStatus struct {
	ContainerID string `json:"containerId"`
	Status      string `json:"status"` // "running", "stopped", "exited"
	URL         string `json:"url"`
	Port        int    `json:"port"`
}

// DockerProvider implements the IaC Provider interface for local/VPS Docker Engine container provisioning.
type DockerProvider struct {
	NetworkName string
}

func NewDockerProvider() *DockerProvider {
	return &DockerProvider{
		NetworkName: "taawun_network",
	}
}

// Deploy provisions a containerized environment based on the AppManifest.
func (p *DockerProvider) Deploy(ctx context.Context, manifest *AppManifest) (*ContainerStatus, error) {
	containerName := fmt.Sprintf("taawun_app_%s", manifest.ArtifactID)

	// Stop/Remove existing container if present
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()

	args := []string{
		"run", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:%d", manifest.HostPort, manifest.Port),
	}

	if manifest.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", manifest.MemoryMB))
	}

	for k, v := range manifest.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, manifest.Image)

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker deploy failed for %s: %s (%w)", manifest.Name, string(out), err)
	}

	containerID := strings.TrimSpace(string(out))
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}

	return &ContainerStatus{
		ContainerID: containerID,
		Status:      "running",
		URL:         fmt.Sprintf("http://localhost:%d", manifest.HostPort),
		Port:        manifest.HostPort,
	}, nil
}

// Teardown stops and removes the deployed app container.
func (p *DockerProvider) Teardown(ctx context.Context, artifactID string) error {
	containerName := fmt.Sprintf("taawun_app_%s", artifactID)
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker teardown failed: %s (%w)", string(out), err)
	}
	return nil
}

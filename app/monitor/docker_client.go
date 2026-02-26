// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package monitor implements Docker log collection and parsing workflows.
package monitor

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/sk-pkg/logger"
)

// DockerManager wraps Docker SDK client operations used by collector handlers.
type DockerManager struct {
	client *client.Client
	logger *logger.Manager
}

// NewDockerClientManager creates and validates a Docker client manager.
//
// Parameters:
//   - ctx: context used for Docker ping validation.
//   - logger: logger manager retained by DockerManager.
//
// Returns:
//   - *DockerManager: initialized Docker manager.
//   - error: returned when client creation or ping fails.
func NewDockerClientManager(ctx context.Context, logger *logger.Manager) (*DockerManager, error) {
	// Initialize docker client from environment and negotiated API version.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	// Verify Docker daemon connectivity before returning manager.
	_, err = cli.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return &DockerManager{client: cli, logger: logger}, nil
}

// GetClient returns the underlying Docker SDK client.
//
// Parameters:
//   - ctx: reserved for interface consistency.
//
// Returns:
//   - *client.Client: docker SDK client instance.
func (m *DockerManager) GetClient(ctx context.Context) *client.Client {
	return m.client
}

// GetContainerInspect returns detailed inspect data for a container ID.
//
// Parameters:
//   - ctx: request context.
//   - containerID: Docker container ID.
//
// Returns:
//   - types.ContainerJSON: inspect payload.
//   - error: docker API error.
func (m *DockerManager) GetContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return m.client.ContainerInspect(ctx, containerID)
}

// ContainerList lists containers using Docker list options.
//
// Parameters:
//   - ctx: request context.
//   - options: docker list options.
//
// Returns:
//   - []types.Container: matched containers.
//   - error: docker API error.
func (m *DockerManager) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	return m.client.ContainerList(ctx, options)
}

// ContainerLogs opens a reader stream for container logs.
//
// Parameters:
//   - ctx: request context.
//   - containerID: Docker container ID.
//   - options: log streaming options.
//
// Returns:
//   - io.ReadCloser: streaming reader.
//   - error: docker API error.
func (m *DockerManager) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	return m.client.ContainerLogs(ctx, containerID, options)
}

// Events subscribes to Docker daemon events.
//
// Parameters:
//   - ctx: request context.
//   - options: event filter options.
//
// Returns:
//   - <-chan events.Message: event stream channel.
//   - <-chan error: asynchronous error channel.
func (m *DockerManager) Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
	return m.client.Events(ctx, options)
}

// GetContainerInfo resolves container name or ID depending on identifier type.
//
// Parameters:
//   - ctx: request context.
//   - containerIdentifier: source name or ID value to resolve.
//   - identifierType: resolve mode, either "name" or "id".
//
// Returns:
//   - string: resolved identifier target.
//   - error: resolve error or not-found error.
func (m *DockerManager) GetContainerInfo(ctx context.Context, containerIdentifier, identifierType string) (string, error) {
	switch identifierType {
	case "id":
		filter := filters.NewArgs()
		filter.Add("name", containerIdentifier)
		containers, err := m.ContainerList(ctx, container.ListOptions{All: true, Filters: filter})
		if err != nil {
			return "", err
		}

		for _, c := range containers {
			for _, name := range c.Names {
				if strings.TrimPrefix(name, "/") == containerIdentifier {
					return c.ID, nil
				}
			}
		}
	case "name":
		containerJSON, err := m.GetContainerInspect(ctx, containerIdentifier)
		if err != nil {
			return "", err
		}
		if containerJSON.Name != "" {
			return strings.TrimPrefix(containerJSON.Name, "/"), nil
		}
	default:
		return "", fmt.Errorf("unsupported identifier type: %s", identifierType)
	}

	return "", fmt.Errorf("container %s not found", containerIdentifier)
}

// getContainerState returns runtime state text for a container.
//
// Parameters:
//   - ctx: request context.
//   - containerID: Docker container ID.
//
// Returns:
//   - string: container state such as running, exited, or paused.
//   - error: docker API error.
func (m *DockerManager) getContainerState(ctx context.Context, containerID string) (string, error) {
	// Reuse inspect API to obtain current container state.
	containerJSON, err := m.GetContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}

	return containerJSON.State.Status, nil
}

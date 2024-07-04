package monitor

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"github.com/sk-pkg/logger"
)

// DockerManager 结构体，封装 Docker 客户端和日志管理器
type DockerManager struct {
	client *client.Client
	logger *logger.Manager
}

// NewDockerClientManager 创建 Docker 客户端管理器
func NewDockerClientManager(ctx context.Context, logger *logger.Manager) (*DockerManager, error) {
	// 初始化 Docker 客户端
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	// 测试 Docker 客户端是否正常
	_, err = cli.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return &DockerManager{client: cli, logger: logger}, nil
}

// GetClient 获取 Docker 客户端
func (m *DockerManager) GetClient(ctx context.Context) *client.Client {
	return m.client
}

// GetContainerInspect 获取容器详情
func (m *DockerManager) GetContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return m.client.ContainerInspect(ctx, containerID)
}

// ContainerList 列出所有容器
func (m *DockerManager) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	return m.client.ContainerList(ctx, options)
}

// ContainerLogs 获取日志
func (m *DockerManager) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	return m.client.ContainerLogs(ctx, containerID, options)
}

// Events 获取 Docker 事件
func (m *DockerManager) Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
	return m.client.Events(ctx, options)
}

// GetContainerInfo 获取容器信息
// identifierType: 容器标识符类型, 可选值为 "name" 或 "id"
func (m *DockerManager) GetContainerInfo(ctx context.Context, containerIdentifier, identifierType string) (string, error) {
	containers, err := m.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}

	for _, c := range containers {
		switch identifierType {
		case "id":
			for _, name := range c.Names {
				if strings.TrimPrefix(name, "/") == containerIdentifier {
					return c.ID, nil
				}
			}
		case "name":
			if c.ID == containerIdentifier {
				if len(c.Names) > 0 {
					return strings.TrimPrefix(c.Names[0], "/"), nil
				}
			}
		}
	}

	return "", fmt.Errorf("container %s not found", containerIdentifier)
}

// containerMatchesName 检查容器名称是否在列表中
func (m *DockerManager) containerMatchesName(ctx context.Context, containerNameList []string, containerID string) bool {
	// 遍历容器名称列表，检查是否匹配
	for _, containerName := range containerNameList {
		id, err := m.GetContainerInfo(ctx, containerName, "id")
		if err == nil && id == containerID {
			return true
		}
	}

	return false
}

// getContainerState 获取容器状态
func (m *DockerManager) getContainerState(ctx context.Context, containerID string) (string, error) {
	// 获取容器的状态
	containerJSON, err := m.GetContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}

	return containerJSON.State.Status, nil
}

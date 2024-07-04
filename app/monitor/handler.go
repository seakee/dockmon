package monitor

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/seakee/dockmon/app/pkg/trace"
	"github.com/seakee/dockmon/app/service/collector"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/redis"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type (
	// handler 结构体，负责日志收集工作
	handler struct {
		logger           *logger.Manager
		redis            *redis.Manager
		dockerManager    *DockerManager
		service          collector.LogService
		activeContainers *activeContainers
		unstructuredLogs *unstructuredLogs
		traceID          *trace.ID
		configs          *Config
	}

	// Config 监视器配置
	Config struct {
		ContainerNames           []string
		TimeLayout               []string
		UnstructuredLogLineFlags []string
	}

	// Handler 接口
	Handler interface {
		Start(ctx context.Context)
	}

	// activeContainers 用于记录当前活跃的容器
	activeContainers struct {
		mu      sync.RWMutex
		entries map[string]bool
	}

	// unstructuredLogs 用于暂存未结构化的日志
	unstructuredLogs struct {
		mu      sync.RWMutex
		entries map[string]*unstructuredLogBuffer
	}

	// unstructuredLogBuffer 结构体，用于暂存未结构化的日志
	unstructuredLogBuffer struct {
		containerID   string
		containerName string
		logs          []string
		logTime       string
	}
)

// New 创建日志收集器
func New(ctx context.Context, db *gorm.DB,
	logger *logger.Manager,
	redis *redis.Manager,
	config *Config,
	traceID *trace.ID) (Handler, error) {
	// 初始化 Docker 客户端
	dcm, err := NewDockerClientManager(ctx, logger)
	if err != nil {
		return nil, err
	}

	// 创建并返回日志收集器
	return &handler{
		logger:           logger,
		redis:            redis,
		dockerManager:    dcm,
		service:          collector.NewLogService(db, redis, logger),
		activeContainers: &activeContainers{entries: make(map[string]bool)},
		unstructuredLogs: &unstructuredLogs{entries: make(map[string]*unstructuredLogBuffer)},
		traceID:          traceID,
		configs:          config,
	}, nil
}

// Start 启动日志收集
func (h *handler) Start(ctx context.Context) {
	for _, containerName := range h.configs.ContainerNames {
		// 根据容器名称获取容器 ID
		containerID, err := h.dockerManager.GetContainerInfo(ctx, containerName, "id")
		if err != nil {
			h.logger.Error(ctx, "获取容器 ID 失败", zap.String("containerName", containerName), zap.Error(err))
			continue
		}

		// 启动一个 goroutine 来收集日志
		go h.collectLogs(ctx, containerID, containerName)
	}

	// 监听 Docker 事件
	go h.watchDockerEvents(ctx, h.configs.ContainerNames)
	// 启动定期清理
	go h.periodicCleanUp(ctx)
}

// 定期清理长时间未活动的 goroutine
func (h *handler) periodicCleanUp(ctx context.Context) {
	ticker := time.NewTicker(time.Hour) // 每小时执行一次清理
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.logger.Info(ctx, "定期清理开始")

			h.activeContainers.mu.Lock()

			for containerID := range h.activeContainers.entries {
				// 检查容器是否仍然在运行
				containerJSON, err := h.dockerManager.client.ContainerInspect(ctx, containerID)
				if err != nil {
					h.logger.Error(ctx, "获取容器信息失败 during clean-up", zap.String("containerID", containerID), zap.Error(err))
					continue
				}

				// 如果容器未运行，则从 activeContainers 中删除
				if !containerJSON.State.Running {
					delete(h.activeContainers.entries, containerID)
					h.logger.Info(ctx, "清理未活动的容器日志收集", zap.String("containerID", containerID))
				}
			}

			h.activeContainers.mu.Unlock()

			h.logger.Info(ctx, "定期清理结束")
		case <-ctx.Done():
			h.logger.Info(ctx, "停止定期清理")
			return
		}
	}
}

// watchDockerEvents 监听 Docker 事件
func (h *handler) watchDockerEvents(ctx context.Context, containerNameList []string) {
	h.logger.Info(ctx, "开始监听 Docker 事件")

	// 设置过滤器以监听容器事件
	filter := filters.NewArgs()
	filter.Add("type", "container")
	msgs, errs := h.dockerManager.Events(ctx, events.ListOptions{Filters: filter})

	for {
		select {
		case msg := <-msgs:
			// 获取容器名称
			containerName, err := h.dockerManager.GetContainerInfo(ctx, msg.Actor.ID, "name")
			if err != nil {
				h.logger.Error(ctx, "获取容器名称失败", zap.Error(err))
				continue
			}

			h.logger.Info(ctx, fmt.Sprintf("收到 %s 事件", msg.Action), zap.String("name", containerName))

			switch msg.Action {
			case "start":
				if h.dockerManager.containerMatchesName(ctx, containerNameList, msg.Actor.ID) {
					go h.collectLogs(ctx, msg.Actor.ID, containerName)
				}
			}
		case err := <-errs:
			h.logger.Error(ctx, "监听 Docker 事件失败", zap.Error(err))
			return
		case <-ctx.Done():
			h.logger.Info(ctx, "停止监听 Docker 事件")
			return
		}
	}
}

// collectLogs 收集指定容器的日志
func (h *handler) collectLogs(ctx context.Context, containerID, containerName string) {
	ctx = context.WithValue(ctx, logger.TraceIDKey, h.traceID.New())

	h.unstructuredLogs.mu.Lock()
	h.unstructuredLogs.entries[containerID] = &unstructuredLogBuffer{
		containerID:   containerID,
		containerName: containerName,
		logs:          make([]string, 0),
	}
	h.unstructuredLogs.mu.Unlock()

	// 检查是否已经在收集指定容器的日志
	h.activeContainers.mu.RLock()
	if h.activeContainers.entries[containerID] {
		h.activeContainers.mu.RUnlock()
		h.logger.Info(ctx, "容器日志已经在收集中", zap.String("containerName", containerName))
		return
	}
	h.activeContainers.mu.RUnlock()

	// 设置状态为正在收集日志
	h.activeContainers.mu.Lock()
	h.activeContainers.entries[containerID] = true
	h.activeContainers.mu.Unlock()

	defer func() {
		h.activeContainers.mu.Lock()
		delete(h.activeContainers.entries, containerID)
		h.activeContainers.mu.Unlock()
	}() // 收集结束后清除状态

	// 创建带有取消功能的上下文
	logCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 确保只执行一次清理操作
	var once sync.Once

	// 启动 goroutine 监控容器生命周期，如果容器停止则取消日志收集上下文
	go func() {
		// 获取容器的状态
		state, err := h.dockerManager.getContainerState(ctx, containerID)
		// 如果容器状态为 running 则启动状态监视
		if state == "running" && err == nil {
			if ok := h.monitorContainer(logCtx, containerID, containerName); !ok {
				once.Do(cancel)
			}
		}
	}()

	h.logger.Info(logCtx, "开始收集容器日志", zap.String("containerName", containerName))

	// 配置获取容器日志的选项
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "all",
		Timestamps: true,
	}

	// 获取上次日志收集的时间戳，并设置 `Since` 选项
	if lastTimestamp, ok := h.getLastLogTimestamp(ctx, containerName); ok {
		options.Since = lastTimestamp
	}

	// 调用 Docker API 获取容器日志
	reader, err := h.dockerManager.ContainerLogs(ctx, containerID, options)
	if err != nil {
		h.logger.Error(ctx, "获取容器日志失败", zap.String("containerName", containerName), zap.Error(err))
		return
	}
	defer reader.Close()

	// 创建一个 bufio.Scanner 来逐行读取日志
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Bytes()

		// 处理日志行，过滤掉前8个字符的不可见字符
		if len(line) > 8 {
			resultString := string(line)
			if h.containsUnprintableCharacters(line[:8]) {
				resultString = string(line[8:])
			}

			// 提取时间戳和日志行
			logTime, logLine := h.extractLogTimestamp(resultString)
			// 处理日志行
			h.processLogLine(logCtx, logTime, logLine, containerID, containerName)
			// 更新上次日志收集的时间戳
			h.updateLastLogTimestamp(ctx, containerName, logTime)
		}
	}

	// 检查扫描过程中的错误
	if err = scanner.Err(); err != nil {
		h.logger.Error(
			logCtx, "扫描容器日志失败",
			zap.String("containerName", containerName),
			zap.Error(err),
		)
	}

	// 检查是否有未处理的未结构化日志行
	h.unstructuredLogs.mu.RLock()
	logs, ok := h.unstructuredLogs.entries[containerID]
	h.unstructuredLogs.mu.RUnlock()
	if ok && len(logs.logs) > 0 {
		h.processUnstructuredLog(ctx, containerID)
	}

	h.logger.Info(ctx, "容器日志收集结束", zap.String("containerName", containerName))

	// 确保取消函数在错误情况下也被调用
	once.Do(cancel)
}

// 检查是否包含不可打印字符
func (h *handler) containsUnprintableCharacters(s []byte) bool {
	for len(s) > 0 {
		r, size := utf8.DecodeRune(s)
		if r == utf8.RuneError && size == 1 {
			return true
		}
		if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			return true
		}
		s = s[size:]
	}
	return false
}

// extractLogTimestamp 提取时间戳并返回日志行
func (h *handler) extractLogTimestamp(logLine string) (string, string) {
	parts := strings.SplitN(logLine, " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", logLine
}

// getLastLogTimestamp 获取上次日志收集的时间戳
func (h *handler) getLastLogTimestamp(ctx context.Context, containerName string) (string, bool) {
	// 从 Redis 中检索时间戳
	timestamp, err := h.redis.GetString(containerName + ":lastTimestamp")
	if err != nil {
		h.logger.Error(ctx, "获取容器时间戳失败", zap.String("containerName", containerName), zap.Error(err))
		return "", false
	}

	return timestamp, true
}

// updateLastLogTimestamp 更新上次日志收集的时间戳
func (h *handler) updateLastLogTimestamp(ctx context.Context, containerName, timestamp string) {
	// 更新到当前时间戳并存储
	err := h.redis.SetString(containerName+":lastTimestamp", timestamp, 0)
	if err != nil {
		h.logger.Error(ctx, "更新容器时间戳失败", zap.String("containerName", containerName), zap.Error(err))
		return
	}
}

// 监控容器的生命周期，若容器退出则返回 false
func (h *handler) monitorContainer(ctx context.Context, containerID, containerName string) bool {
	for {
		// 获取容器的状态
		state, err := h.dockerManager.getContainerState(ctx, containerID)
		if err != nil {
			h.logger.Error(ctx, "获取容器状态失败", zap.String("containerName", containerName), zap.Error(err))

			return false
		}

		select {
		case <-ctx.Done():
			return false
		default:
			// 检查容器是否正在运行
			if state != "running" {
				h.logger.Info(ctx, "容器已停止", zap.String("containerName", containerName))

				return false
			}
		}

		// 每 5 秒检查一次容器状态
		time.Sleep(5 * time.Second)
	}
}

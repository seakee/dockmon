// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

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
	// handler implements container log collection and parsing workflows.
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

	// Config contains monitor runtime configuration.
	Config struct {
		MonitoredContainers      *MonitoredContainers
		TimeLayout               []string
		UnstructuredLogLineFlags []string
	}

	// MonitoredContainers tracks include/exclude container sets.
	MonitoredContainers struct {
		Names    []string // Monitored container names.
		Ids      []string // Resolved monitored container IDs.
		BlockIDs []string // Container IDs excluded from monitoring.
		mu       sync.RWMutex
	}

	// Handler defines monitor lifecycle operations.
	Handler interface {
		Start(ctx context.Context)
	}

	// activeContainers tracks containers currently being collected.
	activeContainers struct {
		mu      sync.RWMutex
		entries map[string]bool
	}

	// unstructuredLogs buffers multiline unstructured logs by container.
	unstructuredLogs struct {
		mu      sync.RWMutex
		entries map[string]*unstructuredLogBuffer
	}

	// unstructuredLogBuffer stores temporary log parsing state per container.
	unstructuredLogBuffer struct {
		containerID   string
		containerName string
		logs          []string
		logTime       string
	}
)

// New creates a Docker log collector handler.
//
// Parameters:
//   - ctx: context used for Docker client setup.
//   - db: database client for log persistence.
//   - logger: logger manager used by collector.
//   - redis: redis manager for timestamp checkpoints.
//   - config: monitor runtime configuration.
//   - traceID: trace ID generator for collector logs.
//
// Returns:
//   - Handler: initialized collector instance.
//   - error: returned when Docker client initialization fails.
func New(ctx context.Context, db *gorm.DB,
	logger *logger.Manager,
	redis *redis.Manager,
	config *Config,
	traceID *trace.ID) (Handler, error) {
	// Build docker client manager before collector startup.
	dcm, err := NewDockerClientManager(ctx, logger)
	if err != nil {
		return nil, err
	}

	// Wire service and internal state containers.
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

// Start begins initial log collection and background event watchers.
//
// Parameters:
//   - ctx: parent context for collector goroutines.
//
// Returns:
//   - None.
func (h *handler) Start(ctx context.Context) {
	h.configs.MonitoredContainers.mu.Lock()
	for _, containerName := range h.configs.MonitoredContainers.Names {
		// Resolve ID from configured container name.
		containerID, err := h.dockerManager.GetContainerInfo(ctx, containerName, "id")
		if err != nil {
			h.logger.Error(ctx, "failed to resolve container ID", zap.String("containerName", containerName), zap.Error(err))
			continue
		}

		if !inSlice(containerID, h.configs.MonitoredContainers.Ids) {
			h.configs.MonitoredContainers.Ids = append(h.configs.MonitoredContainers.Ids, containerID)
		}

		// Start one log collection worker for each configured container.
		go h.collectLogs(ctx, containerID, containerName)
	}
	h.configs.MonitoredContainers.mu.Unlock()

	// Watch docker events for dynamic container lifecycle changes.
	go h.watchDockerEvents(ctx)
	// Start periodic cleanup for stale container workers.
	go h.periodicCleanUp(ctx)
}

// inSlice reports whether a value exists in a string slice.
//
// Parameters:
//   - value: item to search.
//   - slice: candidate collection.
//
// Returns:
//   - bool: true when value exists.
func inSlice(value string, slice []string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}

	return false
}

// removeFromSlice removes all occurrences of target from a slice.
//
// Parameters:
//   - slice: source slice.
//   - target: value to remove.
//
// Returns:
//   - []string: filtered slice sharing same backing array.
func removeFromSlice(slice []string, target string) []string {
	result := slice[:0]
	for _, item := range slice {
		if item != target {
			result = append(result, item)
		}
	}

	return result
}

// ensureMonitoredContainerID adds container ID to monitor list and removes it
// from block list.
//
// Parameters:
//   - containerID: container ID to ensure.
//
// Returns:
//   - None.
func (h *handler) ensureMonitoredContainerID(containerID string) {
	h.configs.MonitoredContainers.mu.Lock()
	defer h.configs.MonitoredContainers.mu.Unlock()

	if !inSlice(containerID, h.configs.MonitoredContainers.Ids) {
		h.configs.MonitoredContainers.Ids = append(h.configs.MonitoredContainers.Ids, containerID)
	}

	h.configs.MonitoredContainers.BlockIDs = removeFromSlice(h.configs.MonitoredContainers.BlockIDs, containerID)
}

// cleanupContainerState clears in-memory state for a container.
//
// Parameters:
//   - containerID: container ID to clean.
//
// Returns:
//   - None.
func (h *handler) cleanupContainerState(containerID string) {
	h.activeContainers.mu.Lock()
	delete(h.activeContainers.entries, containerID)
	h.activeContainers.mu.Unlock()

	h.unstructuredLogs.mu.Lock()
	delete(h.unstructuredLogs.entries, containerID)
	h.unstructuredLogs.mu.Unlock()

	h.configs.MonitoredContainers.mu.Lock()
	h.configs.MonitoredContainers.Ids = removeFromSlice(h.configs.MonitoredContainers.Ids, containerID)
	h.configs.MonitoredContainers.BlockIDs = removeFromSlice(h.configs.MonitoredContainers.BlockIDs, containerID)
	h.configs.MonitoredContainers.mu.Unlock()
}

// isMonitoredContainer checks whether a container should be monitored.
//
// Parameters:
//   - ctx: trace-aware context for logs.
//   - containerIdentifier: container name or ID.
//   - identifierType: identifier type, either "name" or "id".
//
// Returns:
//   - bool: true when container should be monitored.
func (h *handler) isMonitoredContainer(ctx context.Context, containerIdentifier, identifierType string) bool {
	if identifierType == "id" {
		h.configs.MonitoredContainers.mu.RLock()
		// Fast path: skip IDs explicitly blocked.
		if inSlice(containerIdentifier, h.configs.MonitoredContainers.BlockIDs) {
			h.configs.MonitoredContainers.mu.RUnlock()
			return false
		}

		// Fast path: allow IDs already known as monitored.
		if inSlice(containerIdentifier, h.configs.MonitoredContainers.Ids) {
			h.configs.MonitoredContainers.mu.RUnlock()
			return true
		}

		h.configs.MonitoredContainers.mu.RUnlock()

		name, err := h.dockerManager.GetContainerInfo(ctx, containerIdentifier, "name")
		if err != nil {
			if isContainerNotFoundError(err) {
				h.logger.Warn(
					ctx, "container not found, skip monitor check",
					zap.String("containerID", containerIdentifier),
					zap.Error(err),
				)
				h.cleanupContainerState(containerIdentifier)
				return false
			}
			if isContextCanceledError(err) {
				h.logger.Info(ctx, "container name lookup canceled", zap.String("containerID", containerIdentifier), zap.Error(err))
				return false
			}
			h.logger.Error(ctx, "failed to resolve container name", zap.String("containerID", containerIdentifier), zap.Error(err))
			return false
		}

		h.configs.MonitoredContainers.mu.Lock()
		defer h.configs.MonitoredContainers.mu.Unlock()

		if inSlice(name, h.configs.MonitoredContainers.Names) {
			if !inSlice(containerIdentifier, h.configs.MonitoredContainers.Ids) {
				h.configs.MonitoredContainers.Ids = append(h.configs.MonitoredContainers.Ids, containerIdentifier)
			}
			h.configs.MonitoredContainers.BlockIDs = removeFromSlice(h.configs.MonitoredContainers.BlockIDs, containerIdentifier)
			return true
		}

		if !inSlice(containerIdentifier, h.configs.MonitoredContainers.BlockIDs) {
			h.configs.MonitoredContainers.BlockIDs = append(h.configs.MonitoredContainers.BlockIDs, containerIdentifier)
		}
		return false
	}

	if identifierType == "name" {
		h.configs.MonitoredContainers.mu.RLock()
		defer h.configs.MonitoredContainers.mu.RUnlock()
		if inSlice(containerIdentifier, h.configs.MonitoredContainers.Names) {
			return true
		}
	}

	return false
}

// periodicCleanUp periodically removes stale container collection state.
//
// Parameters:
//   - ctx: parent context controlling cleanup lifecycle.
//
// Returns:
//   - None.
func (h *handler) periodicCleanUp(ctx context.Context) {
	ticker := time.NewTicker(time.Hour) // Run cleanup hourly.
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.logger.Info(ctx, "start cleaning inactive container collectors")

			h.activeContainers.mu.RLock()
			containerIDs := make([]string, 0, len(h.activeContainers.entries))
			for containerID := range h.activeContainers.entries {
				containerIDs = append(containerIDs, containerID)
			}
			h.activeContainers.mu.RUnlock()

			for _, containerID := range containerIDs {
				// Check whether the container is still running.
				containerJSON, err := h.dockerManager.client.ContainerInspect(ctx, containerID)
				if err != nil {
					if isContainerNotFoundError(err) {
						h.logger.Info(ctx, "container not found, cleaning collector state", zap.String("containerID", containerID))
						h.cleanupContainerState(containerID)
						continue
					}
					if isContextCanceledError(err) {
						h.logger.Info(ctx, "cleanup task canceled", zap.String("containerID", containerID), zap.Error(err))
						continue
					}
					h.logger.Error(ctx, "failed to inspect container during cleanup", zap.String("containerID", containerID), zap.Error(err))
					continue
				}

				// Remove collector state for stopped containers.
				if !containerJSON.State.Running {
					h.cleanupContainerState(containerID)
					h.logger.Info(ctx, "cleaned inactive container collector", zap.String("containerID", containerID))
				}
			}

			h.logger.Info(ctx, "finished cleaning inactive container collectors")
		case <-ctx.Done():
			h.logger.Info(ctx, "stop inactive container collector cleanup")
			return
		}
	}
}

// watchDockerEvents watches Docker container lifecycle events.
//
// Parameters:
//   - ctx: parent context controlling watcher lifecycle.
//
// Returns:
//   - None.
func (h *handler) watchDockerEvents(ctx context.Context) {
	h.logger.Info(ctx, "start watching Docker events")

	// Filter event stream to container events only.
	filter := filters.NewArgs()
	filter.Add("type", "container")
	msgs, errs := h.dockerManager.Events(ctx, events.ListOptions{Filters: filter})

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				h.logger.Warn(ctx, "Docker event channel closed")
				return
			}

			containerID := msg.Actor.ID
			containerName := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")

			// Prefer event-provided name to avoid noisy not-found lookups.
			if containerName != "" {
				if !h.isMonitoredContainer(ctx, containerName, "name") {
					continue
				}
				h.ensureMonitoredContainerID(containerID)
			} else if !h.isMonitoredContainer(ctx, containerID, "id") {
				continue
			} else {
				// Fallback path: resolve name via Docker API.
				name, err := h.dockerManager.GetContainerInfo(ctx, containerID, "name")
				if err != nil {
					if isContainerNotFoundError(err) {
						h.logger.Warn(
							ctx, "container not found, ignore Docker event",
							zap.String("action", string(msg.Action)),
							zap.String("containerID", containerID),
							zap.Error(err),
						)
						h.cleanupContainerState(containerID)
						continue
					}
					if isContextCanceledError(err) {
						h.logger.Info(
							ctx, "container name lookup canceled",
							zap.String("action", string(msg.Action)),
							zap.String("containerID", containerID),
							zap.Error(err),
						)
						continue
					}
					h.logger.Error(ctx, "failed to resolve container name", zap.String("containerID", containerID), zap.Error(err))
					continue
				}
				containerName = name
			}

			h.logger.Info(ctx, fmt.Sprintf("received %s event", msg.Action), zap.String("name", containerName))

			switch msg.Action {
			case "start":
				go h.collectLogs(ctx, containerID, containerName)
			case "stop", "die", "destroy":
				h.cleanupContainerState(containerID)
			}
		case err, ok := <-errs:
			if !ok {
				h.logger.Warn(ctx, "Docker event error channel closed")
				return
			}
			if isContextCanceledError(err) {
				h.logger.Info(ctx, "Docker event watcher finished", zap.Error(err))
				return
			}
			h.logger.Error(ctx, "Docker event watcher failed", zap.Error(err))
			return
		case <-ctx.Done():
			h.logger.Info(ctx, "stop watching Docker events")
			return
		}
	}
}

// collectLogs streams, parses, and stores logs for one container.
//
// Parameters:
//   - ctx: parent context of current collector lifecycle.
//   - containerID: Docker container ID.
//   - containerName: human-readable container name.
//
// Returns:
//   - None.
//
// Behavior:
//   - Ensures single active collector per container.
//   - Reads logs with timestamps and incremental since checkpoint.
//   - Persists parsed entries and updates Redis cursor.
func (h *handler) collectLogs(ctx context.Context, containerID, containerName string) {
	ctx = context.WithValue(ctx, logger.TraceIDKey, h.traceID.New())

	// Skip duplicate collector startup for the same container.
	h.activeContainers.mu.RLock()
	if h.activeContainers.entries[containerID] {
		h.activeContainers.mu.RUnlock()
		h.logger.Info(ctx, "container logs are already being collected", zap.String("containerName", containerName))
		return
	}
	h.activeContainers.mu.RUnlock()

	h.unstructuredLogs.mu.Lock()
	h.unstructuredLogs.entries[containerID] = &unstructuredLogBuffer{
		containerID:   containerID,
		containerName: containerName,
		logs:          make([]string, 0),
	}
	h.unstructuredLogs.mu.Unlock()

	// Mark container as active before opening Docker log stream.
	h.activeContainers.mu.Lock()
	h.activeContainers.entries[containerID] = true
	h.activeContainers.mu.Unlock()

	defer func() {
		h.activeContainers.mu.Lock()
		delete(h.activeContainers.entries, containerID)
		h.activeContainers.mu.Unlock()

		h.unstructuredLogs.mu.Lock()
		delete(h.unstructuredLogs.entries, containerID)
		h.unstructuredLogs.mu.Unlock()
	}() // Cleanup active state when collector exits.

	// Create a dedicated cancellable context for log stream lifecycle.
	logCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Ensure cancel runs once even across multiple exit paths.
	var once sync.Once

	// Monitor container lifecycle and stop log collection on container exit.
	go func() {
		// Check initial state before entering monitor loop.
		state, err := h.dockerManager.getContainerState(logCtx, containerID)
		// Start state monitor only when container is currently running.
		if state == "running" && err == nil {
			if ok := h.monitorContainer(logCtx, containerID, containerName); !ok {
				once.Do(cancel)
			}
		}
	}()

	h.logger.Info(logCtx, "start collecting container logs", zap.String("containerName", containerName))

	// Configure Docker log stream options.
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "all",
		Timestamps: true,
	}

	// Resume from last collected timestamp if checkpoint exists.
	if lastTimestamp, ok := h.getLastLogTimestamp(ctx, containerName); ok {
		options.Since = lastTimestamp
	}

	// Open log stream from Docker daemon.
	reader, err := h.dockerManager.ContainerLogs(logCtx, containerID, options)
	if err != nil {
		h.logger.Error(ctx, "failed to open container logs", zap.String("containerName", containerName), zap.Error(err))
		return
	}
	defer reader.Close()

	// Scan log stream line by line.
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Bytes()

		// Docker multiplexed streams may prefix 8-byte non-printable headers.
		if len(line) > 8 {
			resultString := string(line)
			if h.containsUnprintableCharacters(line[:8]) {
				resultString = string(line[8:])
			}

			// Split docker timestamp from message payload.
			logTime, logLine := h.extractLogTimestamp(resultString)
			// Parse and store log line.
			h.processLogLine(logCtx, logTime, logLine, containerID, containerName)
			// Update checkpoint for incremental collection.
			h.updateLastLogTimestamp(ctx, containerName, logTime)
		}
	}

	// Report scanner errors after stream loop exits.
	if err = scanner.Err(); err != nil {
		h.logger.Error(
			logCtx, "failed to scan container logs",
			zap.String("containerName", containerName),
			zap.Error(err),
		)
	}

	// Flush pending buffered unstructured logs.
	h.unstructuredLogs.mu.RLock()
	logs, ok := h.unstructuredLogs.entries[containerID]
	h.unstructuredLogs.mu.RUnlock()
	if ok && len(logs.logs) > 0 {
		h.processUnstructuredLog(ctx, containerID)
	}

	h.logger.Info(ctx, "container log collection finished", zap.String("containerName", containerName))

	// Ensure monitor goroutine receives cancellation on all exits.
	once.Do(cancel)
}

// containsUnprintableCharacters reports whether byte slice has control bytes.
//
// Parameters:
//   - s: byte slice to inspect.
//
// Returns:
//   - bool: true when non-printable, non-space runes exist.
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

// extractLogTimestamp splits a docker log line into timestamp and message.
//
// Parameters:
//   - logLine: raw line emitted by Docker log stream.
//
// Returns:
//   - string: timestamp part when present.
//   - string: message part without timestamp.
func (h *handler) extractLogTimestamp(logLine string) (string, string) {
	parts := strings.SplitN(logLine, " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", logLine
}

// getLastLogTimestamp reads last collected timestamp from Redis.
//
// Parameters:
//   - ctx: trace-aware context for logs.
//   - containerName: container name used as Redis key prefix.
//
// Returns:
//   - string: timestamp value.
//   - bool: true when timestamp is available.
func (h *handler) getLastLogTimestamp(ctx context.Context, containerName string) (string, bool) {
	// Read incremental cursor from Redis.
	timestamp, err := h.redis.GetString(containerName + ":lastTimestamp")
	if err != nil {
		h.logger.Error(ctx, "failed to get container timestamp", zap.String("containerName", containerName), zap.Error(err))
		return "", false
	}

	return timestamp, true
}

// updateLastLogTimestamp writes latest processed timestamp to Redis.
//
// Parameters:
//   - ctx: trace-aware context for logs.
//   - containerName: container name used as Redis key prefix.
//   - timestamp: latest processed timestamp.
//
// Returns:
//   - None.
func (h *handler) updateLastLogTimestamp(ctx context.Context, containerName, timestamp string) {
	// Persist incremental cursor for next collector startup.
	err := h.redis.SetString(containerName+":lastTimestamp", timestamp, 0)
	if err != nil {
		h.logger.Error(ctx, "failed to update container timestamp", zap.String("containerName", containerName), zap.Error(err))
		return
	}
}

// monitorContainer polls container state and stops when it is no longer running.
//
// Parameters:
//   - ctx: monitor context controlling polling lifecycle.
//   - containerID: Docker container ID.
//   - containerName: human-readable container name for logs.
//
// Returns:
//   - bool: false when monitoring should stop.
func (h *handler) monitorContainer(ctx context.Context, containerID, containerName string) bool {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		// Poll current runtime state from Docker API.
		state, err := h.dockerManager.getContainerState(ctx, containerID)
		if err != nil {
			if isContextCanceledError(err) {
				h.logger.Info(ctx, "container state monitoring finished", zap.String("containerName", containerName), zap.Error(err))
				return false
			}
			if isContainerNotFoundError(err) {
				h.logger.Warn(ctx, "container not found, stop state monitoring", zap.String("containerName", containerName), zap.Error(err))
				h.cleanupContainerState(containerID)
				return false
			}
			h.logger.Error(ctx, "failed to get container state", zap.String("containerName", containerName), zap.Error(err))

			return false
		}

		// Stop monitoring and collector when container exits.
		if state != "running" {
			h.logger.Info(ctx, "container has stopped", zap.String("containerName", containerName))
			h.cleanupContainerState(containerID)
			return false
		}

		// Poll every 5 seconds to balance latency and API pressure.
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

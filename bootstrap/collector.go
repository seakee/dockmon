// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bootstrap

import (
	"context"
	"os"
	"strings"

	"github.com/seakee/dockmon/app/monitor"
	"go.uber.org/zap"
)

// startCollector initializes and starts the Docker log collector subsystem.
//
// Parameters:
//   - ctx: trace-aware context used for lifecycle logs and downstream calls.
//
// Returns:
//   - None.
//
// Behavior:
//   - Optionally appends the current app container to monitored names when
//     running inside Docker and monitor-self is enabled.
//   - Starts collector loop immediately after dependency construction.
func (a *App) startCollector(ctx context.Context) {
	// Add current container name when self-monitor mode is enabled.
	if a.Config.Collector.MonitorSelf && a.Config.System.Name != "" && a.checkIfRunningInContainer() {
		a.Config.Collector.ContainerName = append(a.Config.Collector.ContainerName, a.Config.System.Name)
	}

	handler, err := monitor.New(
		ctx,
		a.MysqlDB["dockmon"],
		a.Logger,
		a.Redis["dockmon"],
		&monitor.Config{
			UnstructuredLogLineFlags: a.Config.Collector.UnstructuredLogLineFlags,
			MonitoredContainers:      &monitor.MonitoredContainers{Names: a.Config.Collector.ContainerName},
			TimeLayout:               a.Config.Collector.TimeLayout,
		},
		a.TraceID,
	)
	if err != nil {
		a.Logger.Error(ctx, "Collector load failed", zap.Error(err))
		return
	}

	handler.Start(ctx)

	a.Logger.Info(ctx, "Collector loaded successfully")
}

// checkIfRunningInContainer returns whether the current process runs in Docker.
//
// Returns:
//   - bool: true when container runtime fingerprints are detected.
//
// Behavior:
//   - Checks environment marker, /.dockerenv, and /proc/1/cgroup.
func (a *App) checkIfRunningInContainer() bool {
	// Check the standard container runtime environment variable.
	if _, exists := os.LookupEnv("container"); exists {
		return true
	}

	// Check container-specific marker file.
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check cgroup metadata for docker hints.
	data, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "docker")
}

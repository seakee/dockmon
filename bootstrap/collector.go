package bootstrap

import (
	"context"
	"os"
	"strings"

	"github.com/seakee/dockmon/app/monitor"
	"go.uber.org/zap"
)

func (a *App) startCollector(ctx context.Context) {
	// 检查日志收集器如果运行在 docker 容器里面,并且配置文件开启了收集自身日志
	// 则将容器名称添加到要监控的容器名称列表中
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
			ContainerNames:           a.Config.Collector.ContainerName,
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

// checkIfRunningInContainer 检查程序是否在容器中运行
func (a *App) checkIfRunningInContainer() bool {
	// 检查环境变量
	if _, exists := os.LookupEnv("container"); exists {
		return true
	}

	// 检查 /.dockerenv 文件
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// 检查 /proc/1/cgroup 文件
	data, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "docker")
}

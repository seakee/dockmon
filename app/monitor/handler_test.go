// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package monitor

import (
	"context"
	"time"

	"github.com/seakee/dockmon/app/pkg/trace"
	"github.com/seakee/dockmon/app/service/collector"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/mysql"
	"github.com/sk-pkg/redis"
)

func newTestCollector() (*handler, error) {
	manager := redis.New(
		redis.WithPrefix("dockmon"),
		redis.WithAddress("redis_host"),
		redis.WithIdleTimeout(30),
		redis.WithMaxActive(100),
		redis.WithMaxIdle(30),
		redis.WithDB(0),
	)
	db, _ := mysql.New(mysql.WithConfigs(
		mysql.Config{
			User:     "db_username",
			Password: "db_password",
			Host:     "db_host",
			DBName:   "d_name",
		}),
		mysql.WithConnMaxLifetime(3*time.Hour),
		mysql.WithMaxIdleConn(10),
		mysql.WithMaxOpenConn(50),
	)
	l, _ := logger.New()

	ctx := context.Background()
	dcm, err := NewDockerClientManager(ctx, l)
	if err != nil {
		return nil, err
	}

	containerNameList := []string{"go-api"}

	timeLayout := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.000",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05.000-0700",
		"2006/01/02",
		"2006/01/02 15:04:05",
		"2006/01/02 15:04:05.000",
		"2006/01/02T15:04:05Z07:00",
		"2006/01/02T15:04:05.000Z07:00",
		"Mon Jan 2 15:04:05 MST 2006",
		"02 Jan 06 15:04 MST",
		"02 Jan 2006 15:04:05",
	}

	UnstructuredLogLineFlags := []string{
		"fatal error:",
		"[GIN-debug]",
		"[GIN-warning]",
		"panic:",
	}

	traceID := trace.NewTraceID()

	return &handler{
		logger:           l,
		redis:            manager,
		dockerManager:    dcm,
		service:          collector.NewLogService(db, manager, l),
		activeContainers: &activeContainers{entries: make(map[string]bool)},
		unstructuredLogs: &unstructuredLogs{entries: make(map[string]*unstructuredLogBuffer)},
		traceID:          traceID,
		configs: &Config{
			MonitoredContainers:      &MonitoredContainers{Names: containerNameList},
			TimeLayout:               timeLayout,
			UnstructuredLogLineFlags: UnstructuredLogLineFlags,
		},
	}, nil
}

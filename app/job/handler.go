// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package job registers scheduled background jobs.
package job

import (
	"github.com/seakee/dockmon/app/pkg/schedule"
	"github.com/sk-pkg/feishu"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/redis"
	"gorm.io/gorm"
)

// Register adds background jobs into the scheduler.
//
// Parameters:
//   - logger: logger manager for job execution logs.
//   - redis: redis clients map keyed by profile name.
//   - db: database clients map keyed by database name.
//   - feishu: optional Feishu manager for notifications.
//   - s: scheduler instance that receives registered jobs.
//
// Returns:
//   - None.
//
// Behavior:
//   - Keeps sample jobs commented out until explicitly enabled.
func Register(logger *logger.Manager, redis map[string]*redis.Manager, db map[string]*gorm.DB, feishu *feishu.Manager, s *schedule.Schedule) {
	// Monitor broadband public network IP changes
	// ipMonitor := monitor.NewIpMonitor(logger, redis["dockmon"])
	// s.AddJob("IpMonitor", ipMonitor).PerMinuit(5).WithoutOverlapping()
}

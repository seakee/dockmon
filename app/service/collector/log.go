// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package collector provides service-layer orchestration for collected logs.
package collector

import (
	"context"

	collectorModel "github.com/seakee/dockmon/app/model/collector"
	"github.com/seakee/dockmon/app/repository/collector"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/redis"
	"gorm.io/gorm"
)

type (
	// LogService defines business operations for collected logs.
	LogService interface {
		Store(ctx context.Context, log *collectorModel.Log) (int, error)
	}

	// logService is the default LogService implementation.
	logService struct {
		repo   collector.Repo
		logger *logger.Manager
		redis  *redis.Manager
	}
)

// Store persists one collected log entry.
//
// Parameters:
//   - ctx: request or task context.
//   - log: log entity to persist.
//
// Returns:
//   - int: created record ID.
//   - error: storage error.
func (l logService) Store(ctx context.Context, log *collectorModel.Log) (int, error) {
	return l.repo.CreateLog(log)
}

// NewLogService creates a LogService with repository dependencies.
//
// Parameters:
//   - db: GORM database client.
//   - redis: Redis manager.
//   - logger: logger manager.
//
// Returns:
//   - LogService: initialized service implementation.
func NewLogService(db *gorm.DB, redis *redis.Manager, logger *logger.Manager) LogService {
	return &logService{
		repo:   collector.NewLogRepo(db, redis),
		logger: logger,
		redis:  redis,
	}
}

// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package collector implements collector-domain repository access methods.
package collector

import (
	"github.com/seakee/dockmon/app/model/collector"
	"github.com/sk-pkg/redis"
	"gorm.io/gorm"
)

type (
	// Repo defines persistence operations for collected logs.
	Repo interface {
		FirstLog(*collector.Log) (*collector.Log, error)
		CreateLog(*collector.Log) (int, error)
	}

	// repo is a GORM-backed Repo implementation.
	repo struct {
		redis *redis.Manager
		db    *gorm.DB
	}
)

// CreateLog inserts a collected log record.
//
// Parameters:
//   - log: log model to persist.
//
// Returns:
//   - int: created record ID.
//   - error: insertion error.
func (r *repo) CreateLog(log *collector.Log) (int, error) {
	return log.Create(r.db)
}

// FirstLog returns the first record that matches query fields.
//
// Parameters:
//   - app: query model with filter fields.
//
// Returns:
//   - *collector.Log: matched log record.
//   - error: query error.
func (r *repo) FirstLog(app *collector.Log) (*collector.Log, error) {
	return app.First(r.db)
}

// NewLogRepo creates a log repository with shared dependencies.
//
// Parameters:
//   - db: GORM database client.
//   - redis: Redis manager.
//
// Returns:
//   - Repo: initialized repository implementation.
func NewLogRepo(db *gorm.DB, redis *redis.Manager) Repo {
	return &repo{redis: redis, db: db}
}

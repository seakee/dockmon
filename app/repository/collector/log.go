// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package collector

import (
	"github.com/seakee/dockmon/app/model/collector"
	"github.com/sk-pkg/redis"
	"gorm.io/gorm"
)

type (
	Repo interface {
		FirstLog(*collector.Log) (*collector.Log, error)
		CreateLog(*collector.Log) (int, error)
	}

	repo struct {
		redis *redis.Manager
		db    *gorm.DB
	}
)

func (r *repo) CreateLog(log *collector.Log) (int, error) {
	return log.Create(r.db)
}

func (r *repo) FirstLog(app *collector.Log) (*collector.Log, error) {
	return app.First(r.db)
}

func NewLogRepo(db *gorm.DB, redis *redis.Manager) Repo {
	return &repo{redis: redis, db: db}
}

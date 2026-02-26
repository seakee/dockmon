// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package auth implements auth-domain repository access methods.
package auth

import (
	"github.com/seakee/dockmon/app/model/auth"
	"github.com/sk-pkg/redis"
	"gorm.io/gorm"
)

type (
	// Repo defines persistence operations for auth apps.
	Repo interface {
		GetApp(*auth.App) (*auth.App, error)
		CreateApp(*auth.App) (uint, error)
		ExistAppByName(string) (bool, error)
	}

	// repo is a GORM-backed Repo implementation.
	repo struct {
		redis *redis.Manager
		db    *gorm.DB
	}
)

// ExistAppByName checks whether an app with given name exists.
//
// Parameters:
//   - name: app name to query.
//
// Returns:
//   - bool: true when at least one record exists.
//   - error: query error.
func (r *repo) ExistAppByName(name string) (exist bool, err error) {
	app := &auth.App{AppName: name}
	a, err := app.First(r.db)
	if a != nil {
		exist = true
	}

	return
}

// CreateApp persists a new app record.
//
// Parameters:
//   - app: app model to insert.
//
// Returns:
//   - uint: created record ID.
//   - error: insertion error.
func (r *repo) CreateApp(app *auth.App) (uint, error) {
	return app.Create(r.db)
}

// GetApp returns one app record that matches query fields.
//
// Parameters:
//   - app: query model with filter fields.
//
// Returns:
//   - *auth.App: matched app model.
//   - error: query error.
func (r *repo) GetApp(app *auth.App) (*auth.App, error) {
	return app.First(r.db)
}

// NewAppRepo creates a Repo backed by GORM and Redis dependencies.
//
// Parameters:
//   - db: GORM database client.
//   - redis: Redis manager.
//
// Returns:
//   - Repo: initialized repository implementation.
func NewAppRepo(db *gorm.DB, redis *redis.Manager) Repo {
	return &repo{redis: redis, db: db}
}

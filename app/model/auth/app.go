// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package auth defines persistence models for authentication domain objects.
package auth

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type App struct {
	gorm.Model

	AppName     string `gorm:"column:app_name" json:"app_name"`
	AppID       string `gorm:"column:app_id" json:"app_id"`
	AppSecret   string `gorm:"column:app_secret" json:"app_secret"`
	RedirectUri string `gorm:"column:redirect_uri" json:"redirect_uri"`
	Description string `gorm:"column:description" json:"description"`
	Status      uint8  `gorm:"column:status" json:"status"`
}

// TableName returns the database table name for App.
//
// Returns:
//   - string: physical table name in MySQL.
func (a *App) TableName() string {
	return "auth_app"
}

// First queries and returns the first app record matching non-zero struct fields.
//
// Parameters:
//   - db: GORM database client.
//
// Returns:
//   - *App: first matched app record.
//   - error: query error including gorm.ErrRecordNotFound when absent.
func (a *App) First(db *gorm.DB) (app *App, err error) {
	err = db.Where(a).First(&app).Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return app, err
}

// Create inserts the current App record into database.
//
// Parameters:
//   - db: GORM database client.
//
// Returns:
//   - uint: auto-increment primary key of inserted record.
//   - error: wrapped create error when insertion fails.
func (a *App) Create(db *gorm.DB) (id uint, err error) {
	if err = db.Create(a).Error; err != nil {
		return 0, fmt.Errorf("create failed: %w", err)
	}

	id = a.ID

	return
}

// Delete soft-deletes the current App record.
//
// Parameters:
//   - db: GORM database client.
//
// Returns:
//   - error: wrapped delete error when operation fails.
func (a *App) Delete(db *gorm.DB) (err error) {
	if err = db.Delete(a).Error; err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	return
}

// Updates updates selected fields of the current App by ID.
//
// Parameters:
//   - db: GORM database client.
//   - m: field-value map to update.
//
// Returns:
//   - error: wrapped update error when operation fails.
func (a *App) Updates(db *gorm.DB, m map[string]interface{}) (err error) {
	if err = db.Model(&App{}).Where("id = ?", a.ID).Updates(m).Error; err != nil {
		return fmt.Errorf("updates failed: %w", err)
	}
	return
}

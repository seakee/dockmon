// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package collector defines persistence models for collected container logs.
package collector

import (
	"database/sql"

	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Log struct {
	ID            int            `gorm:"primaryKey;column:id" json:"-"`
	Level         string         `gorm:"level" json:"level"`
	Time          sql.NullTime   `gorm:"time" json:"time"`
	Caller        string         `gorm:"caller" json:"caller"`
	Message       string         `gorm:"message" json:"message"`
	TraceID       string         `gorm:"trace_id" json:"trace_id"`
	ContainerID   string         `gorm:"container_id" json:"container_id"`
	ContainerName string         `gorm:"container_name" json:"container_name"`
	Extra         datatypes.JSON `gorm:"extra" json:"extra"`
}

// TableName returns the database table name for Log.
//
// Returns:
//   - string: physical table name in MySQL.
func (l *Log) TableName() string {
	return "log"
}

// First returns the first record that matches non-zero fields of Log.
//
// Parameters:
//   - db: GORM database client.
//
// Returns:
//   - *Log: first matched log record.
//   - error: query error including gorm.ErrRecordNotFound when absent.
func (l *Log) First(db *gorm.DB) (log *Log, err error) {
	err = db.Where(l).First(&log).Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return log, err
}

// Last returns the latest record that matches non-zero fields of Log.
//
// Parameters:
//   - db: GORM database client.
//
// Returns:
//   - *Log: last matched log record.
//   - error: query error including gorm.ErrRecordNotFound when absent.
func (l *Log) Last(db *gorm.DB) (log *Log, err error) {
	err = db.Where(l).Last(&log).Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return log, err
}

// Create inserts the current Log record into database.
//
// Parameters:
//   - db: GORM database client.
//
// Returns:
//   - int: auto-increment primary key of inserted record.
//   - error: wrapped create error when insertion fails.
func (l *Log) Create(db *gorm.DB) (id int, err error) {
	if err = db.Create(l).Error; err != nil {
		return 0, errors.Wrap(err, "create err")
	}

	id = l.ID

	return
}

// Delete removes the current Log record.
//
// Parameters:
//   - db: GORM database client.
//
// Returns:
//   - error: wrapped delete error when operation fails.
func (l *Log) Delete(db *gorm.DB) (err error) {
	if err = db.Delete(l).Error; err != nil {
		return errors.Wrap(err, "delete err")
	}
	return
}

// Updates updates selected fields of the current Log by ID.
//
// Parameters:
//   - db: GORM database client.
//   - m: field-value map to update.
//
// Returns:
//   - error: wrapped update error when operation fails.
func (l *Log) Updates(db *gorm.DB, m map[string]any) (err error) {
	if err = db.Model(&Log{}).Where("id = ?", l.ID).Updates(m).Error; err != nil {
		return errors.Wrap(err, "updates err")
	}
	return
}

// List returns all records matching non-zero fields of Log.
//
// Parameters:
//   - db: GORM database client.
//
// Returns:
//   - []Log: matched log list.
//   - error: query error.
func (l *Log) List(db *gorm.DB) (logs []Log, err error) {
	err = db.Where(l).Find(&logs).Error
	return
}

// ListByArgs returns logs filtered by raw query conditions and arguments.
//
// Parameters:
//   - db: GORM database client.
//   - query: SQL where expression or struct condition.
//   - args: query placeholder arguments.
//
// Returns:
//   - []Log: matched logs sorted by descending ID.
//   - error: query error.
func (l *Log) ListByArgs(db *gorm.DB, query interface{}, args ...interface{}) (logs []Log, err error) {
	err = db.Where(query, args...).Order("id desc").Find(&logs).Error
	return
}

// CountByArgs returns number of logs matching raw query conditions.
//
// Parameters:
//   - db: GORM database client.
//   - query: SQL where expression or struct condition.
//   - args: query placeholder arguments.
//
// Returns:
//   - int64: matched row count.
func (l *Log) CountByArgs(db *gorm.DB, query interface{}, args ...interface{}) (total int64) {
	db.Where(query, args...).Count(&total)
	return
}

// Count returns number of logs matching non-zero fields of Log.
//
// Parameters:
//   - db: GORM database client.
//
// Returns:
//   - int64: matched row count.
func (l *Log) Count(db *gorm.DB) (total int64) {
	db.Where(l).Count(&total)
	return
}

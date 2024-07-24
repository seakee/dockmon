// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

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

func (l *Log) TableName() string {
	return "log"
}

func (l *Log) First(db *gorm.DB) (log *Log, err error) {
	err = db.Where(l).First(&log).Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return log, err
}

func (l *Log) Last(db *gorm.DB) (log *Log, err error) {
	err = db.Where(l).Last(&log).Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return log, err
}

func (l *Log) Create(db *gorm.DB) (id int, err error) {
	if err = db.Create(l).Error; err != nil {
		return 0, errors.Wrap(err, "create err")
	}

	id = l.ID

	return
}

func (l *Log) Delete(db *gorm.DB) (err error) {
	if err = db.Delete(l).Error; err != nil {
		return errors.Wrap(err, "delete err")
	}
	return
}

func (l *Log) Updates(db *gorm.DB, m map[string]any) (err error) {
	if err = db.Model(&Log{}).Where("id = ?", l.ID).Updates(m).Error; err != nil {
		return errors.Wrap(err, "updates err")
	}
	return
}

func (l *Log) List(db *gorm.DB) (logs []Log, err error) {
	err = db.Where(l).Find(&logs).Error
	return
}

func (l *Log) ListByArgs(db *gorm.DB, query interface{}, args ...interface{}) (logs []Log, err error) {
	err = db.Where(query, args...).Order("id desc").Find(&logs).Error
	return
}

func (l *Log) CountByArgs(db *gorm.DB, query interface{}, args ...interface{}) (total int64) {
	db.Where(query, args...).Count(&total)
	return
}

func (l *Log) Count(db *gorm.DB) (total int64) {
	db.Where(l).Count(&total)
	return
}

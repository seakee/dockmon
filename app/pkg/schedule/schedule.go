// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package schedule provides a lightweight in-process scheduler with optional
// single-node locking via Redis.
package schedule

import (
	"time"

	"github.com/seakee/dockmon/app/pkg/trace"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/redis"
)

type (
	// Schedule stores registered jobs and shared dependencies.
	Schedule struct {
		Logger  *logger.Manager
		Redis   *redis.Manager
		Job     []*Job
		TraceID *trace.ID
	}
)

// New creates a scheduler instance.
//
// Parameters:
//   - logger: logger manager used by jobs.
//   - redis: redis manager used for distributed locks.
//   - traceID: trace ID generator shared by jobs.
//
// Returns:
//   - *Schedule: initialized scheduler.
//
// Example:
//
//	s := schedule.New(logger, redis, traceID)
func New(logger *logger.Manager, redis *redis.Manager, traceID *trace.ID) *Schedule {
	return &Schedule{
		Logger:  logger,
		Redis:   redis,
		Job:     make([]*Job, 0),
		TraceID: traceID,
	}
}

// AddJob registers a named job handler and returns a configurable Job object.
//
// Parameters:
//   - name: human-readable job name.
//   - handlerFunc: implementation that executes job logic.
//
// Returns:
//   - *Job: configurable job instance for run-time options.
func (s *Schedule) AddJob(name string, handlerFunc HandlerFunc) *Job {
	return s.addJob(name, handlerFunc)
}

// addJob creates and appends an internal job entry.
//
// Parameters:
//   - name: job name used in logs and lock keys.
//   - handlerFunc: schedule handler implementation.
//
// Returns:
//   - *Job: created job object.
func (s *Schedule) addJob(name string, handlerFunc HandlerFunc) *Job {
	j := &Job{
		Name:                  name,
		Logger:                s.Logger,
		Redis:                 s.Redis,
		Handler:               handlerFunc,
		EnableMultipleServers: true,
		EnableOverlapping:     true,
		RunTime:               &RunTime{Done: make(chan struct{})},
		TraceID:               s.TraceID,
	}

	s.Job = append(s.Job, j)

	return j
}

// Start launches the scheduler loop that checks job triggers every second.
//
// Returns:
//   - None.
//
// Behavior:
//   - Keeps running in a background goroutine.
//   - Invokes each job's run() method on every tick.
func (s *Schedule) Start() {
	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			for _, j := range s.Job {
				j.run()
			}
		}
	}()
}

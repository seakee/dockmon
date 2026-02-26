// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package schedule

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/seakee/dockmon/app/pkg/trace"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/redis"
	"github.com/sk-pkg/util"
	"go.uber.org/zap"
)

const (
	DailyRunType      = "daily"
	PerSecondsRunType = "seconds"
	PerMinuitRunType  = "minuit"
	PerHourRunType    = "hour"

	defaultServerLockTTL = 600 // Default lock TTL for one-server mode in seconds.
)

type (
	// Job defines one schedulable task with runtime options.
	Job struct {
		Name                  string          // Job instance name.
		Logger                *logger.Manager // Logger manager.
		Redis                 *redis.Manager  // Redis manager.
		Handler               HandlerFunc     // Job executor.
		EnableMultipleServers bool            // Whether multiple nodes can execute the job.
		EnableOverlapping     bool            // Whether overlapping runs are allowed.
		RunTime               *RunTime        // Runtime scheduling state.
		TraceID               *trace.ID
	}

	// HandlerFunc is implemented by all scheduled job handlers.
	HandlerFunc interface {
		Exec(ctx context.Context)
		Error() <-chan error
		Done() <-chan struct{}
	}

	// RunTime stores runtime trigger and lock state for one job.
	RunTime struct {
		Type          string        // Schedule trigger type.
		Time          interface{}   // Trigger value (times or duration interval).
		Locked        bool          // Local lock when overlap is disabled.
		PerTypeLocked bool          // One-time guard for interval ticker startup.
		Done          chan struct{} // Completion signal for lock renewal loop.
		RandomDelay   *RandomDelay  // Optional random delay before each run.
	}

	// RandomDelay defines random delay boundaries in seconds.
	RandomDelay struct {
		Min int
		Max int
	}
)

// WithoutOverlapping disables overlapping execution for this job instance.
//
// Returns:
//   - *Job: the same job for chained configuration.
func (j *Job) WithoutOverlapping() *Job {
	j.EnableOverlapping = false
	return j
}

// RandomDelay sets random delay range in seconds before each execution.
//
// Parameters:
//   - min: minimum delay in seconds.
//   - max: maximum delay in seconds and must be greater than min.
//
// Returns:
//   - *Job: the same job for chained configuration.
//
// Panics:
//   - Panics when max is less than min.
func (j *Job) RandomDelay(min, max int) *Job {
	if max < min {
		panic("must max > min")
	}

	j.RunTime.RandomDelay = &RandomDelay{
		Min: min,
		Max: max,
	}

	return j
}

// DailyAt schedules the job at one or more fixed local times every day.
//
// Parameters:
//   - time: one or more "HH:MM:SS" trigger points.
//
// Returns:
//   - *Job: the same job for chained configuration.
//
// Behavior:
//   - This trigger type is mutually exclusive with other trigger types.
//
// Example:
//
//	s.AddJob("sync", h).DailyAt("07:30:00", "18:00:00")
func (j *Job) DailyAt(time ...string) *Job {
	if j.RunTime.Type == "" {
		j.RunTime.Type = DailyRunType
		j.RunTime.Time = time
	}
	return j
}

// PerSeconds schedules the job at a fixed second interval.
//
// Parameters:
//   - seconds: interval in seconds.
//
// Returns:
//   - *Job: the same job for chained configuration.
//
// Behavior:
//   - This trigger type is mutually exclusive with other trigger types.
func (j *Job) PerSeconds(seconds int) *Job {
	if j.RunTime.Type == "" {
		j.RunTime.Type = PerSecondsRunType
		j.RunTime.Time = time.Duration(seconds) * time.Second
	}
	return j
}

// PerMinuit schedules the job at a fixed minute interval.
//
// Parameters:
//   - minuit: interval in minutes.
//
// Returns:
//   - *Job: the same job for chained configuration.
//
// Behavior:
//   - This trigger type is mutually exclusive with other trigger types.
func (j *Job) PerMinuit(minuit int) *Job {
	if j.RunTime.Type == "" {
		j.RunTime.Type = PerMinuitRunType
		j.RunTime.Time = time.Duration(minuit) * time.Minute
	}
	return j
}

// PerHour schedules the job at a fixed hour interval.
//
// Parameters:
//   - hour: interval in hours.
//
// Returns:
//   - *Job: the same job for chained configuration.
//
// Behavior:
//   - This trigger type is mutually exclusive with other trigger types.
func (j *Job) PerHour(hour int) *Job {
	if j.RunTime.Type == "" {
		j.RunTime.Type = PerHourRunType
		j.RunTime.Time = time.Duration(hour) * time.Hour
	}
	return j
}

// OnOneServer enables distributed locking so only one node executes the job.
//
// Returns:
//   - *Job: the same job for chained configuration.
//
// Behavior:
//   - Requires Redis availability to acquire and renew lock keys.
func (j *Job) OnOneServer() *Job {
	j.EnableMultipleServers = false
	return j
}

// runWithRecover wraps one execution cycle with panic recovery.
//
// Returns:
//   - None.
func (j *Job) runWithRecover() {
	ctx := context.WithValue(context.Background(), logger.TraceIDKey, j.TraceID.New())

	defer func() {
		// Keep scheduler alive even when job handler panics.
		if r := recover(); r != nil {
			j.Logger.Error(ctx, "job has a panic error", zap.Any("error", r))
		}
	}()

	j.handler(ctx)
}

// run checks trigger conditions and dispatches job execution accordingly.
//
// Returns:
//   - None.
//
// Behavior:
//   - Daily mode compares current local time against configured points.
//   - Interval modes start one ticker once and execute repeatedly.
func (j *Job) run() {
	switch j.RunTime.Type {
	case DailyRunType:
		// Execute when any configured daily time matches current second.
		times := j.RunTime.Time.([]string)
		for _, t := range times {
			if time.Now().Format("15:04:05") == t {
				go j.runWithRecover()
			}
		}
	case PerSecondsRunType, PerMinuitRunType, PerHourRunType:
		// Start interval ticker only once for this job.
		if j.RunTime.PerTypeLocked {
			return
		}

		j.RunTime.PerTypeLocked = true
		go func() {
			ticker := time.NewTicker(j.RunTime.Time.(time.Duration))
			for range ticker.C {
				go j.runWithRecover()
			}
		}()
	}
}

// handler executes one job run with overlap and distributed-lock controls.
//
// Parameters:
//   - ctx: trace-aware context for logs.
//
// Returns:
//   - None.
//
// Behavior:
//   - Applies local overlap lock when configured.
//   - Optionally acquires/renews one-server Redis lock.
//   - Starts async listener for handler error/done channels.
func (j *Job) handler(ctx context.Context) {
	if !j.EnableOverlapping {
		// Reject new execution while previous run is still active.
		if j.RunTime.Locked {
			return
		}
		j.RunTime.Locked = true
	}

	if !j.EnableMultipleServers {
		// Ensure exactly one node executes by acquiring distributed lock.
		if !j.lock("Server", defaultServerLockTTL, false) {
			j.RunTime.Locked = false
			return
		}

		go j.renewalServerLock(ctx)
	}

	// Optionally add jitter to reduce synchronized load spikes.
	j.randomDelay()

	j.Logger.Info(ctx, util.SpliceStr("The scheduled job: ", j.Name, " starts execution."))

	// Consume handler channels and perform cleanup when execution completes.
	go func(ctx context.Context) {
	Exit:
		for {
			select {
			case err := <-j.Handler.Error():
				if err != nil {
					j.Logger.Error(ctx, fmt.Sprintf("An error occurred while executing the %s.", j.Name), zap.Error(err))
				}
			case <-j.Handler.Done():
				// Release distributed lock renewal loop for this execution.
				if !j.EnableMultipleServers {
					j.RunTime.Done <- struct{}{}
				}

				j.RunTime.Locked = false

				j.Logger.Info(ctx, util.SpliceStr("The scheduled job: ", j.Name, " has done."))

				break Exit
			}
		}
	}(ctx)

	j.Handler.Exec(ctx)
}

// randomDelay sleeps for a random duration within configured bounds.
//
// Returns:
//   - None.
func (j *Job) randomDelay() {
	if j.RunTime.RandomDelay == nil {
		return
	}

	source := rand.NewSource(time.Now().UnixNano())
	generator := rand.New(source)

	delay := generator.Intn(j.RunTime.RandomDelay.Max) + j.RunTime.RandomDelay.Min
	time.Sleep(time.Duration(delay) * time.Second)
}

// lock acquires or renews a Redis lock for this job.
//
// Parameters:
//   - name: lock scope suffix, e.g., "Server".
//   - ttl: lock TTL in seconds.
//   - renewal: true to renew existing lock, false to acquire.
//
// Returns:
//   - bool: true when lock operation succeeds.
func (j *Job) lock(name string, ttl int, renewal bool) bool {
	prefix := j.Redis.Prefix
	key := util.SpliceStr(prefix, "schedule:jobLock:", j.Name, ":", name)

	// Use EXPIRE for renewal and SET NX for acquisition.
	if renewal {
		_, err := j.Redis.Do("EXPIRE", key, ttl)
		if err == nil {
			return true
		}
	} else {
		ok, err := j.Redis.Do("SET", key, "locked", "EX", ttl, "NX")
		if ok != nil && err == nil {
			return true
		}
	}

	return false
}

// unLock releases a named lock for the current job.
//
// Parameters:
//   - ctx: trace-aware context for error logs.
//   - name: lock scope suffix.
//
// Returns:
//   - None.
func (j *Job) unLock(ctx context.Context, name string) {
	key := util.SpliceStr("schedule:jobLock:", j.Name, ":", name)

	ok, err := j.Redis.Del(key)
	if !ok && err != nil {
		j.Logger.Error(ctx, util.SpliceStr("unLock job:", name, "failed"), zap.Error(err))
	}
}

// renewalServerLock keeps server lock alive until current run finishes.
//
// Parameters:
//   - ctx: trace-aware context for lock release logs.
//
// Returns:
//   - None.
func (j *Job) renewalServerLock(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
Exit:
	for {
		select {
		case <-ticker.C:
			// Refresh lock TTL periodically while handler is running.
			j.lock("Server", defaultServerLockTTL, true)
		case <-j.RunTime.Done:
			// Release lock immediately when current execution finishes.
			j.unLock(ctx, "Server")
			break Exit
		}
	}
}

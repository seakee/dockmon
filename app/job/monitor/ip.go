// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package monitor implements scheduled job handlers under the job domain.
package monitor

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/seakee/dockmon/app/pkg/schedule"
	"github.com/sk-pkg/feishu"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/redis"
	"go.uber.org/zap"
)

const (
	CheckCNIpApi = "http://members.3322.org/dyndns/getip"
	CheckIpApi   = "http://whatismyip.akamai.com/"
	lastIpKey    = "monitor:ip:lastIp"
)

type ipHandler struct {
	done   chan struct{}
	error  chan error
	logger *logger.Manager
	redis  *redis.Manager
	lastIp string
	feishu *feishu.Manager
}

// setLastIp loads the last observed public IP from Redis.
//
// Returns:
//   - None.
//
// Behavior:
//   - Sends async error to error channel when Redis read fails.
func (ih *ipHandler) setLastIp() {
	lastIp, err := ih.redis.GetString(lastIpKey)
	if err != nil {
		ih.error <- fmt.Errorf("failed to get last IP from Redis: %w", err)
		return
	}

	ih.lastIp = lastIp
}

// Exec checks current public IP and stores updates when it changes.
//
// Parameters:
//   - ctx: trace-aware context used for structured logs.
//
// Returns:
//   - None.
//
// Behavior:
//   - Reads public IP from CheckCNIpApi.
//   - Compares it with cached value in Redis.
//   - Emits one done signal after execution.
func (ih *ipHandler) Exec(ctx context.Context) {
	ih.setLastIp()

	client := resty.New()
	res, err := client.R().Get(CheckCNIpApi)
	if err == nil && res != nil && res.StatusCode() == 200 {
		currentIp := strings.TrimRight(string(res.Body()), "\n")
		if ih.lastIp != currentIp && currentIp != "" {
			ih.logger.Info(ctx, "IP has changed", zap.String("last ip", ih.lastIp), zap.String("current ip", currentIp))
			ih.lastIp = currentIp

			if err = ih.redis.SetString(lastIpKey, currentIp, 0); err != nil {
				ih.error <- fmt.Errorf("failed to set last IP (%s) in Redis: %w", currentIp, err)
			}
		}
	} else if err != nil {
		ih.error <- fmt.Errorf("failed to check IP from %s: %w", CheckCNIpApi, err)
	}

	ih.done <- struct{}{}
}

// Error exposes the asynchronous error channel of the job handler.
//
// Returns:
//   - <-chan error: read-only channel carrying execution errors.
func (ih *ipHandler) Error() <-chan error {
	return ih.error
}

// Done exposes the completion channel of the job handler.
//
// Returns:
//   - <-chan struct{}: read-only channel signaling execution completion.
func (ih *ipHandler) Done() <-chan struct{} {
	return ih.done
}

// NewIpMonitor creates a schedule-compatible handler for public IP monitoring.
//
// Parameters:
//   - logger: logger manager for change notifications.
//   - redis: redis manager used to persist last observed IP.
//
// Returns:
//   - schedule.HandlerFunc: initialized IP monitor job handler.
//
// Example:
//
//	job := monitor.NewIpMonitor(logger, redis)
func NewIpMonitor(logger *logger.Manager, redis *redis.Manager) schedule.HandlerFunc {
	return &ipHandler{
		done:   make(chan struct{}),
		error:  make(chan error),
		logger: logger,
		lastIp: "",
		redis:  redis,
	}
}

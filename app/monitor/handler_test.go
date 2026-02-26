// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package monitor

import (
	"context"
	"reflect"
	"testing"

	collectorModel "github.com/seakee/dockmon/app/model/collector"
	"github.com/seakee/dockmon/app/pkg/trace"
	"github.com/sk-pkg/logger"
)

type mockLogService struct{}

func (mockLogService) Store(ctx context.Context, log *collectorModel.Log) (int, error) {
	return 1, nil
}

// newTestCollector creates a handler instance for unit tests.
//
// Returns:
//   - *handler: initialized monitor handler for tests.
//   - error: initialization error.
func newTestCollector() (*handler, error) {
	l, err := logger.New()
	if err != nil {
		return nil, err
	}

	containerNameList := []string{"go-api"}

	timeLayout := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.000",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05.000-0700",
		"2006/01/02",
		"2006/01/02 15:04:05",
		"2006/01/02 15:04:05.000",
		"2006/01/02T15:04:05Z07:00",
		"2006/01/02T15:04:05.000Z07:00",
		"Mon Jan 2 15:04:05 MST 2006",
		"02 Jan 06 15:04 MST",
		"02 Jan 2006 15:04:05",
	}

	UnstructuredLogLineFlags := []string{
		"fatal error:",
		"[GIN-debug]",
		"[GIN-warning]",
		"panic:",
	}

	traceID := trace.NewTraceID()

	return &handler{
		logger:           l,
		redis:            nil,
		dockerManager:    nil,
		service:          mockLogService{},
		activeContainers: &activeContainers{entries: make(map[string]bool)},
		unstructuredLogs: &unstructuredLogs{entries: make(map[string]*unstructuredLogBuffer)},
		traceID:          traceID,
		configs: &Config{
			MonitoredContainers:      &MonitoredContainers{Names: containerNameList},
			TimeLayout:               timeLayout,
			UnstructuredLogLineFlags: UnstructuredLogLineFlags,
		},
	}, nil
}

// TestRemoveFromSlice verifies that removeFromSlice removes all target values.
//
// Parameters:
//   - t: testing context.
//
// Returns:
//   - None.
func TestRemoveFromSlice(t *testing.T) {
	input := []string{"a", "b", "a", "c"}
	got := removeFromSlice(input, "a")
	want := []string{"b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("removeFromSlice() = %v, want %v", got, want)
	}
}

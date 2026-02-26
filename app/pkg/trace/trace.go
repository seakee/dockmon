// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package trace provides concurrent-safe trace ID generation utilities.
package trace

import (
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sk-pkg/util"
)

const (
	initIndex = 10000000 // Initial sequence value for each prefix epoch.
	indexBase = 36       // Base used to encode sequence and timestamp.
)

var (
	hostnameOnce sync.Once // Ensures hostname lookup is executed once.
	hostname     string    // Cached hostname reused by all trace IDs.
)

// ID generates unique trace IDs with a host+timestamp prefix.
type ID struct {
	index  uint64     // Sequence number, accessed atomically.
	prefix string     // Prefix containing hostname and timestamp.
	mu     sync.Mutex // Protects prefix refresh and reset operations.
}

// NewTraceID creates a trace ID generator initialized with host prefix data.
//
// Returns:
//   - *ID: initialized trace ID generator.
//
// Example:
//
//	tid := trace.NewTraceID()
//	id := tid.New()
func NewTraceID() *ID {
	t := &ID{
		index: initIndex,
	}
	t.updatePrefix()
	return t
}

// updatePrefix refreshes the prefix using current timestamp and cached hostname.
//
// Returns:
//   - None.
//
// Behavior:
//   - Fetches hostname once and falls back to "unknown" on failure.
//   - Resets sequence counter to initial value.
func (t *ID) updatePrefix() {
	var err error

	t.mu.Lock()
	defer t.mu.Unlock()

	hostnameOnce.Do(func() {
		hostname, err = os.Hostname()
		if err != nil {
			log.Printf("failed to get hostname: %v", err)
			// Use a stable fallback to keep ID generation available.
			hostname = "unknown"
		}
	})

	t.prefix = util.SpliceStr(hostname, "-", strconv.FormatInt(time.Now().UnixNano(), indexBase), "-")
	t.index = initIndex
}

// New returns a new unique trace ID string.
//
// Returns:
//   - string: unique trace ID composed of prefix and base36 sequence.
func (t *ID) New() string {
	// Atomically increment the sequence to avoid contention.
	newIndex := atomic.AddUint64(&t.index, 1)

	// On overflow, refresh prefix and reset sequence once.
	if newIndex == 0 {
		t.mu.Lock()
		defer t.mu.Unlock()
		if atomic.LoadUint64(&t.index) == 0 {
			t.updatePrefix()
		}
	}

	// Encode sequence with compact base36 representation.
	id := strconv.FormatUint(newIndex, indexBase)

	return util.SpliceStr(t.prefix, id)
}

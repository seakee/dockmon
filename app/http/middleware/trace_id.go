// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"github.com/gin-gonic/gin"
)

// SetTraceID returns middleware that binds a trace ID to every request.
//
// Returns:
//   - gin.HandlerFunc: middleware that reads or generates X-Trace-ID.
//
// Behavior:
//   - Reuses client-provided X-Trace-ID when present.
//   - Generates and echoes a new trace ID otherwise.
func (m middleware) SetTraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = m.traceID.New()
			c.Writer.Header().Set("X-Trace-ID", traceID)
		}

		c.Set("trace_id", traceID)

		c.Next()
	}
}

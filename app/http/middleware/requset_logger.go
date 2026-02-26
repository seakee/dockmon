// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/util"
	"go.uber.org/zap"
)

// RequestLogger returns middleware that records structured HTTP request logs.
//
// Returns:
//   - gin.HandlerFunc: middleware that logs latency, request metadata, and body.
//
// Behavior:
//   - Reads and restores request body so handlers can consume it later.
//   - Logs trace ID, status code, latency, method, URI, and source IP.
func (m middleware) RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Capture start time before forwarding to downstream handlers.
		startTime := time.Now()

		// Read and restore body so it remains available after logging.
		buf, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(buf))

		// Execute remaining middleware/handler chain.
		c.Next()

		// Capture end time after request processing.
		endTime := time.Now()

		// Calculate total request latency.
		latencyTime := endTime.Sub(startTime)

		// Collect HTTP method.
		reqMethod := c.Request.Method

		// Collect request URI.
		reqUri := c.Request.RequestURI

		// Collect response status code.
		statusCode := c.Writer.Status()

		// Resolve client IP from proxy headers.
		clientIP := util.GetRealIP(c)

		traceID, exists := c.Get("trace_id")
		if !exists {
			traceID = m.traceID.New()
		}

		ctx := context.WithValue(context.Background(), logger.TraceIDKey, traceID.(string))

		// Emit a structured request log entry.
		m.logger.Info(ctx,
			"Request Logs",
			zap.Int("StatusCode", statusCode),
			zap.Any("Latency", latencyTime),
			zap.String("IP", clientIP),
			zap.String("Method", reqMethod),
			zap.String("RequestPath", reqUri),
			zap.Any("body", string(buf)),
		)
	}
}

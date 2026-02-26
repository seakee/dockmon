// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package monitor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/seakee/dockmon/app/model/collector"
	"go.uber.org/zap"
)

// LogEntry represents one normalized log record before persistence.
type LogEntry struct {
	Level         string                 `json:"L"`
	Time          string                 `json:"T"`
	Caller        string                 `json:"C"`
	Message       string                 `json:"M"`
	TraceID       string                 `json:"TraceID"`
	ContainerID   string                 `json:"ContainerID"`
	ContainerName string                 `json:"ContainerName"`
	Extra         map[string]interface{} `json:"-"` // Additional structured fields.
}

var ansiEscapeCodeRegexp = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

const maxLogMessageBytes = 64000

// parseTimeString parses a time string using configured candidate layouts.
//
// Parameters:
//   - timeStr: raw time string extracted from log payload.
//
// Returns:
//   - time.Time: parsed local time.
//   - error: returned when no configured layout matches.
func (h *handler) parseTimeString(timeStr string) (time.Time, error) {
	for _, format := range h.configs.TimeLayout {
		if parsedTime, err := time.ParseInLocation(format, timeStr, time.Local); err == nil {
			return parsedTime, nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse time string: %s", timeStr)
}

// storeLog normalizes and persists one parsed log entry.
//
// Parameters:
//   - ctx: trace-aware context used for persistence logs.
//   - entry: parsed log entry to store.
//
// Returns:
//   - None.
//
// Behavior:
//   - Skips empty messages.
//   - Parses optional time field into sql.NullTime.
//   - Persists sanitized message and extra JSON fields.
func (h *handler) storeLog(ctx context.Context, entry *LogEntry) {
	// when message is empty, skip
	if entry.Message == "" {
		return
	}

	// Serialize additional JSON fields for storage.
	extraJSON, err := json.Marshal(entry.Extra)
	if err != nil {
		h.logger.Error(ctx, "marshal extra error", zap.Error(err))
		return
	}

	t := sql.NullTime{Valid: false}

	// Parse optional log timestamp.
	var dateTime time.Time
	if entry.Time != "" {
		dateTime, err = h.parseTimeString(entry.Time)
		if err != nil {
			h.logger.Error(ctx, "parse time error", zap.String("DateTimeString", entry.Time), zap.String("ContainerName", entry.ContainerName), zap.Error(err))
			return
		}
		t = sql.NullTime{Time: dateTime, Valid: true}
	}

	// Build persistence model and store it.
	log := &collector.Log{
		Level:         entry.Level,
		Time:          t,
		Caller:        entry.Caller,
		Message:       cleanString(entry.Message),
		TraceID:       entry.TraceID,
		ContainerID:   entry.ContainerID,
		ContainerName: entry.ContainerName,
		Extra:         extraJSON,
	}

	if _, err = h.service.Store(ctx, log); err != nil {
		h.logger.Error(ctx, "create log error", zap.Error(err))
	}
}

// cleanString sanitizes log messages before database persistence.
//
// Parameters:
//   - s: raw message string.
//
// Returns:
//   - string: cleaned UTF-8-safe message with control chars removed.
func cleanString(s string) string {
	if s == "" {
		return s
	}

	cleaned := strings.ToValidUTF8(s, "")
	cleaned = ansiEscapeCodeRegexp.ReplaceAllString(cleaned, "")

	v := make([]rune, 0, len(cleaned))
	for _, r := range cleaned {
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			v = append(v, r)
			continue
		}
		// Replace non-printable runes to avoid DB write failures.
		v = append(v, ' ')
	}

	cleaned = strings.TrimSpace(string(v))
	cleaned = truncateUTF8ByBytes(cleaned, maxLogMessageBytes)

	return cleaned
}

// truncateUTF8ByBytes truncates a string by byte size without breaking UTF-8.
//
// Parameters:
//   - s: source string.
//   - maxBytes: byte limit.
//
// Returns:
//   - string: truncated UTF-8-valid string.
func truncateUTF8ByBytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}

	truncated := s[:maxBytes]
	for !utf8.ValidString(truncated) && len(truncated) > 0 {
		truncated = truncated[:len(truncated)-1]
	}

	return truncated
}

// processUnstructuredLog flushes buffered unstructured lines for one container.
//
// Parameters:
//   - ctx: trace-aware context for persistence logs.
//   - containerID: container key of buffered lines.
//
// Returns:
//   - None.
func (h *handler) processUnstructuredLog(ctx context.Context, containerID string) {
	h.unstructuredLogs.mu.Lock()
	defer h.unstructuredLogs.mu.Unlock()

	uLog, exists := h.unstructuredLogs.entries[containerID]
	if !exists {
		return
	}

	defer func() {
		uLog.logs = uLog.logs[:0] // Reset buffered lines after flush.
	}()

	if len(uLog.logs) > 0 {
		// Use first line time prefix when available.
		logTime := h.extractTimeIfExists(uLog.logs[0])
		if logTime == "" {
			logTime = uLog.logTime
		}

		// Merge multiline logs into one structured entry.
		entry := h.processRemainingLogLines(ctx, uLog.logs, logTime, containerID, uLog.containerName)

		// Persist merged unstructured log.
		h.storeLog(ctx, &entry)
	}
}

// processLogLine processes one raw log line for a container.
//
// Parameters:
//   - ctx: trace-aware context for logs and persistence.
//   - logTime: extracted timestamp part.
//   - logLine: extracted message part.
//   - containerID: container ID.
//   - containerName: container name.
//
// Returns:
//   - None.
//
// Behavior:
//   - Tries JSON parsing first.
//   - Buffers unstructured multiline logs when needed.
func (h *handler) processLogLine(ctx context.Context, logTime, logLine, containerID, containerName string) {
	h.unstructuredLogs.mu.RLock()
	unstructuredLogCount := len(h.unstructuredLogs.entries[containerID].logs)

	// Try JSON format first.
	entry, err := h.tryParseLogToJson(ctx, logLine, containerID, containerName)
	if err == nil {
		if unstructuredLogCount > 0 {
			h.unstructuredLogs.mu.RUnlock()
			h.processUnstructuredLog(ctx, containerID)
		} else {
			h.unstructuredLogs.mu.RUnlock()
		}

		h.storeLog(ctx, &entry)
		return
	}

	// Handle unstructured logs and flush buffer when new message starts.
	if unstructuredLogCount > 0 && h.isUnstructuredLog(logLine) {
		h.unstructuredLogs.mu.RUnlock()
		h.processUnstructuredLog(ctx, containerID)
	} else {
		h.unstructuredLogs.mu.RUnlock()
	}

	h.unstructuredLogs.mu.Lock()
	// After previous buffer flush, set base timestamp for next unstructured block.
	if len(h.unstructuredLogs.entries[containerID].logs) == 0 {
		h.unstructuredLogs.entries[containerID].logTime = logTime
	}

	h.unstructuredLogs.entries[containerID].logs = append(h.unstructuredLogs.entries[containerID].logs, logLine)

	h.unstructuredLogs.mu.Unlock()
}

// isUnstructuredLog reports whether a log line starts a new unstructured block.
//
// Parameters:
//   - logLine: raw log message line.
//
// Returns:
//   - bool: true when line matches known unstructured prefixes.
func (h *handler) isUnstructuredLog(logLine string) bool {
	if h.isTimePrefix(logLine) {
		return true
	}

	for _, flag := range h.configs.UnstructuredLogLineFlags {
		if strings.HasPrefix(logLine, flag) {
			return true
		}
	}

	return false
}

// processRemainingLogLines merges buffered lines into one LogEntry.
//
// Parameters:
//   - ctx: trace-aware context reserved for future expansion.
//   - lines: buffered multiline log lines.
//   - logTime: timestamp associated with this block.
//   - containerID: source container ID.
//   - containerName: source container name.
//
// Returns:
//   - LogEntry: normalized unstructured log entry.
func (h *handler) processRemainingLogLines(ctx context.Context, lines []string, logTime, containerID, containerName string) LogEntry {
	// Merge all buffered lines to preserve stack traces and multiline output.
	message := strings.Join(lines, "\n")

	// Determine level heuristically from merged message content.
	level := h.determineLogLevel(ctx, message)

	return LogEntry{
		Level:         level,
		Time:          logTime,
		Caller:        "",
		Message:       message,
		TraceID:       "",
		ContainerID:   containerID,
		ContainerName: containerName,
		Extra:         make(map[string]interface{}),
	}
}

// tryParseLogToJson parses one JSON log line into LogEntry.
//
// Parameters:
//   - ctx: trace-aware context reserved for future expansion.
//   - line: raw log line.
//   - containerID: source container ID.
//   - containerName: source container name.
//
// Returns:
//   - LogEntry: parsed entry on success.
//   - error: JSON decode error.
func (h *handler) tryParseLogToJson(ctx context.Context, line string, containerID string, containerName string) (LogEntry, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return LogEntry{}, err
	}

	entry := LogEntry{
		Extra:         make(map[string]interface{}),
		ContainerID:   containerID,
		ContainerName: containerName,
	}

	// Map known fields and preserve unknown fields in Extra.
	for key, value := range raw {
		switch key {
		case "L":
			entry.Level = stringifyJSONValue(value)
		case "T":
			entry.Time = stringifyJSONValue(value)
		case "C":
			entry.Caller = stringifyJSONValue(value)
		case "M":
			entry.Message = stringifyJSONValue(value)
		case "TraceID":
			entry.TraceID = stringifyJSONValue(value)
		default:
			entry.Extra[key] = value
		}
	}

	return entry, nil
}

// determineLogLevel infers log level from message content.
//
// Parameters:
//   - ctx: trace-aware context reserved for future expansion.
//   - message: merged message content.
//
// Returns:
//   - string: normalized level string.
func (h *handler) determineLogLevel(ctx context.Context, message string) string {
	lowerMessage := strings.ToLower(message)

	// Priority-ordered level keywords.
	logLevels := []struct {
		key   string
		value string
	}{
		{"fatal", "FATAL"},
		{"panic", "PANIC"},
		{"error", "ERROR"},
		{"debug", "DEBUG"},
		{"warning", "WARN"},
		{"warn", "WARN"},
	}

	// Return first matched level keyword.
	for _, logLevel := range logLevels {
		if strings.Contains(lowerMessage, logLevel.key) {
			return logLevel.value
		}
	}

	return "INFO"
}

// stringifyJSONValue converts decoded JSON values to string form.
//
// Parameters:
//   - v: decoded JSON value.
//
// Returns:
//   - string: normalized textual representation.
func stringifyJSONValue(v interface{}) string {
	if v == nil {
		return ""
	}

	if s, ok := v.(string); ok {
		return s
	}

	return fmt.Sprint(v)
}

// matchTimePrefix matches supported date-time prefixes at line start.
//
// Parameters:
//   - line: raw log line.
//
// Returns:
//   - string: matched prefix or empty string.
//
// Supported formats:
//   - yyyy/MM/dd
//   - yyyy/MM/dd HH:mm:ss
//   - yyyy/MM/dd HH:mm:ss.SSSSSS
func (h *handler) matchTimePrefix(line string) string {
	re := regexp.MustCompile(`^\d{4}/\d{2}/\d{2}( \d{2}:\d{2}:\d{2}(\.\d{6})?)?`)
	return re.FindString(line)
}

// isTimePrefix reports whether line starts with a supported time prefix.
//
// Parameters:
//   - line: raw log line.
//
// Returns:
//   - bool: true when line starts with recognized time format.
func (h *handler) isTimePrefix(line string) bool {
	return h.matchTimePrefix(line) != ""
}

// extractTimeIfExists extracts line-leading time prefix when available.
//
// Parameters:
//   - line: raw log line.
//
// Returns:
//   - string: extracted time prefix or empty string.
func (h *handler) extractTimeIfExists(line string) string {
	return h.matchTimePrefix(line)
}

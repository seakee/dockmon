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

// LogEntry 日志条目结构体
type LogEntry struct {
	Level         string                 `json:"L"`
	Time          string                 `json:"T"`
	Caller        string                 `json:"C"`
	Message       string                 `json:"M"`
	TraceID       string                 `json:"TraceID"`
	ContainerID   string                 `json:"ContainerID"`
	ContainerName string                 `json:"ContainerName"`
	Extra         map[string]interface{} `json:"-"` // 额外信息
}

// parseTimeString 尝试解析多种时间格式的字符串并返回 time.Time 类型
func (h *handler) parseTimeString(timeStr string) (time.Time, error) {
	for _, format := range h.configs.TimeLayout {
		if parsedTime, err := time.ParseInLocation(format, timeStr, time.Local); err == nil {
			return parsedTime, nil
		}
	}

	return time.Time{}, fmt.Errorf("无法解析时间字符串: %s", timeStr)
}

// storeLog 存储日志
func (h *handler) storeLog(ctx context.Context, entry *LogEntry) {
	// when message is empty, skip
	if entry.Message == "" {
		return
	}

	// 序列化日志的额外信息
	extraJSON, err := json.Marshal(entry.Extra)
	if err != nil {
		h.logger.Error(ctx, "marshal extra error", zap.Error(err))
		return
	}

	t := sql.NullTime{Valid: false}

	// 解析日志时间
	var dateTime time.Time
	if entry.Time != "" {
		dateTime, err = h.parseTimeString(entry.Time)
		if err != nil {
			h.logger.Error(ctx, "parse time error", zap.String("DateTimeString", entry.Time), zap.String("ContainerName", entry.ContainerName), zap.Error(err))
			return
		}
		t = sql.NullTime{Time: dateTime, Valid: true}
	}

	// 创建日志对象并存储
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

// cleanString 清理字符串中的不可打印字符
func cleanString(s string) string {
	if !utf8.ValidString(s) {
		v := make([]rune, 0, len(s))
		for i, r := range s {
			if r == utf8.RuneError {
				_, size := utf8.DecodeRuneInString(s[i:])
				if size == 1 {
					continue
				}
			}
			if !unicode.IsPrint(r) {
				// 替换不可打印字符为空格或其他可打印字符
				r = ' '
			}
			v = append(v, r)
		}
		return string(v)
	}
	return s
}

// processUnstructuredLog 处理未结构化的日志
func (h *handler) processUnstructuredLog(ctx context.Context, containerID string) {
	h.unstructuredLogs.mu.Lock()
	defer h.unstructuredLogs.mu.Unlock()

	uLog, exists := h.unstructuredLogs.entries[containerID]
	if !exists {
		return
	}

	defer func() {
		uLog.logs = uLog.logs[:0] // 清空日志
	}()

	if len(uLog.logs) > 0 {
		// 提取日志中的时间
		logTime := h.extractTimeIfExists(uLog.logs[0])
		if logTime == "" {
			logTime = uLog.logTime
		}

		// 处理剩余的日志行
		entry := h.processRemainingLogLines(ctx, uLog.logs, logTime, containerID, uLog.containerName)

		// 存储日志
		h.storeLog(ctx, &entry)
	}
}

// processLogLine 处理一行日志
func (h *handler) processLogLine(ctx context.Context, logTime, logLine, containerID, containerName string) {
	h.unstructuredLogs.mu.RLock()
	unstructuredLogCount := len(h.unstructuredLogs.entries[containerID].logs)

	// 尝试解析 JSON 格式的日志
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

	// 处理未结构化的日志
	if unstructuredLogCount > 0 && h.isUnstructuredLog(logLine) {
		h.unstructuredLogs.mu.RUnlock()
		h.processUnstructuredLog(ctx, containerID)
	} else {
		h.unstructuredLogs.mu.RUnlock()
	}

	h.unstructuredLogs.mu.Lock()
	// 处理完上一个未结构化的日志后 logs 会被重置
	// 所以可以将当前日志行的时间赋值给 logTime，当做未结构化的日志的时间
	if len(h.unstructuredLogs.entries[containerID].logs) == 0 {
		h.unstructuredLogs.entries[containerID].logTime = logTime
	}

	h.unstructuredLogs.entries[containerID].logs = append(h.unstructuredLogs.entries[containerID].logs, logLine)

	h.unstructuredLogs.mu.Unlock()
}

// isUnstructuredLog 判断是否为未结构化的日志
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

// processRemainingLogLines 处理剩余的日志行并返回日志条目
func (h *handler) processRemainingLogLines(ctx context.Context, lines []string, logTime, containerID, containerName string) LogEntry {
	// 合并所有日志行
	message := strings.Join(lines, "\n")

	// 确定日志级别
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

// tryParseLogToJson 尝试将日志解析 JSON 格式
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

	// 解析 JSON 字段
	for key, value := range raw {
		switch key {
		case "L":
			entry.Level = value.(string)
		case "T":
			entry.Time = value.(string)
		case "C":
			entry.Caller = value.(string)
		case "M":
			entry.Message = value.(string)
		case "TraceID":
			entry.TraceID = value.(string)
		default:
			entry.Extra[key] = value
		}
	}

	return entry, nil
}

// determineLogLevel 确定日志级别
func (h *handler) determineLogLevel(ctx context.Context, message string) string {
	lowerMessage := strings.ToLower(message)

	// 日志级别列表
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

	// 匹配日志级别
	for _, logLevel := range logLevels {
		if strings.Contains(lowerMessage, logLevel.key) {
			return logLevel.value
		}
	}

	return "INFO"
}

// matchTimePrefix 使用正则表达式匹配时间前缀
// 支持格式: yyyy/MM/dd, yyyy/MM/dd HH:mm:ss, yyyy/MM/dd HH:mm:ss.SSSSSS
func (h *handler) matchTimePrefix(line string) string {
	re := regexp.MustCompile(`^\d{4}/\d{2}/\d{2}( \d{2}:\d{2}:\d{2}(\.\d{6})?)?`)
	return re.FindString(line)
}

// isTimePrefix 检查行是否以日期时间前缀开头
func (h *handler) isTimePrefix(line string) bool {
	return h.matchTimePrefix(line) != ""
}

// extractTimeIfExists 提取日志中的时间前缀（如果存在）
func (h *handler) extractTimeIfExists(line string) string {
	return h.matchTimePrefix(line)
}

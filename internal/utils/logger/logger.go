// Package logger 提供全局结构化日志能力。
//
// 使用 Go 1.21+ 标准库 log/slog 实现，支持：
// - JSON 和 Text 两种输出格式
// - DEBUG/INFO/WARN/ERROR 四个级别
// - 输出到 stdout/stderr/文件
//
// 使用方式：
//
//	logger.Init(logger.Config{Level: "debug", Format: "json"})
//	logger.Info("message processed", "session", sessionKey, "tokens", 150)

package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Config 控制日志行为
type Config struct {
	// Level 日志级别: debug, info, warn, error
	Level string `json:"level"`
	// Format 输出格式: json, text
	Format string `json:"format"`
	// Output 输出目标: stdout, stderr, 或文件路径
	Output string `json:"output"`
}

var (
	// defaultLogger 是一个全局可用的 Logger 实例
	defaultLogger *slog.Logger
	// once 确保 lazy init 只执行一次
	once sync.Once
)

// ensureInit 确保默认 logger 已初始化（懒加载）
func ensureInit() {
	once.Do(func() {
		if defaultLogger == nil {
			// 使用默认配置初始化
			handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
			defaultLogger = slog.New(handler)
		}
	})
}

func Init(cfg Config) error {
	// 解析日志级别
	level := parseLevel(cfg.Level)

	// 选择输出
	output, err := getOutput(cfg.Output)
	if err != nil {
		return err
	}

	// 选择格式
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if strings.EqualFold(cfg.Format, "json") {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
	return nil
}

// parseLevel 将字符串转换为 slog.Level
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// getOutput 根据配置返回对应的 io.Writer
func getOutput(output string) (io.Writer, error) {
	switch strings.ToLower(strings.TrimSpace(output)) {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		// 尝试打开文件进行写入
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
}

func Debug(msg string, args ...any) {
	ensureInit()
	defaultLogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	ensureInit()
	defaultLogger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	ensureInit()
	defaultLogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	ensureInit()
	defaultLogger.Error(msg, args...)
}

func With(args ...any) *slog.Logger {
	ensureInit()
	return defaultLogger.With(args...)
}

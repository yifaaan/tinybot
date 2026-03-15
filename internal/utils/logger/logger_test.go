package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// resetLoggerStateForTest 把 logger 包里的全局状态隔离开，
// 避免一个测试对 defaultLogger / slog.Default() 的修改污染下一个测试。
func resetLoggerStateForTest(t *testing.T) {
	t.Helper()

	oldDefaultLogger := defaultLogger
	oldSlogDefault := slog.Default()

	defaultLogger = nil
	once = sync.Once{}

	t.Cleanup(func() {
		defaultLogger = oldDefaultLogger
		once = sync.Once{}
		if oldSlogDefault != nil {
			slog.SetDefault(oldSlogDefault)
		}
	})
}

// cleanupLogDir 尝试释放并删除测试产生的日志目录。
//
// Windows 下文件句柄释放有时会比测试断言稍晚一点，
// 所以这里先把 logger 重新指回 stderr，再多做几轮 GC + 删除重试。
func cleanupLogDir(t *testing.T, dir string) {
	t.Helper()

	_ = Init(Config{
		Level:  "info",
		Format: "text",
		Output: "stderr",
	})

	var lastErr error
	for i := 0; i < 10; i++ {
		runtime.GC()
		lastErr = os.RemoveAll(dir)
		if lastErr == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	if lastErr != nil {
		t.Fatalf("RemoveAll(%q) failed: %v", dir, lastErr)
	}
}

func TestParseLevel_UnknownDefaultsToInfo(t *testing.T) {
	if got := parseLevel("mystery"); got != slog.LevelInfo {
		t.Fatalf("parseLevel(%q) = %v, want %v", "mystery", got, slog.LevelInfo)
	}
	if got := parseLevel(""); got != slog.LevelInfo {
		t.Fatalf("parseLevel(%q) = %v, want %v", "", got, slog.LevelInfo)
	}
}

func TestInit_InvalidOutputPathReturnsError(t *testing.T) {
	resetLoggerStateForTest(t)

	invalidPath := filepath.Join(t.TempDir(), "missing", "tinybot.log")
	err := Init(Config{
		Level:  "info",
		Format: "text",
		Output: invalidPath,
	})
	if err == nil {
		t.Fatal("expected Init() error, got nil")
	}
}

func TestInit_FileOutputCreatesLogFile(t *testing.T) {
	resetLoggerStateForTest(t)

	logDir, err := os.MkdirTemp("", "tinybot-logger-*")
	if err != nil {
		t.Fatalf("MkdirTemp() error: %v", err)
	}
	t.Cleanup(func() {
		cleanupLogDir(t, logDir)
	})

	logPath := filepath.Join(logDir, "tinybot.log")
	if err := Init(Config{
		Level:  "info",
		Format: "text",
		Output: logPath,
	}); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Init 完成后写一条日志，验证文件输出不仅能创建文件，
	// 还真的能把内容写进去。
	Info("logger file output test", "component", "logger")

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Stat(%q) error: %v", logPath, err)
	}
	if info.IsDir() {
		t.Fatalf("log path %q is a directory, want file", logPath)
	}
	if info.Size() == 0 {
		t.Fatalf("log file %q is empty, want log content", logPath)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error: %v", logPath, err)
	}
	text := string(data)
	if !strings.Contains(text, "logger file output test") {
		t.Fatalf("log file missing message, got %q", text)
	}
}

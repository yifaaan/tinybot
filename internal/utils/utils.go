package utils

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExpandUser expands the user path to the absolute path.
func ExpandUser(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}

	if path == "~" {
		return homeDir, nil
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:]), nil
	}

	return path, nil
}

// Ensure a directory exists, creating it if necessary.
func EnsureDir(path string) (string, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}
	return path, nil
}

func TodayDate() string {
	return time.Now().In(time.Local).Format("2006-01-02")
}

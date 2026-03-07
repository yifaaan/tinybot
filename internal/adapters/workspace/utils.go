package workspace

import (
	"os"
	"time"
)

// Ensure a directory exists, creating it if necessary.
func ensureDir(path string) (string, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}
	return path, nil
}

func todayDate() string {
	return time.Now().In(time.Local).Format("2026-03-07")
}

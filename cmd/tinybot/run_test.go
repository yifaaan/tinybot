package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_NoArgs_PrintsHelp(t *testing.T) {
	var out bytes.Buffer

	if err := run(nil, &out, filepath.Join(t.TempDir(), "workspace")); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "Usage: tinybot") {
		t.Fatalf("help output missing usage: %s", text)
	}
	if !strings.Contains(text, "onboard") {
		t.Fatalf("help output missing onboard command: %s", text)
	}
	if !strings.Contains(text, "status") {
		t.Fatalf("help output missing status command: %s", text)
	}
}

func TestRun_Onboard_CreatesWorkspace(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer

	if err := run([]string{"onboard"}, &out, workspace); err != nil {
		t.Fatalf("run(onboard) error: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "Workspace ready at") {
		t.Fatalf("onboard output missing workspace message: %s", text)
	}
	if !strings.Contains(text, "Created files:") {
		t.Fatalf("onboard output missing created files section: %s", text)
	}
}

func TestRun_Status_ShowsMissingFilesBeforeOnboard(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer

	if err := run([]string{"status"}, &out, workspace); err != nil {
		t.Fatalf("run(status) error: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "Workspace exists: false") {
		t.Fatalf("status output missing workspace flag: %s", text)
	}
	if !strings.Contains(text, "Missing files:") {
		t.Fatalf("status output missing missing-files section: %s", text)
	}
	if !strings.Contains(text, "config.json") {
		t.Fatalf("status output missing config.json: %s", text)
	}
	if !strings.Contains(text, filepath.Join("memory", "MEMORY.md")) {
		t.Fatalf("status output missing memory file: %s", text)
	}
}

func TestRun_CronList_Empty(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer
	if err := run([]string{"cron", "list"}, &out, workspace); err != nil {
		t.Fatalf("run(cron list) error: %v", err)
	}
	if !strings.Contains(out.String(), "No cron jobs found.") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}
func TestRun_CronAdd_ThenList(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer
	if err := run([]string{"cron", "add", "daily", "60", "check status"}, &out, workspace); err != nil {
		t.Fatalf("run(cron add) error: %v", err)
	}
	out.Reset()
	if err := run([]string{"cron", "list"}, &out, workspace); err != nil {
		t.Fatalf("run(cron list) error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "daily") {
		t.Fatalf("cron list output missing job name: %s", text)
	}
}
func TestRun_CronAdd_InvalidArgs(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer
	err := run([]string{"cron", "add", "daily"}, &out, workspace)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

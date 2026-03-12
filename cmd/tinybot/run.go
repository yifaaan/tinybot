package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
	"tinybot/internal/adapters/repository"
	"tinybot/internal/app"
	"tinybot/internal/domain/model"
	"tinybot/internal/ports"
	"tinybot/internal/usecase/cron"
)

// run is the testable entry point for the CLI.
//
// We keep main() tiny on purpose:
// - main() should only deal with process-level concerns such as os.Args and os.Stdout
// - run() contains command dispatch logic that we can unit test without spawning a real process
//
// workspace is injected as an argument so tests can use a temporary directory instead of touching
// the user's real .tinybot workspace.
func run(args []string, out io.Writer, workspace string) error {
	if out == nil {
		out = io.Discard
	}

	if len(args) == 0 {
		printHelp(out)
		return nil
	}

	cmd := strings.TrimSpace(args[0])
	switch cmd {
	case "help", "-h", "--help":
		printHelp(out)
		return nil
	case "onboard":
		return runOnboard(out, workspace)
	case "status":
		return runStatus(out, workspace)
	case "gateway":
		return runGateway(out, workspace)
	case "cron":
		return runCron(args[1:], out, workspace)
	default:
		// Any non-command input is treated as a direct chat message.
		// This keeps the CLI ergonomic for the MVP: `tinybot 你好` still works.
		return runDirectChat(args, out, workspace)
	}
}

func runOnboard(out io.Writer, workspace string) error {
	result, err := app.OnBoard(context.Background(), workspace)
	if err != nil {
		return fmt.Errorf("onboard failed: %w", err)
	}

	_, _ = fmt.Fprintf(out, "Workspace ready at %s\n", result.Workspace)
	if len(result.CreatedFiles) == 0 {
		_, _ = fmt.Fprintln(out, "No new files created.")
		return nil
	}

	_, _ = fmt.Fprintln(out, "Created files:")
	for _, name := range result.CreatedFiles {
		_, _ = fmt.Fprintf(out, "  - %s\n", name)
	}

	return nil
}

func runStatus(out io.Writer, workspace string) error {
	status := app.CheckStatus(workspace)

	_, _ = fmt.Fprintf(out, "Workspace: %s\n", status.Workspace)
	_, _ = fmt.Fprintf(out, "Workspace exists: %t\n", status.WorkspaceExists)
	_, _ = fmt.Fprintf(out, "Config exists: %t\n", status.ConfigExists)
	_, _ = fmt.Fprintf(out, "Memory exists: %t\n", status.MemoryFileExists)
	_, _ = fmt.Fprintf(out, "Skills dir exists: %t\n", status.SkillsDirExists)
	_, _ = fmt.Fprintf(out, "Heartbeat file exists: %t\n", status.HeartbeatFileExists)

	if len(status.MissingFiles) > 0 {
		_, _ = fmt.Fprintln(out, "Missing files:")
		for _, name := range status.MissingFiles {
			_, _ = fmt.Fprintf(out, "  - %s\n", name)
		}
	}

	return nil
}

func runDirectChat(args []string, out io.Writer, workspace string) error {
	appInstance, err := app.NewApp(workspace)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	input := strings.TrimSpace(strings.Join(args, " "))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	reply, err := appInstance.ChatUseCase.ProcessDirect(ctx, "cli:direct", input)
	if err != nil {
		return fmt.Errorf("failed to process direct message: %w", err)
	}

	_, _ = fmt.Fprintln(out, reply)
	return nil
}

func runGateway(out io.Writer, workspace string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	gw, err := app.NewGatewayApp(workspace, os.Stdin, out)
	if err != nil {
		return fmt.Errorf("run gateway: %w", err)
	}
	defer gw.Close()

	_, _ = fmt.Fprintln(out, "tinybot gateway started (console channel mode)")
	_, _ = fmt.Fprintln(out, "Type a message and press Enter. Press Ctrl+C to stop.")
	return gw.Run(ctx)
}

func runCron(args []string, out io.Writer, workspace string) error {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(out, "Usage: tinybot cron <list|add|run-once>")
		return nil
	}

	repo := repository.NewFileCronRepository(workspace)

	switch strings.TrimSpace(args[0]) {
	case "list":
		return runCronList(out, repo)
	case "add":
		return runCronAdd(args[1:], out, repo)
	case "remove":
		return runCronRemove(args[1:], out, repo)
	case "run-once":
		return runCronRunOnce(out, workspace, repo)
	default:
		return fmt.Errorf("unknown cron command: %s", args[0])
	}
}

func runCronList(out io.Writer, repo ports.CronRepository) error {
	jobs, err := repo.ListJobs(context.Background())
	if err != nil {
		return fmt.Errorf("cron list: %w", err)
	}

	if len(jobs) == 0 {
		_, _ = fmt.Fprintln(out, "No cron jobs found.")
		return nil
	}

	for _, job := range jobs {
		_, _ = fmt.Fprintf(out, "- %s (%s) enabled=%t every=%ds\n",
			job.Name,
			job.ID,
			job.Enabled,
			job.Schedule.EverySeconds,
		)
	}
	return nil
}

// tinybot cron add <name> <every_seconds> <prompt>
func runCronAdd(args []string, out io.Writer, repo ports.CronRepository) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: tinybot cron add <name> <every_seconds> <prompt>")
	}

	name := strings.TrimSpace(args[0])
	everySecons, err := strconv.Atoi(strings.TrimSpace(args[1]))
	if err != nil {
		return fmt.Errorf("cron add parse every_second: %w", err)
	}

	prompt := strings.TrimSpace(strings.Join(args[2:], " "))

	if name == "" {
		return fmt.Errorf("cron add: name is requried")
	}
	if prompt == "" {
		return fmt.Errorf("cron add: prompt is required")
	}

	jobs, err := repo.ListJobs(context.Background())
	if err != nil {
		return fmt.Errorf("cron add list jobs: %w", err)
	}

	now := time.Now()
	jobID := fmt.Sprintf("job-%d", now.UnixNano())

	job := model.CronJob{
		ID:         jobID,
		Name:       name,
		Enabled:    true,
		Prompt:     prompt,
		SessionKey: "cron:" + jobID,
		Schedule: model.CronSchedule{
			Kind:         model.CronScheduleEvery,
			EverySeconds: everySecons,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	job.NextRunAt = job.ComputeNextRun(now)

	if err := job.Validate(); err != nil {
		return fmt.Errorf("cron and validate job: %w", err)
	}

	jobs = append(jobs, job)
	if err := repo.SaveJobs(jobs); err != nil {
		return fmt.Errorf("cron add save jobs: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Added cron job %s (%s)\n", job.Name, job.ID)
	return nil
}

func runCronRunOnce(out io.Writer, workspace string, repo ports.CronRepository) error {
	appInstance, err := app.NewApp(workspace)
	if err != nil {
		return fmt.Errorf("cron run-once new app: %w", err)
	}

	svc, err := cron.NewService(repo, appInstance.ChatUseCase, 30)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	msg, err := svc.TriggerOnce(ctx)
	if err != nil {
		return fmt.Errorf("cron run-once: %w", err)
	}

	if strings.TrimSpace(msg) == "" {
		msg = "cron scan completed"
	}

	_, _ = fmt.Fprintln(out, msg)
	return nil
}

// tinybot cron remove <job_id>
func runCronRemove(args []string, out io.Writer, repo ports.CronRepository) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tinybot cron remove <job_id>")
	}

	jobs, err := repo.ListJobs(context.Background())
	if err != nil {
		return fmt.Errorf("cron add list jobs: %w", err)
	}

	found := false
	jobID := strings.TrimSpace(args[0])
	for i, job := range jobs {
		if job.ID == jobID {
			jobs = append(jobs[:i], jobs[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("cron remove jobs: not found job, id=%s", jobID)
	}
	if err := repo.SaveJobs(jobs); err != nil {
		return err
	}
	return nil
}

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
	"tinybot/internal/app"
	"tinybot/internal/domain/model"
	"tinybot/internal/repository/cronrepo"
	cronservice "tinybot/internal/service/cron"
)

type directChatProcessor interface {
	ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error)
}

type directStreamProcessor interface {
	ProcessMessageStream(ctx context.Context, msg model.InboundMessage, onDelta func(string)) (model.OutboundMessage, error)
}

var newDirectChatProcessor = func(workspace string) (directChatProcessor, error) {
	appInstance, err := app.NewApp(workspace)
	if err != nil {
		return nil, err
	}
	return appInstance.ChatService, nil
}

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
	processor, err := newDirectChatProcessor(workspace)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	input := strings.TrimSpace(strings.Join(args, " "))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 类型断言：检查 processor 是否支持流式
	if sp, ok := processor.(directStreamProcessor); ok {
		msg := model.InboundMessage{
			ID:       fmt.Sprintf("direct-%d", time.Now().UnixNano()),
			Channel:  model.ChannelCLI,
			SenderID: "user",
			ChatID:   "direct",
			Content:  input,
			SessionKeyOverride: func() *string {
				s := "cli:direct"
				return &s
			}(),
		}
		_, err := sp.ProcessMessageStream(ctx, msg, func(delta string) {
			fmt.Fprint(out, delta) // 每收到一个增量就立即输出，保持流式体验
		})
		if err != nil {
			return err
		}
		fmt.Fprintln(out) // 流结束后补一个换行
		return nil
	}

	// 回退到非流式

	reply, err := processor.ProcessDirect(ctx, "cli:direct", input)
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
		_, _ = fmt.Fprintln(out, "Usage: tinybot cron <list|add|add-at|add-cron|remove|run-once>")
		return nil
	}

	repo := cronrepo.NewFileCronRepository(workspace)

	switch strings.TrimSpace(args[0]) {
	case "list":
		return runCronList(out, repo)
	case "add":
		return runCronAdd(args[1:], out, repo)
	case "add-at":
		return runCronAddAt(args[1:], out, repo)
	case "add-cron":
		return runCronAddCron(args[1:], out, repo)
	case "remove":
		return runCronRemove(args[1:], out, repo)
	case "run-once":
		return runCronRunOnce(out, workspace, repo)
	default:
		return fmt.Errorf("unknown cron command: %s", args[0])
	}
}

func runCronList(out io.Writer, repo cronservice.Repository) error {
	jobs, err := repo.ListJobs(context.Background())
	if err != nil {
		return fmt.Errorf("cron list: %w", err)
	}

	if len(jobs) == 0 {
		_, _ = fmt.Fprintln(out, "No cron jobs found.")
		return nil
	}

	for _, job := range jobs {
		_, _ = fmt.Fprintf(out, "- %s (%s) enabled=%t %s\n",
			job.Name,
			job.ID,
			job.Enabled,
			formatCronSchedule(job),
		)
	}
	return nil
}

func formatCronSchedule(job model.CronJob) string {
	switch job.Schedule.Kind {
	case model.CronScheduleEvery:
		return fmt.Sprintf("every=%ds", job.Schedule.EverySeconds)

	case model.CronScheduleAt:
		if job.Schedule.At == nil {
			return "at=<nil>"
		}
		return fmt.Sprintf("at=%s", job.Schedule.At.Format(time.RFC3339))
	case model.CronScheduleCron:
		return fmt.Sprintf("cron=%s", job.Schedule.CronExpr)

	default:
		return fmt.Sprintf("schedule=%s", job.Schedule.Kind)
	}
}

// tinybot cron add <name> <every_seconds> <prompt>
func runCronAdd(args []string, out io.Writer, repo cronservice.Repository) error {
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
	ctx := context.Background()
	jobs, err := repo.ListJobs(ctx)
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
	if err := repo.SaveJobs(ctx, jobs); err != nil {
		return fmt.Errorf("cron add save jobs: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Added cron job %s (%s)\n", job.Name, job.ID)
	return nil
}

// tinybot cron add-cron <name> <cron_expr> <prompt>
// 示例: tinybot cron add-cron morning "0 9 * * *" "早上好，今天有什么安排？"
func runCronAddCron(args []string, out io.Writer, repo cronservice.Repository) error {
	if len(args) < 3 {
		return fmt.Errorf("usage tinybot cron add-cron <name> <cron_expr> <prompt>")
	}

	name := strings.TrimSpace(args[0])
	cronExpr := strings.TrimSpace(args[1])
	prompt := strings.TrimSpace(strings.Join(args[2:], " "))

	if name == "" {
		return fmt.Errorf("cron add cron: name is requried")
	}
	if prompt == "" {
		return fmt.Errorf("cron add cron: prompt is required")
	}
	ctx := context.Background()
	jobs, err := repo.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("cron add cron list jobs: %w", err)
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
			Kind:     model.CronScheduleCron,
			CronExpr: cronExpr,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	job.NextRunAt = job.ComputeNextRun(now)

	if err := job.Validate(); err != nil {
		return fmt.Errorf("cron and validate job: %w", err)
	}

	jobs = append(jobs, job)
	if err := repo.SaveJobs(ctx, jobs); err != nil {
		return fmt.Errorf("cron add save jobs: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Added cron job %s (%s) schedule=%s\n", job.Name, job.ID, cronExpr)
	return nil
}

func runCronRunOnce(out io.Writer, workspace string, repo cronservice.Repository) error {
	appInstance, err := app.NewApp(workspace)
	if err != nil {
		return fmt.Errorf("cron run-once new app: %w", err)
	}

	svc, err := cronservice.NewService(repo, appInstance.ChatService, nil)
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
func runCronRemove(args []string, out io.Writer, repo cronservice.Repository) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tinybot cron remove <job_id>")
	}

	ctx := context.Background()
	jobs, err := repo.ListJobs(ctx)
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
	if err := repo.SaveJobs(ctx, jobs); err != nil {
		return err
	}
	return nil
}

func runCronAddAt(args []string, out io.Writer, repo cronservice.Repository) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: tinybot cron add-at <name> <rfc3339_time> <prompt>")
	}

	name := strings.TrimSpace(args[0])
	atRaw := strings.TrimSpace(args[1])
	prompt := strings.TrimSpace(strings.Join(args[2:], " "))

	if name == "" {
		return fmt.Errorf("cron add-at: name is requried")
	}
	if prompt == "" {
		return fmt.Errorf("cron add-at: prompt is required")
	}

	atTime, err := time.Parse(time.RFC3339, atRaw)
	if err != nil {
		return fmt.Errorf("cron add-at: invalid time format: %w", err)
	}

	now := time.Now()
	if atTime.Before(now) {
		return fmt.Errorf("cron add-at: time must be in the future")
	}

	ctx := context.Background()
	jobs, err := repo.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("cron add-at list jobs: %w", err)
	}

	jobID := fmt.Sprintf("job-%d", now.UnixNano())

	job := model.CronJob{
		ID:         jobID,
		Name:       name,
		Enabled:    true,
		Prompt:     prompt,
		SessionKey: "cron:" + jobID,
		Schedule: model.CronSchedule{
			Kind: model.CronScheduleAt,
			At:   &atTime,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	job.NextRunAt = job.ComputeNextRun(now)

	if err := job.Validate(); err != nil {
		return fmt.Errorf("cron add-at validate job: %w", err)
	}
	jobs = append(jobs, job)
	if err := repo.SaveJobs(ctx, jobs); err != nil {
		return fmt.Errorf("cron add-at save jobs: %w", err)
	}

	_, _ = fmt.Fprintf(out, "Added cron job %s (%s) at %s\n", job.Name, job.ID, atTime.Format(time.RFC3339))
	return nil
}

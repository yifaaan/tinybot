# Next Milestone: Cron Expression Support

这份说明的目标不是“把功能一次性写完”，而是给你一个可以自己落地的最小实现切片。

当前仓库里，聊天主链路已经比较完整，下一步更值得补的是 `cron` 的兼容性缺口：

- Go 端已经支持 `every` 和 `at`
- 原始 `nanobot` 还支持 `cron expression`
- 这个缺口主要落在 `domain + service + repository + CLI`，改动边界清晰

相比之下，`deliver/channel/to` 也很重要，但它会把 `cron service` 和 `transport` 发送边界重新拉通，当前作为“下一小步”会更重。

## 1. 为什么这一步最合适

先看当前 Go 端已经稳定下来的部分：

- `internal/service/chat/service.go`
  - 已经有完整的单轮聊天编排、tool loop、session 持久化
- `internal/service/chat/prompt_builder_test.go`
  - prompt 组装顺序、memory、skills、selected skills 都已有覆盖
- `internal/repository/sessionrepo/file_session_repo.go`
  - session repository 已经收口到 chat service 的真实需要

再看当前 `cron` 的缺口：

- `internal/domain/model/cron.go`
  - 目前只有 `every` 和 `at`
  - 文件里还明确保留了 `// TODO: cron`
- `cmd/tinybot/run.go`
  - CLI 目前支持 `cron add` 和 `cron add-at`
  - 还没有 `cron add-cron`
- `internal/service/cron/service.go`
  - 执行后状态推进只处理了 `every` 和 `at`

再对照原始 Python：

- `nanobot/nanobot/cron/types.py`
  - `CronSchedule` 有 `kind="cron"` 和 `expr`
- `nanobot/nanobot/cron/service.py`
  - `_compute_next_run(...)` 已经处理 `cron`
- `nanobot/nanobot/cli/commands.py`
  - `nanobot cron add --cron "0 9 * * *"` 是一等入口

所以，从“最小闭环”角度，下一步最合适实现的是：

1. 在 Go 端补齐 `cron expression` 的领域模型
2. 让 CLI 能创建这种 job
3. 让 repository 能存取这种 job
4. 让 cron service 能按同样的状态迁移规则推进它

## 2. 这一步的边界

这一小步只做下面这些：

- 新增 `CronScheduleCron`
- 在 `CronSchedule` 中新增 `Expr string`
- `Validate()` 接受 `kind=cron`
- `ComputeNextRun()` 支持 cron expression
- CLI 增加 `tinybot cron add-cron <name> <expr> <prompt>`
- `cron list` 能把这种 schedule 正确打印出来
- 为 domain、repository、service、CLI 增加测试

这一小步先不做：

- `deliver/channel/to`
- timezone 支持
- 禁用/启用命令
- 手动运行指定 job
- 更复杂的 cron payload 抽象

默认建议：

- 先把 cron expression 当作“本地时区下的 5 段表达式”
- 不要在这一小步同时引入 timezone 配置
- 不要先为“未来可能支持更多调度器”提前抽一层接口

原因很简单：你现在最需要的是把兼容行为补齐，而不是提前抽象。

## 3. 推荐设计

### 3.1 先维持当前模型中心不变

当前 `CronJob.ComputeNextRun(from time.Time)` 已经是调度逻辑的稳定入口。

这一步不建议一上来把它拆成额外的 `Planner` 接口，原因是：

- 当前只有三种 schedule：`every`、`at`、`cron`
- 过早抽接口会让你为了“抽象”而抽象
- 你现在更应该先把可观察行为补齐，并让测试固定下来

所以这一步推荐的默认方案是：

- 仍然保留 `CronJob.ComputeNextRun(...)`
- 只是让它多处理一种 `cron` schedule

后面如果你准备支持：

- timezone
- 更复杂的 schedule 校验
- 更强的可测试性

再考虑把“表达式求下一次运行时间”提取成一个独立 planner。

### 3.2 依赖处理建议

Go 标准库没有 cron expression 解析器。

这里有两个选择：

1. 自己实现一个简化 parser
2. 引入成熟库

默认推荐第 2 种，因为这个逻辑属于“规则解析”，自己手写更容易埋 bug。

但这一步文档先只给骨架，不强行指定具体库实现。你真正落地时再决定：

- 如果你想优先正确性，推荐成熟库
- 如果你想练习 parser，也可以先只支持一个非常小的子集

## 4. 实现骨架

下面的骨架是“可编译方向”的结构说明，不是最终代码。
你应该按步骤逐个填充，并在每一步补测试。

### 4.1 `internal/domain/model/cron.go`

```go
package model

import (
    "fmt"
    "strings"
    "time"
)

type CronScheduleKind string

const (
    CronScheduleEvery CronScheduleKind = "every"
    CronScheduleAt    CronScheduleKind = "at"
    CronScheduleCron  CronScheduleKind = "cron"
)

// CronSchedule 描述“任务应该在什么时间点触发”。
//
// 这里先保持最小模型：
// - every: 相对间隔
// - at:    单次绝对时间
// - cron:  cron 表达式
//
// 这一小步先不把 timezone 也放进来，避免一次改太多维度。
type CronSchedule struct {
    Kind         CronScheduleKind `json:"kind"`
    EverySeconds int              `json:"every_seconds,omitempty"`
    At           *time.Time       `json:"at,omitempty"`

    // Expr 仅在 kind=cron 时生效。
    // 例子: "0 9 * * *"
    Expr string `json:"expr,omitempty"`
}

func (j *CronJob) Validate() error {
    if j.Name == "" {
        return fmt.Errorf("cron job name is required")
    }
    if j.ID == "" {
        return fmt.Errorf("cron job ID is required")
    }
    if j.Prompt == "" {
        return fmt.Errorf("cron job prompt is required")
    }

    switch j.Schedule.Kind {
    case CronScheduleEvery:
        if j.Schedule.EverySeconds <= 0 {
            return fmt.Errorf("cron job schedule every_seconds must be positive")
        }

    case CronScheduleAt:
        if j.Schedule.At == nil {
            return fmt.Errorf("cron job schedule at is required")
        }
        if j.Schedule.At.IsZero() {
            return fmt.Errorf("cron job schedule at must be a valid time")
        }

    case CronScheduleCron:
        expr := strings.TrimSpace(j.Schedule.Expr)
        if expr == "" {
            return fmt.Errorf("cron job schedule expr is required")
        }

        // TODO:
        // 在这里调用一个小的校验 helper。
        // 建议先做“语法有效”校验，不要在 Validate 里混入“下一次运行时间计算”。
        //
        // 例如:
        // if err := validateCronExpr(expr); err != nil { ... }

    default:
        return fmt.Errorf("unsupported cron schedule kind: %s", j.Schedule.Kind)
    }

    return nil
}

// ComputeNextRun 根据调度类型返回下一次触发时间。
//
// 这里继续把“调度规则”收在领域模型里，
// 是因为当前项目还只有少量 schedule 变体，保持集中更容易理解。
func (j *CronJob) ComputeNextRun(from time.Time) *time.Time {
    switch j.Schedule.Kind {
    case CronScheduleEvery:
        nextRun := from.Add(time.Duration(j.Schedule.EverySeconds) * time.Second)
        return &nextRun

    case CronScheduleAt:
        if j.Schedule.At == nil || j.Schedule.At.Before(from) {
            return nil
        }
        nextRun := *j.Schedule.At
        return &nextRun

    case CronScheduleCron:
        expr := strings.TrimSpace(j.Schedule.Expr)
        if expr == "" {
            return nil
        }

        // TODO:
        // 1. 调用 cron expression helper 计算 from 之后的下一次触发时间
        // 2. 计算失败时返回 nil，而不是 panic
        // 3. 这里的失败应该是“表达式无效或不可计算”，不是进程级错误
        //
        // next, err := nextCronTime(expr, from)
        // if err != nil { return nil }
        // return &next
        return nil

    default:
        return nil
    }
}
```

你从这一步应该学到的是：

- 领域模型里的方法应该只表达“业务规则”
- 不要把 CLI 输入解析、文件路径、日志之类的东西塞进 domain

### 4.2 `internal/service/cron/service.go`

当前这里已经基本正确，真正要补的只有一个状态迁移分支。

```go
func (s *Service) advanceJobAfterRun(job *model.CronJob, runAt time.Time) {
    if job == nil {
        return
    }

    switch job.Schedule.Kind {
    case model.CronScheduleEvery:
        job.NextRunAt = job.ComputeNextRun(runAt)

    case model.CronScheduleAt:
        // 单次任务执行后直接禁用。
        job.Enabled = false
        job.NextRunAt = nil

    case model.CronScheduleCron:
        // cron schedule 与 every 的共同点是：
        // 执行后仍然保持启用，只是推进下一次时间。
        job.NextRunAt = job.ComputeNextRun(runAt)

    default:
        job.NextRunAt = nil
    }
}
```

你从这一步应该学到的是：

- `service` 层负责“状态迁移”
- `domain` 层负责“规则计算”
- 不要把“执行一次后应该禁用吗”塞回 repository

### 4.3 `cmd/tinybot/run.go`

这里建议新增一个非常直接的命令，而不是一开始就把 `add/add-at/add-cron` 折叠成一套复杂参数解析。

对学习阶段来说，显式命令更容易验证。

```go
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
        return fmt.Sprintf("cron=%s", job.Schedule.Expr)

    default:
        return fmt.Sprintf("schedule=%s", job.Schedule.Kind)
    }
}

// tinybot cron add-cron <name> <expr> <prompt>
func runCronAddCron(args []string, out io.Writer, repo cronservice.Repository) error {
    if len(args) < 3 {
        return fmt.Errorf("usage: tinybot cron add-cron <name> <expr> <prompt>")
    }

    name := strings.TrimSpace(args[0])
    expr := strings.TrimSpace(args[1])
    prompt := strings.TrimSpace(strings.Join(args[2:], " "))

    if name == "" {
        return fmt.Errorf("cron add-cron: name is required")
    }
    if expr == "" {
        return fmt.Errorf("cron add-cron: expr is required")
    }
    if prompt == "" {
        return fmt.Errorf("cron add-cron: prompt is required")
    }

    ctx := context.Background()
    jobs, err := repo.ListJobs(ctx)
    if err != nil {
        return fmt.Errorf("cron add-cron list jobs: %w", err)
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
            Kind: model.CronScheduleCron,
            Expr: expr,
        },
        CreatedAt: now,
        UpdatedAt: now,
    }
    job.NextRunAt = job.ComputeNextRun(now)

    if err := job.Validate(); err != nil {
        return fmt.Errorf("cron add-cron validate job: %w", err)
    }
    if job.NextRunAt == nil {
        return fmt.Errorf("cron add-cron: expression did not produce next run time")
    }

    jobs = append(jobs, job)
    if err := repo.SaveJobs(ctx, jobs); err != nil {
        return fmt.Errorf("cron add-cron save jobs: %w", err)
    }

    _, _ = fmt.Fprintf(out, "Added cron job %s (%s) with expr %s\n", job.Name, job.ID, expr)
    return nil
}
```

你从这一步应该学到的是：

- CLI 解析属于 transport/entrypoint 责任
- 不要让 domain model 去解析 `os.Args`
- 学习阶段优先选择显式命令，而不是提早做复杂参数复用

### 4.4 `internal/repository/cronrepo/file_cron_repo.go`

这个文件很可能几乎不用改逻辑。

原因是它现在已经通过 JSON 编解码保存整个 `model.CronJob`。
只要你的结构体字段加对了、`Validate()` 允许 `cron`，repository round-trip 通常就自然成立。

你真正要补的，是测试。

```go
func TestFileCronRepository_SaveJobs_ThenListJobs_WithCronExpr(t *testing.T) {
    workspace := t.TempDir()
    repo := NewFileCronRepository(workspace)

    now := time.Now().UTC().Truncate(time.Second)
    next := now.Add(1 * time.Hour) // 这里只是示意，真正断言时更建议只校验字段 round-trip

    want := []model.CronJob{
        {
            ID:         "job-cron",
            Name:       "weekday-check",
            Enabled:    true,
            Prompt:     "check workspace status",
            SessionKey: "cron:job-cron",
            Schedule: model.CronSchedule{
                Kind: model.CronScheduleCron,
                Expr: "0 9 * * 1-5",
            },
            CreatedAt: now,
            UpdatedAt: now,
            NextRunAt: &next,
        },
    }

    if err := repo.SaveJobs(context.Background(), want); err != nil {
        t.Fatalf("SaveJobs() error: %v", err)
    }

    got, err := repo.ListJobs(context.Background())
    if err != nil {
        t.Fatalf("ListJobs() error: %v", err)
    }

    if len(got) != 1 {
        t.Fatalf("len(got) = %d, want 1", len(got))
    }
    if got[0].Schedule.Kind != model.CronScheduleCron {
        t.Fatalf("got[0].Schedule.Kind = %q, want %q", got[0].Schedule.Kind, model.CronScheduleCron)
    }
    if got[0].Schedule.Expr != "0 9 * * 1-5" {
        t.Fatalf("got[0].Schedule.Expr = %q, want %q", got[0].Schedule.Expr, "0 9 * * 1-5")
    }
}
```

你从这一步应该学到的是：

- repository test 重点验证“持久化行为”
- 不要在 repository test 里重复验证全部 schedule 算法

## 5. 推荐测试顺序

这是我建议你按顺序完成的实现路径。

### Step 1

先写 domain tests，再写实现。

目标文件：

- `internal/domain/model/cron_test.go`

先补这些用例：

1. `Validate()` 接受合法 cron expression
2. `Validate()` 拒绝空 expression
3. `ComputeNextRun()` 对合法 expression 返回非空时间
4. `ComputeNextRun()` 对非法 expression 返回 `nil`

为什么先写这些：

- 你会先固定“领域行为”
- 后面的 service/repository/CLI 都只是消费这个能力

### Step 2

补 `internal/domain/model/cron.go` 的字段、常量、校验和计算。

这一阶段不要碰 CLI。

原因：

- 如果 domain 行为还没稳定，CLI 很容易跟着反复改

### Step 3

补 repository round-trip 测试。

目标文件：

- `internal/repository/cronrepo/file_cron_repo_test.go`

只验证：

- 存得进去
- 读得出来
- `kind` 和 `expr` 不丢

### Step 4

补 cron service 的状态迁移测试。

目标文件：

- `internal/service/cron/service_test.go`

新增一个测试场景：

- due 的 `cron` job 执行后
- `LastRunAt` 被更新
- `LastResult` 或 `LastError` 被更新
- `NextRunAt` 被重新推进
- `Enabled` 仍然是 `true`

### Step 5

最后再补 CLI。

目标文件：

- `cmd/tinybot/run.go`
- `cmd/tinybot/run_test.go`

建议先只做一条 happy path：

- `tinybot cron add-cron weekday "0 9 * * 1-5" "check inbox"`
- 然后 `tinybot cron list`
- 输出里能看到 job name 和 `cron=0 9 * * 1-5`

## 6. 你实现时最容易犯的错

### 错误 1：把 cron expression 解析逻辑塞进 CLI

不要这样做。

CLI 只负责拿字符串参数。
真正的规则校验应该落在 `domain/model`。

否则后续 gateway、配置文件导入、HTTP API 一旦也要创建 job，你会复制逻辑。

### 错误 2：在 repository 里决定下一次运行时间

也不要这样做。

repository 只负责存取，不负责业务规则。

“下一次执行时间怎么算”属于领域规则。

### 错误 3：一开始就把 `deliver/channel/to` 一起做掉

这一步不值得。

因为那会牵扯：

- `CronJob` 结构扩展
- `cron service` 是否要知道发送能力
- gateway/runtime 是否要注入额外 sender
- direct/gateway 模式下的行为差异

它不是一个“小闭环”，而是下一阶段的事情。

## 7. 完成这一步后的下一个里程碑

当你完成 `cron expression` 支持之后，下一步我会建议你做：

- 为 cron 增加 `deliver/channel/to` payload，但先只接入 gateway runtime

原因是那时：

- schedule 规则已经稳定
- repository 格式已经具备扩展基础
- 你可以把注意力集中到“service 到 transport 的边界设计”

这会比现在直接做 delivery 更稳。

## 8. 建议你实际执行的命令

每完成一步就跑一层，不要最后一次性跑全部。

```powershell
go test ./internal/domain/model
go test ./internal/repository/cronrepo
go test ./internal/service/cron
go test ./cmd/tinybot
```

如果你想更稳一点，最后再跑：

```powershell
go test ./...
```

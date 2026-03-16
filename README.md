# tinybot

一个轻量级的 AI 助手，使用 Go 语言重写。

**tinybot** 是一个超轻量级的个人 AI 助手，集成了 LLM 对话、工具执行、定时任务、持久化记忆和多通道消息功能。

这是 Python 项目 [nanobot](https://github.com/HKUDS/nanobot) 的 Go 语言重写版本。

## 功能特性

- **LLM 对话** - 支持上下文感知的自然语言对话
- **工具执行** - 内置文件操作、网络搜索等工具
- **持久化记忆** - 每日笔记和长期记忆存储
- **定时任务** - 基于 Cron 的灵活任务调度
- **多通道支持** - 支持 Console 和 Telegram 通道
- **会话管理** - 对话持久化与自动压缩
- **思考模式** - 可视化展示复杂任务的推理过程

## 内置工具

| 工具 | 描述 |
|------|------|
| `exec` | 执行 Shell 命令 |
| `read_file` | 读取文件内容 |
| `write_file` | 写入或创建文件 |
| `edit_file` | 搜索替换编辑文件 |
| `list_dir` | 列出目录内容 |
| `web_search` | 网络搜索 |
| `web_fetch` | 获取并提取网页内容 |
| `message` | 向通道发送消息 |

## 快速开始

### 环境要求

- Go 1.21+
- LLM API 密钥（支持 Qwen、OpenAI、DeepSeek 或 Ollama）

### 安装

```bash
# 克隆仓库
git clone https://github.com/yourusername/tinybot.git
cd tinybot

# 编译
go build -o tinybot.exe ./cmd/tinybot
```

### 初始化

```bash
# 初始化工作空间
./tinybot.exe onboard

# 设置 API 密钥
export QWEN_API_KEY=your_api_key_here
# 或使用 OpenAI
export OPENAI_API_KEY=your_api_key_here
```

### 使用

```bash
# 直接对话
./tinybot.exe "今天天气怎么样？"

# 查看状态
./tinybot.exe status

# 启动网关模式（长期运行）
./tinybot.exe gateway
```

## CLI 命令

```bash
tinybot <message>              # 直接对话（非命令输入视为消息）
tinybot onboard                # 初始化工作空间
tinybot status                 # 显示工作空间状态
tinybot gateway                # 启动网关模式

# 定时任务管理
tinybot cron list                                    # 列出所有任务
tinybot cron add <名称> <秒数> <提示词>               # 添加间隔任务
tinybot cron add-cron <名称> <cron表达式> <提示词>    # 添加 Cron 任务
tinybot cron add-at <名称> <RFC3339时间> <提示词>     # 添加一次性任务
tinybot cron remove <任务ID>                         # 删除任务
tinybot cron run-once                                # 执行到期任务
```

## 配置

配置文件位于项目根目录的 `config.json`：

> 也可以通过环境变量 `TINYBOT_CONFIG` 指定自定义配置文件路径。

```json
{
  "agents": {
    "workspace": "./workspace",
    "max_tokens": 8192,
    "temperature": 0.7,
    "max_tool_iterations": 20,
    "enable_thinking": true,
    "consolidation": {
      "enabled": true,
      "token_limit": 60000,
      "keep_recent": 10
    }
  },
  "providers": {
    "active": "qwen",
    "list": {
      "qwen": {
        "kind": "qwen",
        "api_key": "...",
        "api_base": "https://dashscope.aliyuncs.com/compatible-mode/v1",
        "model": "qwen3-max"
      }
    }
  },
  "channels": {
    "console": { "enabled": true },
    "telegram": {
      "enabled": false,
      "token": ""
    }
  }
}
```

### 环境变量

| 变量 | 描述 |
|------|------|
| `QWEN_API_KEY` | Qwen API 密钥 |
| `QWEN_API_BASE` | Qwen API 地址 |
| `QWEN_MODEL` | Qwen 模型名称 |
| `OPENAI_API_KEY` | OpenAI API 密钥 |
| `TELEGRAM_BOT_TOKEN` | Telegram Bot Token |

## 项目架构

```
cmd/tinybot/           # CLI 入口
internal/
  app/                 # 依赖注入根
  domain/model/        # 核心类型定义
  service/
    chat/              # 对话编排
    cron/              # 任务调度
    heartbeat/         # 心跳评估
  repository/
    sessionrepo/       # 会话持久化
    cronrepo/          # 定时任务持久化
  transport/
    bus/               # 消息总线
    channel/           # 通道实现
    gateway/           # 网关循环
    runtime/           # 周期性运行器
  adapters/
    provider/          # LLM 提供者适配器
    tool/              # 工具实现
    workspace/         # 记忆与引导文件
```

## 记忆系统

tinybot 通过以下方式维护持久化记忆：

- **MEMORY.md** - 长期记忆（事实、偏好）
- **每日笔记** - YYYY-MM-DD.md 格式的每日记录
- **自动提取** - LLM 自动提取重要信息

```bash
# 记忆文件存储位置：
workspace/memory/
├── MEMORY.md           # 长期记忆
├── 2026-03-16.md       # 今日笔记
└── 2026-03-15.md       # 昨日笔记
```

## 开发

```bash
# 运行所有测试
go test ./...

# 运行测试并显示覆盖率
go test -cover ./...

# 运行指定包的测试
go test -v ./internal/service/chat/...
```

## 与 nanobot 对比

| 功能 | nanobot (Python) | tinybot (Go) |
|------|------------------|--------------|
| LLM 对话 | ✅ | ✅ |
| 工具执行 | ✅ | ✅ |
| 记忆系统 | ✅ | ✅ |
| 定时任务 | ✅ | ✅ |
| Telegram 通道 | ✅ | ✅ |
| 思考模式 | ❌ | ✅ |
| 自动重试 | ❌ | ✅ |
| CLI 状态查看 | ❌ | ✅ |

## 许可证

MIT

## 致谢

- [nanobot](https://github.com/HKUDS/nanobot) - 原始 Python 实现
- [openai-go](https://github.com/openai/openai-go) - OpenAI Go SDK
- [cron](https://github.com/robfig/cron) - Go Cron 库

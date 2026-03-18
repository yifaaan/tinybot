package sessionrepo

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"tinybot/internal/domain/model"
	chatservice "tinybot/internal/service/chat"
)

var _ chatservice.SessionRepository = (*FileSessionRepository)(nil)

// 这个编译期断言用来确保 repository 与 service 层接口没有悄悄漂移。
// 一旦 chatservice.SessionRepository 的方法签名变化，这里会在编译期直接报错。

// FileSessionRepository 是 service/chat 当前使用的文件型 session 存储实现。
//
// 它负责三件具体的事情：
// 1. 把 session key 映射成稳定文件路径
// 2. 以 JSONL 格式读写完整会话
// 3. 在单进程内缓存已加载的 session，减少重复读盘
//
// 为什么 cache 放在 repository 而不是 service：
// - cache 是存储实现细节，不是聊天编排逻辑
// - service 只应关心"拿到一个 session"，不应关心它来自内存还是磁盘
type FileSessionRepository struct {
	WorkSpace  string
	SessionDir string
	// LegacySessionDir 预留给历史目录兼容或未来迁移使用。
	// 当前代码尚未真正使用它，但先保留字段可以提醒后续迁移时注意兼容问题。
	//
	// TODO: 如果确认不再需要旧目录兼容，可以删除这个字段；否则应补一条明确的迁移逻辑。
	LegacySessionDir string
	cache            map[string]*model.Session
}

// NewFileSessionRepository 创建一个面向 workspace 的文件型 session repository。
//
// 参数：
// - workspace: 当前 workspace 路径
//
// 返回：
// - *FileSessionRepository: 初始化后的 repository
//
// 构造时会尽量先创建 sessions 目录，原因是 direct chat/CLI 首次运行时不应该要求用户先手动建目录。
// 当前实现选择"静默尝试创建"而不是把构造函数改成返回 error，是为了让组合根更简单。
//
// TODO: 如果后续希望启动阶段对磁盘权限问题更敏感，可以把这里改成返回 (*FileSessionRepository, error)。
func NewFileSessionRepository(workspace string) *FileSessionRepository {
	sessionDir := filepath.Join(workspace, "sessions")
	_ = os.MkdirAll(sessionDir, 0755)

	return &FileSessionRepository{
		WorkSpace:  workspace,
		SessionDir: sessionDir,
		cache:      make(map[string]*model.Session),
	}
}

// GetSessionPath 返回给定 session key 对应的 JSONL 文件路径。
//
// 参数：
// - key: 稳定会话 key，例如 "cli:direct"、"telegram:123"
//
// 返回：
// - string: 对应的 session 文件路径
//
// 这里会先把 key 中的冒号替换成下划线，再做一次安全文件名清洗。
// 这样做的原因是：
// - session key 对业务来说应该保持可读
// - 文件系统路径则需要尽量避免平台相关非法字符
func (m *FileSessionRepository) GetSessionPath(key string) string {
	safeKey := safeFilename(strings.ReplaceAll(key, ":", "_"))
	return filepath.Join(m.SessionDir, fmt.Sprintf("%s.jsonl", safeKey))
}

// safeFilename 把不适合出现在文件名中的字符替换成下划线。
//
// 当前实现优先追求"路径稳定且跨平台可用"，而不是保留原始 key 的每个字符细节。
func safeFilename(name string) string {
	return regexp.MustCompile(`[<>:"/\\|?*]`).ReplaceAllString(name, "_")
}

// GetOrCreateSession 返回缓存中的 session、磁盘中的 session，或一个新建 session。
//
// 参数：
// - ctx: 请求级上下文
// - key: 稳定 session key
//
// 返回：
// - *model.Session: 当前可用会话
// - error: 上下文已取消等硬失败时返回错误
//
// 这里的关键设计取舍是"读取失败时尽量降级成新 session"，而不是让一次坏文件拖垮整个聊天流程。
// 这和测试里的预期一致：损坏 JSONL 会回退到新会话，让 direct chat 仍然可用。
//
// FIXME: 当前实现会吞掉 LoadSession 的具体错误，虽然对可用性友好，但可观测性较差。
// 后续至少应补日志或指标，否则用户很难知道某个 session 文件其实已经损坏。
func (m *FileSessionRepository) GetOrCreateSession(ctx context.Context, key string) (*model.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("get or create session: %w", err)
	}

	if s, ok := m.cache[key]; ok {
		return s, nil
	}

	s, _ := m.LoadSession(key)
	if s == nil {
		s = model.NewSession(key)
	}
	m.cache[key] = s
	return s, nil
}

// Session JSONL 文件格式：
//
// 第 1 行是 metadata：
//
//	{
//	  "_type": "metadata",
//	  "key": "<session_key>",
//	  "metadata": { ... },
//	  "created_at": "RFC3339",
//	  "updated_at": "RFC3339",
//	  "last_consolidated": <int>
//	}
//
// 第 2..N 行是 message：
//
//	{
//	  "role": "user|assistant|tool|system",
//	  "content": "...",
//	  "created_at": "RFC3339",
//	  "tool_calls": ...,
//	  "tool_call_id": "...",
//	  "name": "..."
//	}
//
// 选择 JSONL 而不是一个大 JSON 数组，主要是为了：
// - 文件更容易用肉眼查看
// - 后续如果需要调试或做部分恢复，更容易直接按行检查
// - 第一行单独放 metadata，也让"列 session 列表"这种操作更便宜
//
// LoadSession 负责把这个 JSONL 文件重新还原成领域层的 Session。
//
// 参数：
// - key: 稳定 session key
//
// 返回：
// - *model.Session: 读出的 session
// - error: 文件不存在、格式错误、读取失败时返回错误
func (m *FileSessionRepository) LoadSession(key string) (*model.Session, error) {
	path := m.GetSessionPath(key)
	_, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	messages := make([]*model.Message, 0)
	metadata := make(map[string]any)
	sessionCreatedAt := time.Now()
	lastConsolidated := 0

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		data := make(map[string]any)
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			return nil, err
		}
		if typ, _ := data["_type"].(string); typ == "metadata" {
			if md, ok := data["metadata"].(map[string]any); ok {
				metadata = md
			}
			if t, ok := parseJSONTime(data["created_at"]); ok {
				sessionCreatedAt = t
			}
			lastConsolidated = toInt(data["last_consolidated"])
			continue
		}

		// message 行同时兼容 created_at 和历史字段 timestamp，
		// 这是为了在格式逐步演进时尽量保持老数据可读。
		msgCreatedAt := time.Now()
		if t, ok := parseJSONTime(data["created_at"]); ok {
			msgCreatedAt = t
		} else if t, ok := parseJSONTime(data["timestamp"]); ok {
			msgCreatedAt = t
		}

		messages = append(messages, &model.Message{
			Role:       toString(data["role"]),
			Content:    toString(data["content"]),
			CreatedAt:  msgCreatedAt,
			ToolCalls:  data["tool_calls"],
			ToolCallID: toString(data["tool_call_id"]),
			Name:       toString(data["name"]),
		})
	}
	return &model.Session{
		Key:              key,
		Messages:         messages,
		CreatedAt:        sessionCreatedAt,
		UpdatedAt:        time.Now(),
		Metadata:         metadata,
		LastConsolidated: lastConsolidated,
	}, nil
}

// SaveSession 把完整 session 重新写回 JSONL 文件。
//
// 参数：
// - ctx: 请求级上下文
// - s: 需要保存的 session
//
// 返回：
// - error: 写盘失败时返回错误
//
// 当前实现采用"整文件重写"而不是增量 append，原因是：
// - 第一阶段代码更简单，状态更容易推理
// - metadata 和 messages 总能保持一致，不会因为半途中断留下难恢复状态
// - 对 tinybot 当前规模来说，优先可读性和稳定性比写入性能更重要
//
// TODO: 如果未来 session 很长、写盘压力明显增大，可以考虑追加写入和定期 compact。
func (m *FileSessionRepository) SaveSession(ctx context.Context, s *model.Session) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	if s == nil {
		return errors.New("session is nil")
	}
	path := m.GetSessionPath(s.Key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("save session mkdir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)

	metadataLine := map[string]any{
		"_type":             "metadata",
		"key":               s.Key,
		"metadata":          s.Metadata,
		"created_at":        s.CreatedAt.Format(time.RFC3339),
		"updated_at":        s.UpdatedAt.Format(time.RFC3339),
		"last_consolidated": s.LastConsolidated,
	}
	jsonData, err := json.Marshal(metadataLine)
	if err != nil {
		return err
	}
	if _, err := writer.Write(jsonData); err != nil {
		return fmt.Errorf("save session write metadata: %w", err)
	}
	if _, err := writer.WriteString("\n"); err != nil {
		return fmt.Errorf("save session write metadata newline: %w", err)
	}
	for _, msg := range s.Messages {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("save session: %w", err)
		}
		jsonData, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if _, err := writer.Write(jsonData); err != nil {
			return fmt.Errorf("save session write message: %w", err)
		}
		if _, err := writer.WriteString("\n"); err != nil {
			return fmt.Errorf("save session write message newline: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("save session flush: %w", err)
	}
	m.cache[s.Key] = s
	return nil
}

// Invalidate 主动删除某个 session 的内存缓存。
//
// 参数：
// - key: session key
//
// 这个方法存在的意义是给"外部文件已变化"或"测试希望强制重读磁盘"这样的场景留一个明确入口。
// 否则调用方只能隐式依赖进程重启才能看到最新磁盘状态。
func (m *FileSessionRepository) Invalidate(key string) {
	delete(m.cache, key)
}

// ListSessions 返回按更新时间倒序排列的 session metadata 列表。
//
// 返回：
// - []map[string]any: 轻量级会话列表信息
// - error: 扫描或解析失败时返回错误
//
// 为什么这里不直接加载完整 session：
// - 列表页/状态命令通常只关心 key、时间、路径等摘要信息
// - 只读 metadata 第一行比加载整份 transcript 更便宜
//
// 如果 metadata 行里没有 key，当前实现会从文件名推导一个回退 key。
// 这是为了兼容早期或不完整数据，避免"列表命令因为老文件缺字段而直接失效"。
func (m *FileSessionRepository) ListSessions() ([]map[string]any, error) {
	filenames, err := filepath.Glob(filepath.Join(m.SessionDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}

	sessions := make([]map[string]any, 0, len(filenames))
	for _, filename := range filenames {
		f, err := os.Open(filename)
		if err != nil {
			continue // Skip files we cannot read
		}

		reader := bufio.NewReader(f)
		metadata := make(map[string]any)
		key := ""
		createdAt := ""
		updatedAt := ""
		messageCount := 0
		firstUserContent := ""

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				break
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			data := make(map[string]any)
			if err := json.Unmarshal([]byte(line), &data); err != nil {
				break
			}

			if typ, _ := data["_type"].(string); typ == "metadata" {
				key, _ = data["key"].(string)
				createdAt, _ = data["created_at"].(string)
				updatedAt, _ = data["updated_at"].(string)
				if md, ok := data["metadata"].(map[string]any); ok {
					metadata = md
				}
				continue
			}

			// Count messages and capture first user message
			messageCount++
			if role, _ := data["role"].(string); role == "user" && firstUserContent == "" {
				if content, ok := data["content"].(string); ok {
					firstUserContent = content
				}
			}
		}
		_ = f.Close()

		if key == "" {
			stem := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
			key = strings.Replace(stem, "_", ":", 1)
		}

		sessions = append(sessions, map[string]any{
			"key":               key,
			"created_at":        createdAt,
			"updated_at":        updatedAt,
			"metadata":          metadata,
			"path":              filename,
			"message_count":     messageCount,
			"first_user_content": firstUserContent,
		})
	}
	sort.Slice(sessions, func(i, j int) bool {
		ti, errI := time.Parse(time.RFC3339, fmt.Sprint(sessions[i]["updated_at"]))
		tj, errJ := time.Parse(time.RFC3339, fmt.Sprint(sessions[j]["updated_at"]))
		if errI != nil || errJ != nil {
			return fmt.Sprint(sessions[i]["updated_at"]) < fmt.Sprint(sessions[j]["updated_at"])
		}
		return ti.After(tj)
	})
	return sessions, nil
}

// toString 把动态 JSON 值安全转成字符串。
// 这里采用保守转换，拿不到字符串就返回空串，避免把 repository 逻辑变成一堆类型断言分支。
func toString(v any) string {
	s, _ := v.(string)
	return s
}

// toInt 把动态 JSON 值尽量转成 int。
// 当前只覆盖实际会话文件中可能出现的几种数值形态，优先保证兼容性。
func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// parseJSONTime 解析 JSON 中的 RFC3339 时间字符串。
//
// 返回 false 时，调用方通常会回退到 time.Now()，
// 这样即使遇到历史坏数据，也不会让整个读取流程直接崩掉。
func parseJSONTime(v any) (time.Time, bool) {
	s, ok := v.(string)
	if !ok || s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

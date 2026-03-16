package provider

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"net"
	"strings"
	"time"
	"tinybot/internal/domain/model"
	"tinybot/internal/service/chat"
	"tinybot/internal/utils/logger"
)

type RetryClient struct {
	inner chat.CompletionClient

	maxRetries   int           // 最大重试次数（不含首次调用）
	initialDelay time.Duration // 首次重试的等待时间
	maxDelay     time.Duration // 单次等待时间的最大值
}

// NewRetryClient 创建重试装饰器。
//
// 参数：
// - inner: 被包装的真实 client
// - maxRetries: 最大重试次数，<= 0 时使用默认值 3
// - initialDelay: 首次重试延迟，<= 0 时使用默认值 1s
// - maxDelay: 延迟上限，<= 0 时使用默认值 30s
//
// 返回：
// - *RetryClient: 包装后的 client，实现相同接口
func NewRetryClient(
	inner chat.CompletionClient,
	maxRetries int,
	initialDelay time.Duration,
	maxDelay time.Duration,
) *RetryClient {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if initialDelay <= 0 {
		initialDelay = 1 * time.Second
	}
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}
	return &RetryClient{
		inner:        inner,
		maxRetries:   maxRetries,
		initialDelay: initialDelay,
		maxDelay:     maxDelay,
	}
}

// Chat 实现 CompletionClient 接口，在调用失败时自动重试。
//
// 重试流程：
// 1. 调用内部 client
// 2. 如果成功，直接返回
// 3. 如果失败，判断错误是否可重试（isRetryable）
// 4. 可重试 → 等待指数退避时间 → 回到步骤 1
// 5. 不可重试 / 达到上限 / ctx 取消 → 返回最后一次错误
func (r RetryClient) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error) {
	var lastErr error

	for attempt := 0; attempt < r.maxRetries+1; attempt++ {
		if attempt > 0 {
			delay := r.backoffDelay(attempt)
			logger.Warn("retrying llm call", "attempt", attempt, "max_retries", r.maxRetries, "delay", delay, "last_error", lastErr)

			select {
			case <-ctx.Done():
				return model.LLMResponse{}, ctx.Err()
			case <-time.After(delay):
				// 等待时间结束，继续重试
			}
		}

		resp, err := r.inner.Chat(ctx, messages, tools, maxTokens, temperature)
		if err == nil {
			if attempt > 0 {
				logger.Info("llm call succeeded after retry", "attempt", attempt, "max_retries", r.maxRetries)
			}
			return resp, nil
		}
		lastErr = err

		if !isRetryable(err) {
			return model.LLMResponse{}, err
		}
	}
	return model.LLMResponse{}, lastErr
}

func (r *RetryClient) ChatStream(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) <-chan model.StreamEvent {
	streamer, ok := r.inner.(chat.StreamingCompletionClient)
	if !ok {
		ch := make(chan model.StreamEvent, 1)
		ch <- model.StreamEvent{Kind: model.StreamEventError, Err: errors.New("streaming not supported")}
		close(ch)
		return ch
	}

	ch := make(chan model.StreamEvent)
	go func() {
		defer close(ch)

		var lastErr error
		for attempt := 0; attempt < r.maxRetries+1; attempt++ {
			if attempt > 0 {
				delay := r.backoffDelay(attempt)
				logger.Warn("retrying llm stream", "attempt", attempt, "max_retries", r.maxRetries, "delay", delay, "last_error", lastErr)
				select {
				case <-ctx.Done():
					ch <- model.StreamEvent{Kind: model.StreamEventError, Err: ctx.Err()}
					return
				case <-time.After(delay):
					// 等待时间结束，继续重试
				}
			}

			innerCh := streamer.ChatStream(ctx, messages, tools, maxTokens, temperature)
			success := r.forwardStream(ctx, innerCh, ch, attempt < r.maxRetries)
			if success {
				return
			}
		}
	}()
	return ch
}

// forwardStream 消费内部 stream channel，转发事件到外部 channel。
//
// 参数：
// - canRetry: 是否还有重试配额（用于决定首个 error 事件的处理方式）
//
// 返回：
// - true: 流正常完成或遇到不可重试的错误（调用方应停止）
// - false: 首个事件就是可重试的错误（调用方应重试）
func (r *RetryClient) forwardStream(ctx context.Context, innerCh <-chan model.StreamEvent, outerCh chan<- model.StreamEvent, canRetry bool) bool {
	firstEvent := true

	for event := range innerCh {
		if firstEvent && event.Kind == model.StreamEventError && canRetry && isRetryable(event.Err) {
			return false
		}
		firstEvent = false

		select {
		case <-ctx.Done():
			return true
		case outerCh <- event:
			// 事件转发成功，继续消费内部流
			continue
		}
	}
	return true
}

// backoffDelay 计算第 attempt 次重试的等待时间。
//
// 算法：指数退避 + 随机抖动（jitter）
//
// 为什么要加 jitter：
// - 如果多个 goroutine 同时失败，不加 jitter 它们会在相同时刻重试
// - 这会造成"惊群效应"（thundering herd），让 API 服务器压力更大
// - jitter 让每次重试时间略有偏移，分散请求压力
//
// 公式：delay = min(initialDelay * 2^(attempt-1) * (0.5 + rand(0,0.5)), maxDelay)
func (r *RetryClient) backoffDelay(attempt int) time.Duration {
	// 指数增长：1s → 2s → 4s → 8s → ...
	backoff := float64(r.initialDelay) * math.Pow(2, float64(attempt-1))
	// 加入 jitter：在 [50%, 100%] 范围内随机浮动
	jitter := 0.5 + rand.Float64()*0.5
	backoff *= jitter
	delay := time.Duration(backoff)
	if delay > r.maxDelay {
		delay = r.maxDelay
	}
	return delay
}

// isRetryable 判断一个错误是否值得重试。
//
// 可重试的典型场景：
// - 网络临时故障（net.Error 且 Temporary() == true）
// - 连接被重置（connection reset）
// - 服务端 5xx 错误（502, 503, 429 rate limit）
// - 上下文超时（deadline exceeded，但不是主动取消）
//
// 不可重试的典型场景：
// - 认证失败（401, 403）
// - 请求格式错误（400）
// - 主动取消上下文（context canceled）
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	// context.Canceled 是用户主动取消，不应重试
	if errors.Is(err, context.Canceled) {
		return false
	}
	// context.DeadlineExceeded 是超时，可以重试
	// （可能是单次调用超时，主 ctx 还有余量）
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// 网络层临时错误
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	// 根据错误消息中的 HTTP 状态码关键词判断
	msg := strings.ToLower(err.Error())
	// 429 Too Many Requests（限流）
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") {
		return true
	}
	// 5xx 服务端错误
	if strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "504") {
		return true
	}
	// 连接级错误
	if strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "eof") {
		return true
	}
	return false
}

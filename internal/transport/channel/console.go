package channel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"tinybot/internal/domain/model"
	"tinybot/internal/transport"
)

// ConsoleChannelConfig configures the stdin/stdout channel used by the local gateway.
type ConsoleChannelConfig struct {
	ChatID     string
	SenderID   string
	Prompt     string
	ShowPrefix bool
}

// ConsoleChannel reads stdin into inbound messages and writes outbound replies to stdout.
//
// Responsibilities:
//   - translate console input lines into inbound messages
//   - render outbound messages back to the console
type ConsoleChannel struct {
	bus    transport.MessageBus
	cfg    ConsoleChannelConfig
	input  io.Reader
	output io.Writer

	outMu sync.Mutex
}

// NewConsoleChannel creates a console-backed channel with default IDs and prompt text.
func NewConsoleChannel(bus transport.MessageBus, cfg ConsoleChannelConfig, input io.Reader, output io.Writer) *ConsoleChannel {
	if strings.TrimSpace(cfg.ChatID) == "" {
		cfg.ChatID = "gateway"
	}
	if strings.TrimSpace(cfg.SenderID) == "" {
		cfg.SenderID = "console-user"
	}
	if strings.TrimSpace(cfg.Prompt) == "" {
		cfg.Prompt = "You>"
	}
	if input == nil {
		input = os.Stdin
	}
	if output == nil {
		output = os.Stdout
	}
	return &ConsoleChannel{
		bus:    bus,
		cfg:    cfg,
		input:  input,
		output: output,
	}
}

// Name returns the channel identifier used for console traffic.
func (c *ConsoleChannel) Name() model.Channel {
	return model.ChannelCLI
}

// Start reads console input until EOF, context cancellation, or a bus error.
// 把终端输入变成 inbound，然后写进 bus
func (c *ConsoleChannel) Start(ctx context.Context) error {
	lines := make(chan string)
	errCh := make(chan error, 1)

	go func() {
		defer close(lines)

		scanner := bufio.NewScanner(c.input)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		errCh <- scanner.Err()
	}()

	if err := c.writePrompt(); err != nil {
		return fmt.Errorf("console channel write prompt: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("console channel scan: %w", err)
			}
			return nil
		case line, ok := <-lines:
			if !ok {
				return nil
			}

			line = strings.TrimSpace(line)
			if line == "" {
				if err := c.writePrompt(); err != nil {
					return fmt.Errorf("console channel write prompt: %w", err)
				}
				continue
			}

			msg := model.InboundMessage{
				ID:           fmt.Sprintf("console-%d", time.Now().UnixNano()),
				Channel:      model.ChannelCLI,
				SenderID:     c.cfg.SenderID,
				ChatID:       c.cfg.ChatID,
				Content:      line,
				CreatedAt:    time.Now(),
				StreamWriter: c,
			}
			if err := c.bus.PublishInbound(ctx, msg); err != nil {
				if errors.Is(err, transport.ErrBusClosed) || errors.Is(err, context.Canceled) {
					return nil
				}
				return fmt.Errorf("publish inbound: %w", err)
			}
		}
	}
}

// Send writes one outbound reply to the console.
// 把从 bus 派发下来的 outbound 真正写回终端
func (c *ConsoleChannel) Send(ctx context.Context, out model.OutboundMessage) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("console channel send: %w", ctx.Err())
	default:
		if strings.TrimSpace(out.Content) == "" {
			return nil
		}
		if out.Streamed {
			return c.writePromptOnly()
		}
		if err := c.writeReplyAndPrompt(out.Content); err != nil {
			return fmt.Errorf("console channel write reply: %w", err)
		}
		return nil
	}
}

func (c *ConsoleChannel) writePrompt() error {
	c.outMu.Lock()
	defer c.outMu.Unlock()

	_, err := fmt.Fprintf(c.output, "%s ", c.cfg.Prompt)
	return err
}

// writePromptOnly 只打印换行和提示符，用于流式输出结束后
func (c *ConsoleChannel) writePromptOnly() error {
	c.outMu.Lock()
	defer c.outMu.Unlock()

	if _, err := fmt.Fprintln(c.output); err != nil {
		return err
	}
	_, err := fmt.Fprint(c.output, c.cfg.Prompt)
	return err
}

func (c *ConsoleChannel) writeReplyAndPrompt(text string) error {
	c.outMu.Lock()
	defer c.outMu.Unlock()

	if c.cfg.ShowPrefix {
		if _, err := fmt.Fprintf(c.output, "tinybot> %s\n", text); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(c.output, text); err != nil {
			return err
		}
	}

	_, err := fmt.Fprint(c.output, c.cfg.Prompt)
	return err
}

func (c *ConsoleChannel) WriteDelta(delta string) error {
	c.outMu.Lock()
	defer c.outMu.Unlock()

	_, err := fmt.Fprint(c.output, delta)
	return err
}

func (c *ConsoleChannel) Flush() error {
	c.outMu.Lock()
	defer c.outMu.Unlock()

	if flusher, ok := c.output.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

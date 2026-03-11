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
	"tinybot/internal/ports"
)

type ConsoleChannelConfig struct {
	ChatID     string
	SenderID   string
	Prompt     string
	ShowPrefix bool
}

type ConsoleChannel struct {
	bus    ports.MessageBus
	cfg    ConsoleChannelConfig
	input  io.Reader
	output io.Writer

	outMu sync.Mutex
}

func NewConsoleChannel(bus ports.MessageBus, cfg ConsoleChannelConfig, input io.Reader, output io.Writer) *ConsoleChannel {
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

func (c *ConsoleChannel) Name() model.Channel {
	return model.ChannelCLI
}

func (c *ConsoleChannel) Start(ctx context.Context) error {
	lines := make(chan string)
	errCh := make(chan error, 1)

	// Read input from console in a separate goroutine
	go func() {
		defer close(lines)

		scanner := bufio.NewScanner(c.input)
		for scanner.Scan() {
			// text does not include the newline character
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		// When EOF is reached, scanner.Err() will return nil
		errCh <- scanner.Err()

	}()

	if err := c.writePrompt(); err != nil {
		return fmt.Errorf("console channel write prompt: %w", err)
	}
	// Main loop to process input lines and handle context cancellation
	for {
		// fmt.Printf("%s ", c.cfg.Prompt)
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
				ID:        fmt.Sprintf("console-%d", time.Now().UnixNano()),
				Channel:   model.ChannelCLI,
				SenderID:  c.cfg.SenderID,
				ChatID:    c.cfg.ChatID,
				Content:   line,
				CreatedAt: time.Now(),
			}
			if err := c.bus.PublishInbound(ctx, msg); err != nil {
				if errors.Is(err, ports.ErrBusClosed) || errors.Is(err, context.Canceled) {
					return nil
				}
				return fmt.Errorf("publish inbound: %w", err)
			}
			// fmt.Printf("%s ", c.cfg.Prompt)
		}
	}
}

func (c *ConsoleChannel) Send(ctx context.Context, out model.OutboundMessage) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("console channel send: %w", ctx.Err())
	default:
		if strings.TrimSpace(out.Content) == "" {
			return nil
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

// writeReplyAndPrompt 先输出模型回复，再输出下一次 prompt。
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

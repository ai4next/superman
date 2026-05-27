package im

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	cccore "github.com/chenhg5/cc-connect/core"
)

type Client struct {
	handler Handler
	logger  *slog.Logger

	mu        sync.RWMutex
	platforms map[string]Platform
	started   bool
}

type ClientOption func(*Client)

func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

func NewClient(handler Handler, opts ...ClientOption) (*Client, error) {
	if handler == nil {
		return nil, errors.New("im: handler is required")
	}
	c := &Client{
		handler:   handler,
		logger:    slog.Default(),
		platforms: make(map[string]Platform),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.logger == nil {
		c.logger = slog.Default()
	}
	return c, nil
}

func NewClientFromConfig(cfg Config, handler Handler, opts ...ClientOption) (*Client, error) {
	c, err := NewClient(handler, opts...)
	if err != nil {
		return nil, err
	}
	for _, pc := range cfg.Platforms {
		if !pc.Enabled {
			continue
		}
		if pc.Name == "" {
			return nil, errors.New("im: platform name is required")
		}
		if _, err := c.Add(pc.Name, pc.Options); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Client) Add(name string, opts map[string]any) (Platform, error) {
	p, err := NewPlatform(name, opts)
	if err != nil {
		return nil, err
	}
	if err := c.AddPlatform(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (c *Client) AddPlatform(p Platform) error {
	if p == nil {
		return errors.New("im: platform is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return errors.New("im: cannot add platform after start")
	}
	name := p.Name()
	if name == "" {
		return errors.New("im: platform name is empty")
	}
	if _, ok := c.platforms[name]; ok {
		return fmt.Errorf("im: platform %q already added", name)
	}
	c.platforms[name] = p
	return nil
}

func (c *Client) Platform(name string) (Platform, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.platforms[name]
	return p, ok
}

func (c *Client) Platforms() map[string]Platform {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]Platform, len(c.platforms))
	for name, p := range c.platforms {
		out[name] = p
	}
	return out
}

func (c *Client) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return errors.New("im: client already started")
	}
	platforms := c.platformsLocked()
	c.started = true
	c.mu.Unlock()

	started := make([]Platform, 0, len(platforms))
	for _, p := range platforms {
		if lifecycle, ok := p.(cccore.AsyncRecoverablePlatform); ok {
			lifecycle.SetLifecycleHandler(clientLifecycleHandler{client: c})
		}
		if err := p.Start(c.handleMessage); err != nil {
			for i := len(started) - 1; i >= 0; i-- {
				if stopErr := started[i].Stop(); stopErr != nil {
					c.logger.Debug("im: rollback stop failed", "platform", started[i].Name(), "error", stopErr)
				}
			}
			c.mu.Lock()
			c.started = false
			c.mu.Unlock()
			return fmt.Errorf("im: start platform %q: %w", p.Name(), err)
		}
		started = append(started, p)
	}

	go func() {
		<-ctx.Done()
		if err := c.Stop(); err != nil {
			c.logger.Debug("im: stop after context cancellation failed", "error", err)
		}
	}()
	return nil
}

func (c *Client) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.Start(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	return c.Stop()
}

func (c *Client) Stop() error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	platforms := c.platformsLocked()
	c.started = false
	c.mu.Unlock()

	var errs []error
	for i := len(platforms) - 1; i >= 0; i-- {
		if err := platforms[i].Stop(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", platforms[i].Name(), err))
		}
	}
	return errors.Join(errs...)
}

func (c *Client) Reply(ctx context.Context, msg *Message, content string) error {
	if msg == nil {
		return errors.New("im: message is nil")
	}
	p, ok := c.Platform(msg.Platform)
	if !ok {
		return fmt.Errorf("im: platform %q is not attached", msg.Platform)
	}
	return p.Reply(ctx, msg.ReplyCtx, content)
}

func (c *Client) Send(ctx context.Context, msg *Message, content string) error {
	if msg == nil {
		return errors.New("im: message is nil")
	}
	p, ok := c.Platform(msg.Platform)
	if !ok {
		return fmt.Errorf("im: platform %q is not attached", msg.Platform)
	}
	return p.Send(ctx, msg.ReplyCtx, content)
}

func (c *Client) SendImage(ctx context.Context, msg *Message, img ImageAttachment) error {
	if msg == nil {
		return errors.New("im: message is nil")
	}
	p, ok := c.Platform(msg.Platform)
	if !ok {
		return fmt.Errorf("im: platform %q is not attached", msg.Platform)
	}
	sender, ok := p.(cccore.ImageSender)
	if !ok {
		return ErrNotSupported
	}
	return sender.SendImage(ctx, msg.ReplyCtx, img)
}

func (c *Client) SendFile(ctx context.Context, msg *Message, file FileAttachment) error {
	if msg == nil {
		return errors.New("im: message is nil")
	}
	p, ok := c.Platform(msg.Platform)
	if !ok {
		return fmt.Errorf("im: platform %q is not attached", msg.Platform)
	}
	sender, ok := p.(cccore.FileSender)
	if !ok {
		return ErrNotSupported
	}
	return sender.SendFile(ctx, msg.ReplyCtx, file)
}

func (c *Client) SendWithButtons(ctx context.Context, msg *Message, content string, buttons [][]ButtonOption) error {
	if msg == nil {
		return errors.New("im: message is nil")
	}
	p, ok := c.Platform(msg.Platform)
	if !ok {
		return fmt.Errorf("im: platform %q is not attached", msg.Platform)
	}
	sender, ok := p.(cccore.InlineButtonSender)
	if !ok {
		return ErrNotSupported
	}
	return sender.SendWithButtons(ctx, msg.ReplyCtx, content, buttons)
}

func (c *Client) SendCard(ctx context.Context, msg *Message, card *Card) error {
	if msg == nil {
		return errors.New("im: message is nil")
	}
	if card == nil {
		return errors.New("im: card is nil")
	}
	p, ok := c.Platform(msg.Platform)
	if !ok {
		return fmt.Errorf("im: platform %q is not attached", msg.Platform)
	}
	if sender, ok := p.(cccore.CardSender); ok {
		return sender.SendCard(ctx, msg.ReplyCtx, card)
	}
	return p.Send(ctx, msg.ReplyCtx, card.RenderText())
}

func (c *Client) platformsLocked() []Platform {
	names := make([]string, 0, len(c.platforms))
	for name := range c.platforms {
		names = append(names, name)
	}
	sort.Strings(names)
	platforms := make([]Platform, 0, len(names))
	for _, name := range names {
		platforms = append(platforms, c.platforms[name])
	}
	return platforms
}

func (c *Client) handleMessage(p cccore.Platform, msg *cccore.Message) {
	c.handler(context.Background(), c, p, msg)
}

type clientLifecycleHandler struct {
	client *Client
}

func (h clientLifecycleHandler) OnPlatformReady(p cccore.Platform) {
	if h.client != nil && h.client.logger != nil {
		h.client.logger.Info("im: platform ready", "platform", p.Name())
	}
}

func (h clientLifecycleHandler) OnPlatformUnavailable(p cccore.Platform, err error) {
	if h.client != nil && h.client.logger != nil {
		h.client.logger.Warn("im: platform unavailable", "platform", p.Name(), "error", err)
	}
}

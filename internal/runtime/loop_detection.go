package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/ai4next/superman/internal/bus"
)

const (
	DefaultLoopWindowSize = 10
	DefaultLoopMaxRepeats = 5
)

var ErrLoopDetected = errors.New("agent loop detected")

type LoopDetectionConfig struct {
	Enabled    bool
	WindowSize int
	MaxRepeats int
}

type LoopDetector struct {
	windowSize int
	maxRepeats int
	started    map[string]toolInteraction
	signatures []string
}

type toolInteraction struct {
	name string
	args string
}

func NewLoopDetector(cfg LoopDetectionConfig) *LoopDetector {
	if !cfg.Enabled {
		return nil
	}
	windowSize := cfg.WindowSize
	if windowSize <= 0 {
		windowSize = DefaultLoopWindowSize
	}
	maxRepeats := cfg.MaxRepeats
	if maxRepeats <= 0 {
		maxRepeats = DefaultLoopMaxRepeats
	}
	return &LoopDetector{
		windowSize: windowSize,
		maxRepeats: maxRepeats,
		started:    make(map[string]toolInteraction),
	}
}

func (d *LoopDetector) Observe(event bus.Event) error {
	if d == nil {
		return nil
	}
	switch event.Type {
	case bus.EventToolCallStarted:
		if event.ToolID == "" {
			return nil
		}
		d.started[event.ToolID] = toolInteraction{name: event.ToolName, args: event.Args}
	case bus.EventToolCallFinished:
		name := event.ToolName
		args := event.Args
		if started, ok := d.started[event.ToolID]; ok {
			if name == "" {
				name = started.name
			}
			args = started.args
			delete(d.started, event.ToolID)
		}
		if name == "" {
			return nil
		}
		signature := toolSignature(name, args, event.Result, event.Status)
		d.signatures = append(d.signatures, signature)
		if len(d.signatures) > d.windowSize {
			d.signatures = d.signatures[len(d.signatures)-d.windowSize:]
		}
		if d.repeated(signature) {
			return fmt.Errorf("%w: repeated %s with same input/result more than %d times in the last %d tool results", ErrLoopDetected, name, d.maxRepeats, d.windowSize)
		}
	}
	return nil
}

func (d *LoopDetector) repeated(signature string) bool {
	if d == nil || signature == "" || len(d.signatures) < d.windowSize {
		return false
	}
	count := 0
	for _, existing := range d.signatures {
		if existing == signature {
			count++
			if count > d.maxRepeats {
				return true
			}
		}
	}
	return false
}

func toolSignature(name, args, result, status string) string {
	h := sha256.New()
	io.WriteString(h, name)
	io.WriteString(h, "\x00")
	io.WriteString(h, args)
	io.WriteString(h, "\x00")
	io.WriteString(h, result)
	io.WriteString(h, "\x00")
	io.WriteString(h, status)
	return hex.EncodeToString(h.Sum(nil))
}

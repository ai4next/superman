// Package im connects Superman to instant-messaging platforms.
package im

import (
	"context"

	"github.com/chenhg5/cc-connect/core"
)

type Message = core.Message
type ImageAttachment = core.ImageAttachment
type FileAttachment = core.FileAttachment
type AudioAttachment = core.AudioAttachment
type LocationAttachment = core.LocationAttachment
type ButtonOption = core.ButtonOption
type Card = core.Card
type Platform = core.Platform

type Handler func(ctx context.Context, client *Client, platform Platform, msg *Message)

type PlatformConfig struct {
	Name    string         `mapstructure:"name" yaml:"name" json:"name"`
	Enabled bool           `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Options map[string]any `mapstructure:"options" yaml:"options" json:"options"`
}

type Config struct {
	Platforms []PlatformConfig `mapstructure:"platforms" yaml:"platforms" json:"platforms"`
}

type Capabilities struct {
	ReplyContextReconstructor bool
	FormattingInstructions    bool
	TypingIndicator           bool
	DoneReaction              bool
	Images                    bool
	Files                     bool
	MessageUpdate             bool
	ProgressStyle             bool
	ProgressCardPayload       bool
	InlineButtons             bool
	Cards                     bool
	CardNavigation            bool
	CardRefresh               bool
	Lifecycle                 bool
}

func PlatformCapabilities(p Platform) Capabilities {
	if p == nil {
		return Capabilities{}
	}
	_, replyReconstruct := p.(core.ReplyContextReconstructor)
	_, formatting := p.(core.FormattingInstructionProvider)
	_, typing := p.(core.TypingIndicator)
	_, done := p.(core.TypingIndicatorDone)
	_, images := p.(core.ImageSender)
	_, files := p.(core.FileSender)
	_, update := p.(core.MessageUpdater)
	_, progressStyle := p.(core.ProgressStyleProvider)
	_, progressPayload := p.(core.ProgressCardPayloadSupport)
	_, buttons := p.(core.InlineButtonSender)
	_, cards := p.(core.CardSender)
	_, cardNav := p.(core.CardNavigable)
	_, cardRefresh := p.(core.CardRefresher)
	_, lifecycle := p.(core.AsyncRecoverablePlatform)

	return Capabilities{
		ReplyContextReconstructor: replyReconstruct,
		FormattingInstructions:    formatting,
		TypingIndicator:           typing,
		DoneReaction:              done,
		Images:                    images,
		Files:                     files,
		MessageUpdate:             update,
		ProgressStyle:             progressStyle,
		ProgressCardPayload:       progressPayload,
		InlineButtons:             buttons,
		Cards:                     cards,
		CardNavigation:            cardNav,
		CardRefresh:               cardRefresh,
		Lifecycle:                 lifecycle,
	}
}

package im

import cccore "github.com/chenhg5/cc-connect/core"

type PlatformFactory = cccore.PlatformFactory

var ErrNotSupported = cccore.ErrNotSupported

func RegisterPlatform(name string, factory PlatformFactory) {
	cccore.RegisterPlatform(name, factory)
}

func NewPlatform(name string, opts map[string]any) (Platform, error) {
	if opts == nil {
		opts = map[string]any{}
	}
	return cccore.CreatePlatform(name, opts)
}

func RegisteredPlatforms() []string {
	return cccore.ListRegisteredPlatforms()
}

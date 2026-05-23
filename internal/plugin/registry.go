package plugin

import adkplugin "google.golang.org/adk/plugin"

type Registry struct {
	plugins map[string]*adkplugin.Plugin
}

func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]*adkplugin.Plugin),
	}
}

func (r *Registry) Register(name string, p *adkplugin.Plugin) {
	r.plugins[name] = p
}

func (r *Registry) Get(name string) *adkplugin.Plugin {
	return r.plugins[name]
}

func (r *Registry) All() []*adkplugin.Plugin {
	var result []*adkplugin.Plugin
	for _, p := range r.plugins {
		result = append(result, p)
	}
	return result
}

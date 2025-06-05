package plugins

import (
	"fmt"

	"github.com/sofatutor/llm-proxy/internal/dispatcher"
)

// PluginFactory is a function that creates a new plugin instance
type PluginFactory func() dispatcher.BackendPlugin

// Registry holds all available plugin factories
var Registry = make(map[string]PluginFactory)

// init registers all built-in plugins
func init() {
	Registry["file"] = func() dispatcher.BackendPlugin {
		return NewFilePlugin()
	}

	Registry["lunary"] = func() dispatcher.BackendPlugin {
		return NewLunaryPlugin()
	}

	Registry["helicone"] = func() dispatcher.BackendPlugin {
		return NewHeliconePlugin()
	}
}

// NewPlugin creates a new plugin instance by name
func NewPlugin(name string) (dispatcher.BackendPlugin, error) {
	factory, exists := Registry[name]
	if !exists {
		return nil, fmt.Errorf("unknown plugin: %s", name)
	}

	return factory(), nil
}

// ListPlugins returns a list of available plugin names
func ListPlugins() []string {
	var names []string
	for name := range Registry {
		names = append(names, name)
	}
	return names
}

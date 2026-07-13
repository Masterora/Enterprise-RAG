package agent

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) error {
	if tool == nil {
		return errors.New("agent tool is nil")
	}
	spec := tool.Spec()
	name := normalizeToolName(spec.Name)
	if name == "" || strings.TrimSpace(spec.Description) == "" {
		return errors.New("agent tool requires name and description")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; exists {
		return errors.New("agent tool already registered: " + name)
	}
	r.tools[name] = tool
	return nil
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[normalizeToolName(name)]
	return tool, ok
}

func (r *Registry) Specs(allowed map[string]struct{}) []ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	specs := make([]ToolSpec, 0, len(r.tools))
	for name, tool := range r.tools {
		if len(allowed) > 0 {
			if _, ok := allowed[name]; !ok {
				continue
			}
		}
		specs = append(specs, tool.Spec())
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs
}

func normalizeToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/chriscow/minds"
)

// FunctionTool represents a callable function that can be used by agents
type FunctionTool interface {
	Name() string
	Description() string
	Call(ctx context.Context, args []byte) ([]byte, error)
	Schema() *minds.Definition
}

// ToolRegistry manages a collection of function tools
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]FunctionTool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]FunctionTool),
	}
}

// Register adds a function tool to the registry
func (r *ToolRegistry) Register(tool FunctionTool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}
	
	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool with name '%s' already registered", name)
	}
	
	r.tools[name] = tool
	return nil
}

// Lookup finds a tool by name
func (r *ToolRegistry) Lookup(name string) (FunctionTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tool, exists := r.tools[name]
	return tool, exists
}

// List returns all registered tools
func (r *ToolRegistry) List() []FunctionTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tools := make([]FunctionTool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Remove removes a tool from the registry
func (r *ToolRegistry) Remove(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.tools[name]; exists {
		delete(r.tools, name)
		return true
	}
	return false
}

// Clear removes all tools from the registry
func (r *ToolRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.tools = make(map[string]FunctionTool)
}

// Count returns the number of registered tools
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	return len(r.tools)
}

// Names returns all tool names
func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
package mcpserver

import (
	"fmt"
	"sync"
)

type Tool struct {
	originalName string
	serverName   string
	definition   map[string]interface{}
}

func (t *Tool) Key() string {
	return fmt.Sprintf("%s:%s", t.serverName, t.originalName)
}

func (t *Tool) Name() string {
	return fmt.Sprintf("ms_%s_%s", t.serverName, t.originalName)
}

func (t *Tool) GetServerName() string {
	return t.serverName
}

func (t *Tool) GetOriginalName() string {
	return t.originalName
}

type ToolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

func (tr *ToolRegistry) UpdateTool(tool Tool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	
	key := tool.Key()
	tr.tools[key] = tool
}

func (tr *ToolRegistry) ToList() []interface{} {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	var tools []interface{}
	for _, tool := range tr.tools {
		// Create a copy of the definition with the prefixed name
		toolDef := make(map[string]interface{})
		for k, v := range tool.definition {
			toolDef[k] = v
		}
		toolDef["name"] = tool.Name()
		tools = append(tools, toolDef)
	}
	return tools
}

func (tr *ToolRegistry) Print() {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	for _, tool := range tr.tools {
		fmt.Printf("  🔧 %s -> %s (from %s)\n", tool.GetOriginalName(), tool.Name(), tool.GetServerName())
	}
}
func (tr *ToolRegistry) FindByName(name string) (*Tool, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	for _, tool := range tr.tools {
		if tool.Name() == name {
			return &tool, true
		}
	}
	return nil, false
}

 
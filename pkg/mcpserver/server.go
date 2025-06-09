package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nsxbet/mcpshield/pkg"
)



type MCPServer struct {
	Name         string            `yaml:"name"`
	Image        string            `yaml:"image"`
	Command      string            `yaml:"command"`
	Args         []string          `yaml:"args"`
	Env          map[string]string `yaml:"env,omitempty"`
	runtime      pkg.Runtime       `yaml:"-"`
	ctx          context.Context   `yaml:"-"`
	cancel       context.CancelFunc `yaml:"-"`
	toolRegistry *ToolRegistry     `yaml:"-"`
	initRegistry *InitializationRegistry `yaml:"-"`
}

func NewMCPServer(name, image, command string, args []string, env map[string]string, RuntimeFactory pkg.RuntimeFactory) *MCPServer {
	runtime := RuntimeFactory.CreateRuntime(image, command, args, env)
	
	return &MCPServer{
		Name:         name,
		Image:        image,
		Command:      command,
		Args:         args,
		Env:          env,
		runtime:      runtime,
		toolRegistry: NewToolRegistry(),
		initRegistry: NewInitializationRegistry(),
	}
}

func (m *MCPServer) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)
	
	if err := m.runtime.Start(m.ctx); err != nil {
		return err
	}
	
	go func() {
		<-m.ctx.Done()
		m.runtime.Stop(context.Background())
	}()
	
	return nil
}

func (m *MCPServer) Stop(ctx context.Context) {
	if m.cancel != nil {
		m.cancel()
	}
	// Directly call runtime.Stop() to ensure cleanup completes
	if m.runtime != nil {
		m.runtime.Stop(ctx)
	}
}

// Call executes an MCP call and returns the response
func (m *MCPServer) Call(request *pkg.MCPRequest) (*pkg.MCPResponse, error) {
	if m.ctx == nil {
		return nil, fmt.Errorf("server not started")
	}
	
	if m.ctx.Err() != nil {
		return nil, fmt.Errorf("server context cancelled")
	}
	
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	responseBytes, err := m.runtime.Exec(m.ctx, requestBytes)
	if err != nil {
		return nil, fmt.Errorf("runtime exec failed: %w", err)
	}

	var response pkg.MCPResponse
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	return &response, nil
}

func (m *MCPServer) IsReady() bool {
	if m.ctx == nil || m.ctx.Err() != nil {
		return false
	}
	return m.runtime.IsReady()
}

func (m *MCPServer) UpdateToolRegistry() error {
	if !m.IsReady() {
		return fmt.Errorf("server %s is not ready", m.Name)
	}
	
	response, err := m.Call(&pkg.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	})
	if err != nil {
		return fmt.Errorf("failed to get tools from server %s: %w", m.Name, err)
	}
	
	if response.Result == nil {
		return fmt.Errorf("no response result from server %s", m.Name)
	}
	
	result, ok := response.Result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format from server %s", m.Name)
	}
	
	toolsInterface, ok := result["tools"].([]interface{})
	if !ok {
		return nil // No tools is ok
	}
	
	for _, tool := range toolsInterface {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		
		toolName, ok := toolMap["name"].(string)
		if !ok {
			continue
		}
		
		tool := Tool{
			originalName: toolName,
			serverName:   m.Name,
			definition:   toolMap,
		}
		m.toolRegistry.UpdateTool(tool)
	}
	return nil
}

func (m *MCPServer) UpdateInitializationRegistry() error {
	if !m.IsReady() {
		return fmt.Errorf("server %s is not ready", m.Name)
	}
	
	response, err := m.Call(&pkg.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "mcpshield-proxy",
				"version": "1.0.0",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get initialization from server %s: %w", m.Name, err)
	}
	
	m.initRegistry.UpdateInitialization(m.Name, response)
	return nil
}
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nsxbet/mcpshield/pkg"
)

type MCPServer struct {
	Name    string            `yaml:"name"`
	Image   string            `yaml:"image"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env,omitempty"`
	runtime pkg.Runtime       `yaml:"-"`
	ctx     context.Context   `yaml:"-"`
	cancel  context.CancelFunc `yaml:"-"`
}

func NewMCPServer(name, image, command string, args []string, env map[string]string, RuntimeFactory pkg.RuntimeFactory) *MCPServer {
	runtime := RuntimeFactory.CreateRuntime(image, command, args, env)
	
	return &MCPServer{
		Name:    name,
		Image:   image,
		Command: command,
		Args:    args,
		Env:     env,
		runtime: runtime,
	}
}

func (m *MCPServer) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)
	
	if err := m.runtime.Start(m.ctx); err != nil {
		return err
	}
	
	go func() {
		<-m.ctx.Done()
		m.runtime.Stop()
	}()
	
	return nil
}

func (m *MCPServer) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

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

func (m *MCPServer) CallTool(toolName string, arguments interface{}) (*pkg.MCPResponse, error) {
	request := &pkg.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}
	
	return m.Call(request)
}

func (m *MCPServer) ListTools() (*pkg.MCPResponse, error) {
	request := &pkg.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	
	return m.Call(request)
}

func (m *MCPServer) GetName() string {
	return m.Name
}

func (m *MCPServer) IsReady() bool {
	if m.ctx == nil || m.ctx.Err() != nil {
		return false
	}
	return m.runtime.IsReady()
} 
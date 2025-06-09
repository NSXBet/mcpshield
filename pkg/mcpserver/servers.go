package mcpserver

import (
	"context"
	"fmt"

	"github.com/nsxbet/mcpshield/pkg"
)

type MCPServers map[string]*MCPServer

func NewServers(config *pkg.Config, factory pkg.RuntimeFactory) MCPServers {
	servers := make(MCPServers)
	for _, serverConfig := range config.GetMCPServers() {
		server := NewMCPServer(
			serverConfig.Name,
			serverConfig.Image,
			serverConfig.Command,
			serverConfig.Args,
			serverConfig.Env,
			factory,
		)
		servers[serverConfig.Name] = server
	}
	return servers
}

func (s MCPServers) StartAll(ctx context.Context) error {
	for name, server := range s {
		if err := server.Start(ctx); err != nil {
			return fmt.Errorf("failed to start server %s: %w", name, err)
		}
		
		if !server.IsReady() {
			return fmt.Errorf("server %s not ready after start", name)
		}
	}
	
	if err := s.UpdateAllToolRegistries(); err != nil {
		return fmt.Errorf("failed to update tool registries: %w", err)
	}

	if err := s.UpdateAllInitializationRegistries(); err != nil {
		return fmt.Errorf("failed to update initialization registries: %w", err)
	}
		
	fmt.Printf("🎉 All MCP servers started successfully\n")
	s.PrintAllTools()
	fmt.Printf("\n✅ Proxy ready to serve requests\n")
	
	return nil
}

func (s MCPServers) StopAll(ctx context.Context) {
	for _, server := range s {
		server.Stop(ctx)
	}
}

func (s MCPServers) AllTools() []interface{} {
	var allTools []interface{}
	for _, server := range s {
		allTools = append(allTools, server.toolRegistry.ToList()...)
	}
	return allTools
}

func (s MCPServers) CallTool(toolName string, request *pkg.MCPRequest) (*pkg.MCPResponse, error) {
	for _, server := range s {
		tool, found := server.toolRegistry.FindByName(toolName)
		if !found {
			continue
		}
		
		if !server.IsReady() {
			return nil, fmt.Errorf("server not ready: %s", server.Name)
		}
		
		params := request.Params.(map[string]interface{})
		params["name"] = tool.GetOriginalName()
		return server.Call(request)
	}
	return nil, fmt.Errorf("tool not found: %s", toolName)
}

func (s MCPServers) UpdateAllToolRegistries() error {
	for name, server := range s {
		if err := server.UpdateToolRegistry(); err != nil {
			return fmt.Errorf("failed to update tool registry for server %s: %w", name, err)
		}
	}
	return nil
}

func (s MCPServers) UpdateAllInitializationRegistries() error {
	for name, server := range s {
		if err := server.UpdateInitializationRegistry(); err != nil {
			return fmt.Errorf("failed to update initialization registry for server %s: %w", name, err)
		}
	}
	return nil
}

func (s MCPServers) GetAllInitializationResponses() map[string]*pkg.MCPResponse {
	allResponses := make(map[string]*pkg.MCPResponse)
	for _, server := range s {
		responses := server.initRegistry.GetResponses()
		for serverName, response := range responses {
			allResponses[serverName] = response
		}
	}
	return allResponses
}

func (s MCPServers) PrintAllTools() {
	for _, server := range s {
		server.toolRegistry.Print()
	}
} 
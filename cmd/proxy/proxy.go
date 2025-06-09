package main

import (
	"context"
	"fmt"
	"log"

	"github.com/nsxbet/mcpshield/pkg"
	"github.com/nsxbet/mcpshield/pkg/mcpserver"
)

type MCPProxy struct {
	servers   map[string]*mcpserver.MCPServer
	mcpConfig *pkg.MCPConfig
}

func NewMCPProxy(config *pkg.MCPConfig, factory pkg.RuntimeFactory) *MCPProxy {
	servers := make(map[string]*mcpserver.MCPServer)
	for _, serverConfig := range config.MCPServers {
		server := mcpserver.NewMCPServer(
			serverConfig.Name,
			serverConfig.Image,
			serverConfig.Command,
			serverConfig.Args,
			serverConfig.Env,
			factory,
		)
		servers[serverConfig.Name] = server
	}
	
	return &MCPProxy{
		servers:   servers,
		mcpConfig: config,
	}
}

func (p *MCPProxy) startAllServers(ctx context.Context) error {
	fmt.Printf("🚀 Starting MCP servers...\n")
	
	for name, server := range p.servers {
		log.Printf("🚀 Starting MCP server: %s", name)
		if err := server.Start(ctx); err != nil {
			return fmt.Errorf("failed to start server %s: %w", name, err)
		}
		
		if !server.IsReady() {
			return fmt.Errorf("server %s not ready after start", name)
		}
		
		log.Printf("✅ MCP server %s is ready", name)
	}
	
	fmt.Printf("🎉 All MCP servers started successfully\n")
	return nil
}

func (p *MCPProxy) stopAllServers() {
	fmt.Printf("🛑 Stopping MCP servers...\n")
	
	for name, server := range p.servers {
		log.Printf("🛑 Stopping MCP server: %s", name)
		server.Stop()
		log.Printf("✅ MCP server %s stopped", name)
	}
	
	fmt.Printf("🎉 All MCP servers stopped\n")
}

func (p *MCPProxy) GetServerCount() int {
	return len(p.servers)
}

func (p *MCPProxy) Start(ctx context.Context) error {
	return p.startAllServers(ctx)
}

func (p *MCPProxy) Stop() {
	p.stopAllServers()
}

func (p *MCPProxy) ProcessMCPRequest(request *pkg.MCPRequest) (*pkg.MCPResponse, error) {
	log.Printf("📨 %s request", request.Method)

	// Determine server name
	var serverName string
	if request.Method == "tools/call" {
		var err error
		serverName, err = request.GetServerName()
		if err != nil {
			return nil, err
		}
	} else {
		if len(p.mcpConfig.MCPServers) == 0 {
			return nil, fmt.Errorf("no servers configured")
		}
		serverName = p.mcpConfig.MCPServers[0].Name
	}

	// Get server
	if serverName == "" {
		return nil, fmt.Errorf("cannot determine target server from request")
	}
	
	server, exists := p.servers[serverName]
	if !exists {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}
	
	if !server.IsReady() {
		return nil, fmt.Errorf("server not ready: %s", serverName)
	}

	// Process request based on method
	if request.Method == "tools/list" {
		return server.ListTools(request)
	}
	
	if request.Method == "tools/call" {
		originalToolName := request.GetOriginalToolName()
		if originalToolName == "" {
			return nil, fmt.Errorf("cannot extract original tool name")
		}
		
		params := request.Params.(map[string]interface{})
		params["name"] = originalToolName
		
		return server.Call(request)
	}
	
	return server.Call(request)
}
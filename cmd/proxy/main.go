package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nsxbet/mcpshield/pkg"
	"github.com/nsxbet/mcpshield/pkg/mcpserver"
	"github.com/nsxbet/mcpshield/pkg/runtime"
	"gopkg.in/yaml.v2"
)

type MCPProxy struct {
	servers   map[string]*mcpserver.MCPServer
	mcpConfig *MCPConfig
}

type MCPConfig struct {
	MCPServers []MCPServerConfig `yaml:"mcp-servers"`
	Runtime    RuntimeConfig     `yaml:"runtime"`
}

type MCPServerConfig struct {
	Name    string            `yaml:"name"`
	Image   string            `yaml:"image"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env,omitempty"`
}

type KubernetesConfig struct {
	Namespace string `yaml:"namespace"`
}

type RuntimeConfig struct {
	Kubernetes *KubernetesConfig `yaml:"kubernetes"`
}

func main() {
	fmt.Println("🚀 Starting MCP Bridge Proxy...")

	// Read configuration
	config, err := readConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}

	// Create runtime factory
	factory, err := runtime.NewKubernetesRuntimeFactory(config.Runtime.Kubernetes.Namespace)
	if err != nil {
		log.Fatalf("Failed to create runtime factory: %v", err)
	}

	// Create MCP servers
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

	proxy := &MCPProxy{
		servers:   servers,
		mcpConfig: config,
	}

	// Start all MCP servers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = proxy.startAllServers(ctx)
	if err != nil {
		log.Fatalf("Failed to start MCP servers: %v", err)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Setup HTTP routes
	http.HandleFunc("/mcp", proxy.handleMCP)
	http.HandleFunc("/health", handleHealth)

	// Start HTTP server in a goroutine
	server := &http.Server{Addr: ":8080"}
	go func() {
		port := "8080"
		fmt.Printf("✅ MCP Bridge Proxy ready\n")
		fmt.Printf("🌐 MCP endpoint: http://localhost:%s/mcp\n", port)
		fmt.Printf("📍 Running %d MCP servers\n", len(servers))
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n🛑 Shutdown signal received, cleaning up...")

	// Stop all MCP servers
	proxy.stopAllServers()

	// Graceful shutdown of HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	fmt.Println("👋 MCP Bridge Proxy stopped")
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

func (p *MCPProxy) handleMCP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request pkg.MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Log request
	log.Printf("📨 %s request", request.Method)

	// Route request to appropriate server
	response, err := p.routeRequest(&request)
	if err != nil {
		response = &pkg.MCPResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Error:   map[string]interface{}{"code": -32603, "message": err.Error()},
		}
	}

	// Log response
	if responseBytes, err := json.Marshal(response); err == nil {
		log.Printf("📤 response: %s", string(responseBytes))
	}

	json.NewEncoder(w).Encode(response)
}

func (p *MCPProxy) routeRequest(request *pkg.MCPRequest) (*pkg.MCPResponse, error) {
	serverName := p.getTargetServer(request)
	
	server, exists := p.servers[serverName]
	if !exists {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	if !server.IsReady() {
		return nil, fmt.Errorf("server not ready: %s", serverName)
	}

	// Convert request and forward to server
	mcpRequest := &pkg.MCPRequest{
		JSONRPC: request.JSONRPC,
		ID:      request.ID,
		Method:  request.Method,
		Params:  request.Params,
	}



	response, err := server.Call(mcpRequest)
	if err != nil {
		return nil, err
	}

	return &pkg.MCPResponse{
		JSONRPC: response.JSONRPC,
		ID:      response.ID,
		Result:  response.Result,
		Error:   response.Error,
	}, nil
}

func (p *MCPProxy) getTargetServer(request *pkg.MCPRequest) string {
	// For tools/call, extract server from tool name
	if request.Method == "tools/call" {
		serverName := request.GetServerName()
		if serverName != "" {
			return serverName
		}
	}
	
	// Default to first server
	if len(p.mcpConfig.MCPServers) > 0 {
		return p.mcpConfig.MCPServers[0].Name
	}
	
	return ""
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func readConfig(filename string) (*MCPConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config MCPConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
} 
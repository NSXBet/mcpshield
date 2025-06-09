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
	"github.com/nsxbet/mcpshield/pkg/runtime"
)

func main() {
	fmt.Println("🚀 Starting MCP Bridge Proxy...")

	// Read configuration
	config, err := pkg.ReadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}

	// Create runtime factory
	factory, err := runtime.NewKubernetesRuntimeFactory(config.Runtime.Kubernetes.Namespace)
	if err != nil {
		log.Fatalf("Failed to create runtime factory: %v", err)
	}

	// Create proxy with servers
	proxy := NewMCPProxy(config, factory)

	// Start all MCP servers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = proxy.Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start MCP servers: %v", err)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Setup HTTP routes
	http.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		handleMCP(w, r, proxy)
	})
	http.HandleFunc("/health", handleHealth)

	// Start HTTP server in a goroutine
	server := &http.Server{Addr: ":8080"}
	go func() {
		port := "8080"
		fmt.Printf("✅ MCP Bridge Proxy ready\n")
		fmt.Printf("🌐 MCP endpoint: http://localhost:%s/mcp\n", port)
		fmt.Printf("📍 Running %d MCP servers\n", proxy.GetServerCount())
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n🛑 Shutdown signal received, cleaning up...")

	// Stop all MCP servers
	proxy.Stop()

	// Graceful shutdown of HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	fmt.Println("👋 MCP Bridge Proxy stopped")
}

func handleMCP(w http.ResponseWriter, r *http.Request, proxy *MCPProxy) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	var response *pkg.MCPResponse
	defer func() {
		json.NewEncoder(w).Encode(response)
		if responseBytes, err := json.Marshal(response); err == nil {
			log.Printf("📤 response: %s", string(responseBytes))
		}
	}()

	if r.Method != http.MethodPost {
		response = &pkg.MCPResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error:   map[string]interface{}{"code": -32603, "message": "Method not allowed"},
		}
		return
	}

	var request pkg.MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		response = &pkg.MCPResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Error:   map[string]interface{}{"code": -32603, "message": err.Error()},
		}
		return
	}

	response, err := proxy.ProcessMCPRequest(&request)
	if err != nil {
		response = &pkg.MCPResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Error:   map[string]interface{}{"code": -32603, "message": err.Error()},
		}
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}
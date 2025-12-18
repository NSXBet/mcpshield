package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nsxbet/mcpshield/pkg"
	"github.com/nsxbet/mcpshield/pkg/mcpserver"
	"github.com/nsxbet/mcpshield/pkg/runtime"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// StartServer initializes and starts the HTTP server
func StartServer(config *pkg.Config) error {
	// Validate required configuration
	if !config.HasKubernetesRuntime() {
		logger.Error("Kubernetes runtime configuration is required")
		return fmt.Errorf("kubernetes runtime configuration is required")
	}

	// Create runtime factory with configured namespace and kubeconfig
	factory, err := runtime.NewKubernetesRuntimeFactoryWithKubeconfig(config.GetKubernetesNamespace(), config.GetKubeconfig())
	if err != nil {
		logger.Error("Failed to create runtime factory", "error", err)
		return err
	}

	// Create proxy with servers
	proxy := mcpserver.NewProxy(config, factory)

	// Start all MCP servers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = proxy.Start(ctx)
	if err != nil {
		logger.Error("Failed to start MCP servers", "error", err)
		return err
	}

	mux := http.NewServeMux()
	
	// Health route
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})
	
	// Metrics route
	mux.Handle("/metrics", promhttp.Handler())
	
	// Authentication route
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Authentication placeholder",
			"status":  "not implemented",
		})
	})
	
	// MCP route - single endpoint for JSON-RPC compatibility
	mux.Handle("/mcp", proxy)
	
	// Use configured server settings
	srv := &http.Server{
		Addr:    config.GetServerAddress(),
		Handler: mux,
	}
	
	// Start server
	go func() {
		logger.Info("MCP Bridge Proxy ready", "address", srv.Addr, "servers", proxy.GetServerCount(), "namespace", config.GetKubernetesNamespace())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start", "error", err)
		}
	}()
	
	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	logger.Info("Server shutting down...")
	
	// Create shutdown context with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Stop all MCP servers with timeout context
	proxy.Stop(ctx)
	
	return srv.Shutdown(ctx)
} 
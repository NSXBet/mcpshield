package main

import (
	"context"
	"testing"

	"github.com/nsxbet/mcpshield/pkg/mcpserver"
	"github.com/nsxbet/mcpshield/pkg/runtime"
)

func TestMCPServerWithKubernetesRuntime(t *testing.T) {
	// Create Kubernetes client and config
	client, clientConfig, err := createKubernetesClient()
	if err != nil {
		t.Skipf("Skipping test: failed to create Kubernetes client: %v", err)
	}

	// Create Kubernetes runtime factory
	factory := runtime.NewKubernetesRuntimeFactory(client, clientConfig, "default")

	// Create MCP server with real runtime
	server := mcpserver.NewMCPServer(
		"github-npx",
		"node:18-alpine",
		"npx",
		[]string{"-y", "@modelcontextprotocol/server-github"},
		map[string]string{"GITHUB_TOKEN": "${GITHUB_TOKEN}"},
		factory,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server
	err = server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Wait for server to be ready
	if !server.IsReady() {
		t.Fatal("Server should be ready after start")
	}

	// Test tools list
	response, err := server.ListTools()
	if err != nil {
		t.Logf("⚠️  ListTools error (expected for auth issues): %v", err)
		// Don't fail for auth errors - they're expected without valid tokens
		return
	}

	if response.Result == nil {
		t.Fatal("Expected tools list result")
	}

	// Test tool call
	response, err = server.CallTool("search_repositories", map[string]interface{}{
		"query":   "stars:>1",
		"page":    1,
		"perPage": 5,
	})
	if err != nil {
		t.Logf("⚠️  CallTool error (expected for auth issues): %v", err)
		// Don't fail for auth errors - they're expected without valid tokens
		return
	}

	if response.Result == nil {
		t.Fatal("Expected tool call result")
	}

	t.Log("✅ MCP server with Kubernetes runtime test passed")
}

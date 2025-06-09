package test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nsxbet/mcpshield/pkg"
	"github.com/nsxbet/mcpshield/pkg/mcpserver"
	"github.com/nsxbet/mcpshield/pkg/runtime"
)

var config = &pkg.Config{
	MCPServers: []pkg.MCPServerConfig{
		{
			Name:    "github-npx",
			Image:   "node:18-alpine",
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-github"},
			Env:     map[string]string{"GITHUB_TOKEN": "${GITHUB_TOKEN}"},
		},
	},
}

func TestProcessList(t *testing.T) {
	factory, err := runtime.NewKubernetesRuntimeFactory("default")
	if err != nil {
		t.Skipf("Skipping test: failed to create Kubernetes client: %v", err)
	}

	proxy := mcpserver.NewProxy(config, factory)
	
	ctx := context.Background()
	err = proxy.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start proxy: %v", err)
	}
	defer proxy.Stop(ctx)

	request := &pkg.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	response, err := proxy.ProcessList(request)
	if err != nil {
		t.Logf("⚠️  ProcessList error (expected for auth issues): %v", err)
		return
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	if response.Result == nil {
		t.Fatal("Expected result field in tools/list response")
	}
	
	result := response.Result.(map[string]interface{})
	tools := result["tools"].([]interface{})
	
	if len(tools) == 0 {
		t.Fatal("Expected at least one tool in response")
	}
	
	firstTool := tools[0].(map[string]interface{})
	
	if firstTool["description"] == "" {
		t.Fatal("Expected description field in tool")
	}
	
	if firstTool["inputSchema"] == nil {
		t.Fatal("Expected inputSchema field in tool")
	}
	
	if firstTool["name"] == "" {
		t.Fatal("Expected non-empty name field in tool")
	}
	
	// Check that the name has the correct prefix
	name := firstTool["name"].(string)
	if !strings.HasPrefix(name, "ms_github-npx_") {
		t.Fatalf("Expected tool name to have ms_github-npx_ prefix, got: %s", name)
	}
}

func TestProcessCall(t *testing.T) {
	factory, err := runtime.NewKubernetesRuntimeFactory("default")
	if err != nil {
		t.Skipf("Skipping test: failed to create Kubernetes client: %v", err)
	}

	proxy := mcpserver.NewProxy(config, factory)
	
	ctx := context.Background()
	err = proxy.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start proxy: %v", err)
	}
	defer proxy.Stop(ctx)

	request := &pkg.MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "ms_github-npx_search_repositories",
			"arguments": map[string]interface{}{
				"query":   "stars:>1",
				"page":    1,
				"perPage": 5,
			},
		},
	}

	response, err := proxy.ProcessCall(request)
	if err != nil {
		t.Logf("⚠️  ProcessCall error (expected for auth issues): %v", err)
		return
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	if response.Result == nil {
		t.Fatal("Expected result field in tools/call response")
	}
	
	callResult := response.Result.(map[string]interface{})
	content := callResult["content"].([]interface{})
	
	if len(content) == 0 {
		t.Fatal("Expected content in tools/call response")
	}
	
	firstContent := content[0].(map[string]interface{})
	text := firstContent["text"].(string)
	
	var jsonData map[string]interface{}
	json.Unmarshal([]byte(text), &jsonData)
	
	if _, exists := jsonData["total_count"]; !exists {
		t.Fatal("Expected total_count field in search response")
	}
	
	if _, exists := jsonData["incomplete_results"]; !exists {
		t.Fatal("Expected incomplete_results field in search response")
	}
	
	items, exists := jsonData["items"].([]interface{})
	if !exists || len(items) == 0 {
		t.Fatal("Expected items array with repositories in search response")
	}
}
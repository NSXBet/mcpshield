package mcpserver

import (
	"context"
	"testing"

	"github.com/nsxbet/mcpshield/pkg"
	"github.com/nsxbet/mcpshield/pkg/mocks"
	"go.uber.org/mock/gomock"
)



func TestMCPServerLifecycle(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockRuntime := mocks.NewMockRuntime(ctrl)
	mockRuntime.EXPECT().Start(gomock.Any()).Return(nil)
	mockRuntime.EXPECT().IsReady().Return(true).AnyTimes()
	mockRuntime.EXPECT().Exec(gomock.Any(), gomock.Any()).Return([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`), nil).AnyTimes()
	mockRuntime.EXPECT().Stop().Return(nil).AnyTimes()

	mockFactory := mocks.NewMockRuntimeFactory(ctrl)
	mockFactory.EXPECT().CreateRuntime(
		"node:18-alpine",
		"npx", 
		[]string{"-y", "@modelcontextprotocol/server-github"},
		map[string]string{"TEST_VAR": "test_value"},
	).Return(mockRuntime)

	server := NewMCPServer(
		"test-server",
		"node:18-alpine", 
		"npx",
		[]string{"-y", "@modelcontextprotocol/server-github"},
		map[string]string{"TEST_VAR": "test_value"},
		mockFactory,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	if !server.IsReady() {
		t.Fatal("Server should be ready after start")
	}

	listRequest := &pkg.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	response, err := server.ListTools(listRequest)
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}

	if response.Result == nil {
		t.Fatal("Expected tools list result")
	}

	request := &pkg.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "mock_tool",
			"arguments": map[string]interface{}{"query": "test query"},
		},
	}
	response, err = server.Call(request)
	if err != nil {
		t.Fatalf("Failed to call tool: %v", err)
	}

	if response.Result == nil {
		t.Fatal("Expected tool call result")
	}
}
package auth

import (
	"testing"

	"github.com/nsxbet/mcpshield/pkg"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAuth_CompleteFlow(t *testing.T) {
	// Use fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()
	
	// Create auth
	a := New(fakeClient)

	// Test 1: Authenticate - should return principal info
	principal, err := a.Authenticate("valid-token")
	if err != nil {
		t.Errorf("unexpected authentication error: %v", err)
	}
	if principal == nil {
		t.Errorf("expected principal, got nil")
	}
	if principal != nil && principal.Username == "" {
		t.Errorf("expected username to be set")
	}

	// Test 2: FetchAvailableTools (tools/list)
	toolsListRequest := &pkg.MCPRequest{
		Method: "tools/list",
	}
	tools, err := a.FetchAvailableTools(principal, toolsListRequest)
	if err != nil {
		t.Errorf("unexpected error fetching tools: %v", err)
	}
	if tools == nil {
		t.Errorf("expected tools list, got nil")
	}

	// Test 3: VerifyToolCall (tools/call)
	toolCallRequest := &pkg.MCPRequest{
		Method: "tools/call",
		Params: map[string]interface{}{
			"name": "ms_github-npx_search_repositories",
		},
	}
	err = a.VerifyToolCall(principal, toolCallRequest)
	if err != nil {
		t.Errorf("unexpected error verifying tool call: %v", err)
	}
} 
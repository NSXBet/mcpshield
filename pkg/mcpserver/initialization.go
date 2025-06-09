package mcpserver

import (
	"fmt"

	"github.com/nsxbet/mcpshield/pkg"
)

type InitializationRegistry struct {
	responses map[string]*pkg.MCPResponse
}

func NewInitializationRegistry() *InitializationRegistry {
	return &InitializationRegistry{
		responses: make(map[string]*pkg.MCPResponse),
	}
}

func (r *InitializationRegistry) UpdateInitialization(serverName string, response *pkg.MCPResponse) {
	r.responses[serverName] = response
}

func (r *InitializationRegistry) GetResponses() map[string]*pkg.MCPResponse {
	return r.responses
}

func (r *InitializationRegistry) Print() {
	fmt.Printf("📋 Initialization Registry:\n")
	for serverName, response := range r.responses {
		fmt.Printf("  Server: %s - Response: %+v\n", serverName, response)
	}
} 
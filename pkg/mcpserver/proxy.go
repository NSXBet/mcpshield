package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nsxbet/mcpshield/pkg"
)

type Proxy struct {
	servers MCPServers
	config  *pkg.Config
}

func NewProxy(config *pkg.Config, factory pkg.RuntimeFactory) *Proxy {
	return &Proxy{
		servers: NewServers(config, factory),
		config:  config,
	}
}

// ServeHTTP makes Proxy implement http.Handler
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	if r.Method != http.MethodPost {
		p.writeError(w, 1, -32603, "Method not allowed")
		return
	}

	var request pkg.MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		p.writeError(w, 1, -32603, err.Error())
		return
	}

	var response *pkg.MCPResponse
	var err error
	
	switch request.Method {
	case "initialize":
		response, err = p.ProcessInitialize(&request)
	case "notifications/initialized":
		response = &pkg.MCPResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
		}
	case "tools/list":
		response, err = p.ProcessList(&request)
	case "tools/call":
		response, err = p.ProcessCall(&request)
	default:
		log.Printf("🔍 DEBUG: Received method: '%s' with params: %+v", request.Method, request.Params)
		log.Printf("🚨🚨🚨 CRITICAL ERROR: Method '%s' is not implemented - only tool calls and tools list are supported! 🚨🚨🚨", request.Method)
		p.writeError(w, request.ID, -32603, "🚨🚨🚨 CRITICAL ERROR: Method '"+request.Method+"' is not implemented - only tool calls and tools list are supported! 🚨🚨🚨")
		return
	}

	if err != nil {
		p.writeError(w, request.ID, -32603, err.Error())
		return
	}

	json.NewEncoder(w).Encode(response)
}

func (p *Proxy) writeError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := &pkg.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   map[string]interface{}{"code": code, "message": message},
	}
	json.NewEncoder(w).Encode(response)
}

func (p *Proxy) Start(ctx context.Context) error {
	fmt.Printf("🚀 Starting MCP servers...\n")
	
	maxRetries := 3
	baseDelay := 5 * time.Second
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * baseDelay
			fmt.Printf("⏳ Retrying MCP server startup in %v (attempt %d/%d)...\n", delay, attempt+1, maxRetries)
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		
		err := p.servers.StartAll(ctx)
		if err == nil {
			return nil
		}
		
		fmt.Printf("🔄 MCP server startup failed: %v\n", err)
		
		if attempt < maxRetries-1 {
			p.servers.StopAll(ctx)
		}
	}
	
	return fmt.Errorf("failed to start MCP servers after %d attempts", maxRetries)
}

func (p *Proxy) Stop(ctx context.Context) {
	fmt.Printf("🛑 Stopping MCP servers...\n")
	p.servers.StopAll(ctx)
	fmt.Printf("🎉 All MCP servers stopped\n")
}

func (p *Proxy) GetServerCount() int {
	return len(p.servers)
}

func (p *Proxy) ProcessList(request *pkg.MCPRequest) (*pkg.MCPResponse, error) {
	response := &pkg.MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
	}
	response.Result = map[string]interface{}{
		"tools": p.servers.AllTools(),
	}
	return response, nil
}

func (p *Proxy) ProcessCall(request *pkg.MCPRequest) (*pkg.MCPResponse, error) {
	params := request.Params.(map[string]interface{})
	toolName, ok := params["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing tool name in request")
	}
	
	return p.servers.CallTool(toolName, request)
}

func (p *Proxy) ProcessInitialize(request *pkg.MCPRequest) (*pkg.MCPResponse, error) {
	responses := p.servers.GetAllInitializationResponses()
	
	// Simple aggregation: merge capabilities and concat instructions
	aggregatedCapabilities := map[string]interface{}{
		"tools": map[string]interface{}{
			"listChanged": true,
		},
	}
	var allInstructions []string
	allInstructions = append(allInstructions, "MCP Shield Proxy - Aggregates tools from multiple MCP servers")
	
	for _, response := range responses {
		if response == nil || response.Result == nil {
			continue
		}
		
		result, ok := response.Result.(map[string]interface{})
		if !ok {
			continue
		}
		
		// Merge capabilities
		if caps, ok := result["capabilities"].(map[string]interface{}); ok {
			for capType, capValue := range caps {
				aggregatedCapabilities[capType] = capValue
			}
		}
		
		// Collect instructions
		if instructions, ok := result["instructions"].(string); ok && instructions != "" {
			allInstructions = append(allInstructions, instructions)
		}
	}
	
	response := &pkg.MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
	}
	response.Result = map[string]interface{}{
		"protocolVersion": "2025-03-26",
		"capabilities":    aggregatedCapabilities,
		"serverInfo": map[string]interface{}{
			"name":    "mcpshield-proxy",
			"version": "1.0.0",
		},
		"instructions": strings.Join(allInstructions, "\n"),
	}
	return response, nil
}

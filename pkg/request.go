package pkg

import (
	"strings"
)

// GetToolName extracts the tool name from a tools/call request
func (r *MCPRequest) GetToolName() string {
	if r.Method != "tools/call" {
		return ""
	}
	
	params, ok := r.Params.(map[string]interface{})
	if !ok {
		return ""
	}
	
	toolName, ok := params["name"].(string)
	if !ok {
		return ""
	}
	
	return toolName
}

// GetServerName extracts the server name from a prefixed tool name (ms_servername_toolname)
func (r *MCPRequest) GetServerName() string {
	toolName := r.GetToolName()
	if toolName == "" {
		return ""
	}
	
	if strings.HasPrefix(toolName, "ms_") {
		parts := strings.SplitN(toolName[3:], "_", 2) // Remove "ms_" and split on first "_"
		if len(parts) == 2 {
			return parts[0] // Server name
		}
	}
	return ""
}

// GetOriginalToolName extracts the original tool name by removing the ms_ prefix
func (r *MCPRequest) GetOriginalToolName() string {
	toolName := r.GetToolName()
	if toolName == "" {
		return ""
	}
	
	if strings.HasPrefix(toolName, "ms_") {
		parts := strings.SplitN(toolName[3:], "_", 2) // Remove "ms_" and split on first "_"
		if len(parts) == 2 {
			return parts[1] // Original tool name
		}
	}
	return toolName
} 
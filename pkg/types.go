package pkg

import "context"

//go:generate mockgen -source=types.go -destination=mocks/mock_runtime.go -package=mocks -build_flags=-tags=test

type Runtime interface {
	Start(ctx context.Context) error
	Exec(ctx context.Context, input []byte) ([]byte, error)
	Stop(ctx context.Context) error
	IsReady() bool
}

type RuntimeFactory interface {
	CreateRuntime(image, command string, args []string, env map[string]string) Runtime
}

// MCPRequest represents an MCP protocol request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents an MCP protocol response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}
 
package auth

import (
	"github.com/nsxbet/mcpshield/pkg"
	"k8s.io/client-go/kubernetes"
)

// Auth handles authentication and authorization
type Auth struct {
	client kubernetes.Interface
}

// New creates a new Auth instance
func New(client kubernetes.Interface) *Auth {
	return &Auth{client: client}
}

// Authenticate validates a token and returns principal info
func (a *Auth) Authenticate(token string) (*Principal, error) {
	// Implementation needed - call TokenReview API using a.client
	return nil, &AuthError{Code: "not_implemented", Message: "not implemented"}
}

// FetchAvailableTools returns tools the user can access (for tools/list)
func (a *Auth) FetchAvailableTools(principal *Principal, request *pkg.MCPRequest) (interface{}, error) {
	// Implementation needed - tools source TBD
	return []string{"placeholder"}, nil
}

// VerifyToolCall checks if user can execute a tool (for tools/call)
func (a *Auth) VerifyToolCall(principal *Principal, request *pkg.MCPRequest) error {
	// Implementation needed - parse tool name and call SubjectAccessReview API using a.client
	return nil
} 
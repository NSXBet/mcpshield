package auth

// Principal represents an authenticated user
type Principal struct {
	Username       string
	ServiceAccount string
	Namespace      string
}

// AuthError represents authentication/authorization errors
type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
} 
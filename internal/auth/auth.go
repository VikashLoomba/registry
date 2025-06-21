// Package auth provides authentication mechanisms for the MCP registry
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/modelcontextprotocol/registry/internal/model"
)

var (
	// ErrAuthRequired is returned when authentication is required but not provided
	ErrAuthRequired = errors.New("authentication required")
	// ErrUnsupportedAuthMethod is returned when an unsupported auth method is used
	ErrUnsupportedAuthMethod = errors.New("unsupported authentication method")
)

// EphemeralTokenClaims represents the claims in an ephemeral token
type EphemeralTokenClaims struct {
	GitHubUserID   string    `json:"github_user_id"`
	GitHubUsername string    `json:"github_username"`
	IssuedAt       time.Time `json:"issued_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Nonce          string    `json:"nonce"`
}

// Service defines the authentication service interface
type Service interface {
	// StartAuthFlow initiates an authentication flow and returns the flow information
	StartAuthFlow(ctx context.Context, method model.AuthMethod, repoRef string) (map[string]string, string, error)

	// CheckAuthStatus checks the status of an authentication flow using a status token
	CheckAuthStatus(ctx context.Context, statusToken string) (string, error)

	// ValidateAuth validates the authentication credentials
	ValidateAuth(ctx context.Context, auth model.Authentication) (bool, error)

	// ValidateRegistryOwnerAuth validates that the token belongs to the registry owner
	ValidateRegistryOwnerAuth(ctx context.Context, token string) (bool, error)

	// GenerateEphemeralTokenForGitHubUser validates a GitHub token and generates an ephemeral token
	GenerateEphemeralTokenForGitHubUser(ctx context.Context, githubToken string) (string, error)

	// ValidateEphemeralOrOwnerToken validates either an ephemeral token or registry owner token
	ValidateEphemeralOrOwnerToken(ctx context.Context, token string) (bool, *EphemeralTokenClaims, error)
}

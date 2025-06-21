package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/registry/internal/config"
	"github.com/modelcontextprotocol/registry/internal/model"
)

// ServiceImpl implements the Service interface
type ServiceImpl struct {
	config               *config.Config
	githubAuth           *GitHubDeviceAuth
	ephemeralTokenSecret []byte
}

// EphemeralToken represents a signed ephemeral token
type EphemeralToken struct {
	Claims    EphemeralTokenClaims `json:"claims"`
	Signature string               `json:"signature"`
}

// NewAuthService creates a new authentication service
//
//nolint:ireturn // Factory function intentionally returns interface for dependency injection
func NewAuthService(cfg *config.Config) Service {
	githubConfig := GitHubOAuthConfig{
		ClientID:     cfg.GithubClientID,
		ClientSecret: cfg.GithubClientSecret,
	}

	// Initialize ephemeral token secret
	var ephemeralSecret []byte
	if cfg.EphemeralTokenSecret == "" {
		// Generate a random secret if none provided
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			panic("failed to generate ephemeral token secret")
		}
		ephemeralSecret = secretBytes
	} else {
		ephemeralSecret = []byte(cfg.EphemeralTokenSecret)
	}

	return &ServiceImpl{
		config:               cfg,
		githubAuth:           NewGitHubDeviceAuth(githubConfig),
		ephemeralTokenSecret: ephemeralSecret,
	}
}

func (s *ServiceImpl) StartAuthFlow(_ context.Context, _ model.AuthMethod,
	_ string) (map[string]string, string, error) {
	// return not implemented error
	return nil, "", fmt.Errorf("not implemented")
}

func (s *ServiceImpl) CheckAuthStatus(_ context.Context, _ string) (string, error) {
	// return not implemented error
	return "", fmt.Errorf("not implemented")
}

// ValidateAuth validates authentication credentials
func (s *ServiceImpl) ValidateAuth(ctx context.Context, auth model.Authentication) (bool, error) {
	// If authentication is required but not provided
	if auth.Method == "" || auth.Method == model.AuthMethodNone {
		return false, ErrAuthRequired
	}

	switch auth.Method {
	case model.AuthMethodGitHub:
		// Extract repo reference from the repository URL if it's not provided
		return s.githubAuth.ValidateToken(ctx, auth.Token, auth.RepoRef)
	case model.AuthMethodNone:
		return false, ErrAuthRequired
	default:
		return false, ErrUnsupportedAuthMethod
	}
}

// ValidateRegistryOwnerAuth validates that the provided GitHub token belongs to the registry owner
func (s *ServiceImpl) ValidateRegistryOwnerAuth(ctx context.Context, token string) (bool, error) {
	if s.config.RegistryOwnerGithubUsername == "" {
		return false, fmt.Errorf("registry owner GitHub username not configured")
	}

	// Use the ValidateTokenForOwner method
	valid, err := s.githubAuth.ValidateTokenForOwner(ctx, token, s.config.RegistryOwnerGithubUsername)
	if err != nil {
		return false, fmt.Errorf("failed to validate registry owner token: %w", err)
	}

	return valid, nil
}

// GetGitHubAuth returns the GitHub auth instance (needed for OSS publishing)
func (s *ServiceImpl) GetGitHubAuth() *GitHubDeviceAuth {
	return s.githubAuth
}

// GenerateEphemeralTokenForGitHubUser validates a GitHub token and generates an ephemeral token
func (s *ServiceImpl) GenerateEphemeralTokenForGitHubUser(ctx context.Context, githubToken string) (string, error) {
	// Get user info from GitHub
	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create user request: %w", err)
	}

	userReq.Header.Set("Accept", "application/vnd.github+json")
	userReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", githubToken))

	client := &http.Client{}
	userResp, err := client.Do(userReq)
	if err != nil {
		return "", fmt.Errorf("failed to get user info: %w", err)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to authenticate with GitHub: status %d", userResp.StatusCode)
	}

	var userInfo struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	}

	userBody, err := io.ReadAll(userResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read user info: %w", err)
	}

	if err := json.Unmarshal(userBody, &userInfo); err != nil {
		return "", fmt.Errorf("failed to parse user info: %w", err)
	}

	// Generate ephemeral token valid for 1 hour
	ephemeralToken, err := s.generateEphemeralToken(
		fmt.Sprintf("%d", userInfo.ID),
		userInfo.Login,
		time.Hour,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate ephemeral token: %w", err)
	}

	return ephemeralToken, nil
}

// ValidateEphemeralOrOwnerToken validates either an ephemeral token or registry owner token
func (s *ServiceImpl) ValidateEphemeralOrOwnerToken(ctx context.Context, token string) (bool, *EphemeralTokenClaims, error) {
	// First, try to validate as ephemeral token
	claims, err := s.validateEphemeralToken(token)
	if err == nil {
		// Valid ephemeral token
		return true, claims, nil
	}

	// If ephemeral token validation fails, try registry owner token
	isOwner, ownerErr := s.ValidateRegistryOwnerAuth(ctx, token)
	if ownerErr == nil && isOwner {
		// Valid registry owner token
		return true, nil, nil
	}

	// Neither validation succeeded
	return false, nil, fmt.Errorf("invalid token: not a valid ephemeral token (%v) or registry owner token (%v)", err, ownerErr)
}

// generateEphemeralToken creates a new ephemeral token for a GitHub user
func (s *ServiceImpl) generateEphemeralToken(githubUserID, githubUsername string, duration time.Duration) (string, error) {
	// Generate a random nonce
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	now := time.Now()
	claims := EphemeralTokenClaims{
		GitHubUserID:   githubUserID,
		GitHubUsername: githubUsername,
		IssuedAt:       now,
		ExpiresAt:      now.Add(duration),
		Nonce:          base64.StdEncoding.EncodeToString(nonce),
	}

	// Serialize claims
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %w", err)
	}

	// Create signature
	h := hmac.New(sha256.New, s.ephemeralTokenSecret)
	h.Write(claimsJSON)
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	token := EphemeralToken{
		Claims:    claims,
		Signature: signature,
	}

	// Serialize token
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}

	return base64.StdEncoding.EncodeToString(tokenJSON), nil
}

// validateEphemeralToken validates an ephemeral token and returns the claims if valid
func (s *ServiceImpl) validateEphemeralToken(tokenString string) (*EphemeralTokenClaims, error) {
	// Decode token from base64
	tokenJSON, err := base64.StdEncoding.DecodeString(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token format: %w", err)
	}

	// Parse token
	var token EphemeralToken
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Verify signature
	claimsJSON, err := json.Marshal(token.Claims)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal claims for verification: %w", err)
	}

	h := hmac.New(sha256.New, s.ephemeralTokenSecret)
	h.Write(claimsJSON)
	expectedSignature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(token.Signature), []byte(expectedSignature)) {
		return nil, errors.New("invalid token signature")
	}

	// Check expiration
	if time.Now().After(token.Claims.ExpiresAt) {
		return nil, errors.New("token has expired")
	}

	return &token.Claims, nil
}

// ParseAuthorizationHeader extracts the token from an Authorization header
// Supports both "Bearer <token>" and raw token formats
func ParseAuthorizationHeader(authHeader string) string {
	if authHeader == "" {
		return ""
	}

	// Handle bearer token format
	if len(authHeader) > 7 && strings.ToUpper(authHeader[:7]) == "BEARER " {
		return authHeader[7:]
	}

	return authHeader
}

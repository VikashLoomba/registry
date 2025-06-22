// Package v0 contains API handlers for version 0 of the API
package v0

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/registry/internal/auth"
	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/model"
	"github.com/modelcontextprotocol/registry/internal/service"
)

// PublishOSSHandler handles requests to publish open source MCP servers to the registry
// This endpoint takes a GitHub URL and automatically constructs server details
func PublishOSSHandler(registry service.RegistryService, authService auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get auth token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		// Parse token from header
		token := auth.ParseAuthorizationHeader(authHeader)

		// Validate either ephemeral token or registry owner token
		valid, ephemeralClaims, err := authService.ValidateEphemeralOrOwnerToken(r.Context(), token)
		if err != nil {
			http.Error(w, "Authentication failed: "+err.Error(), http.StatusUnauthorized)
			return
		}

		if !valid {
			http.Error(w, "Invalid authentication token", http.StatusForbidden)
			return
		}

		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Parse request body into PublishOSSRequest struct
		var ossReq model.PublishOSSRequest
		err = json.Unmarshal(body, &ossReq)
		if err != nil {
			http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Validate required fields
		if ossReq.RepositoryURL == "" {
			http.Error(w, "Repository URL is required", http.StatusBadRequest)
			return
		}

		// Extract owner and repo from GitHub URL
		owner, repo, err := extractGitHubRepo(ossReq.RepositoryURL)
		if err != nil {
			http.Error(w, "Invalid GitHub repository URL: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Check if a server with this name already exists in the registry
		expectedServerName := fmt.Sprintf("io.github.%s/%s", owner, repo)
		existingServers, _, err := registry.Search(expectedServerName, "", "", 1)
		if err != nil {
			http.Error(w, "Failed to check existing servers: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// If we found any servers with this exact name, return a conflict error
		for _, server := range existingServers {
			if server.Name == expectedServerName {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "Server already exists",
					"message": fmt.Sprintf("A server with name '%s' has already been published to the registry", expectedServerName),
					"name":    expectedServerName,
				})
				return
			}
		}

		// Fetch repository information from GitHub
		authServiceImpl, ok := authService.(*auth.ServiceImpl)
		if !ok {
			http.Error(w, "Internal authentication service error", http.StatusInternalServerError)
			return
		}

		githubAuth := authServiceImpl.GetGitHubAuth()
		repoInfo, err := githubAuth.FetchRepositoryInfo(r.Context(), token, owner, repo)
		if err != nil {
			http.Error(w, "Failed to fetch repository information: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Generate a unique server ID
		serverID, err := generateServerID()
		if err != nil {
			http.Error(w, "Failed to generate server ID", http.StatusInternalServerError)
			return
		}

		// Construct ServerDetail from GitHub repository information
		serverDetail := model.ServerDetail{
			Server: model.Server{
				ID:          serverID,
				Name:        fmt.Sprintf("io.github.%s/%s", owner, repo),
				Description: repoInfo.Description,
				Repository: model.Repository{
					URL:    repoInfo.HTMLURL,
					Source: "github",
					ID:     strconv.Itoa(repoInfo.ID),
				},
				VersionDetail: model.VersionDetail{
					Version:     "1.0.0-oss", // Default version for OSS publishing
					ReleaseDate: time.Now().Format(time.RFC3339),
					IsLatest:    true,
				},
			},
			// Packages can be left empty for basic OSS publishing
			// They can be added later through the regular publish endpoint
		}

		// Call the publish method on the registry service
		err = registry.Publish(&serverDetail)
		if err != nil {
			// Check for specific error types and return appropriate HTTP status codes
			if database.ErrInvalidVersion != nil && strings.Contains(err.Error(), "invalid version") {
				http.Error(w, "Failed to publish server details: "+err.Error(), http.StatusBadRequest)
				return
			}
			if database.ErrAlreadyExists != nil && strings.Contains(err.Error(), "already exists") {
				http.Error(w, "Server already exists in registry", http.StatusConflict)
				return
			}
			http.Error(w, "Failed to publish server details: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Return a 201 Created response with the server details
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		// Determine who published the server
		var publishedBy string
		if ephemeralClaims != nil {
			publishedBy = ephemeralClaims.GitHubUsername
		} else {
			publishedBy = "registry-owner"
		}

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"message":      "OSS server publication successful",
			"id":           serverDetail.ID,
			"name":         serverDetail.Name,
			"repository":   serverDetail.Repository,
			"published_by": publishedBy,
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

// extractGitHubRepo extracts the owner and repository name from a GitHub repository URL
func extractGitHubRepo(repoURL string) (owner, repo string, err error) {
	// Support various GitHub URL formats:
	// https://github.com/owner/repo
	// https://github.com/owner/repo.git
	// git@github.com:owner/repo.git

	// Remove common prefixes and suffixes
	url := strings.TrimSpace(repoURL)
	url = strings.TrimSuffix(url, ".git")

	// Handle https URLs
	if strings.HasPrefix(url, "https://github.com/") {
		parts := strings.Split(strings.TrimPrefix(url, "https://github.com/"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	// Handle SSH URLs
	if strings.HasPrefix(url, "git@github.com:") {
		parts := strings.Split(strings.TrimPrefix(url, "git@github.com:"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("invalid GitHub repository URL format")
}

// generateServerID generates a unique server ID
func generateServerID() (string, error) {
	// Generate a random UUID-like string
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

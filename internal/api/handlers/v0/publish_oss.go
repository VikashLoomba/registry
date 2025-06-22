// Package v0 contains API handlers for version 0 of the API
package v0

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
			log.Printf("publish-oss: Method not allowed: %s", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get auth token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Printf("publish-oss: Missing Authorization header from %s", r.RemoteAddr)
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		// Parse token from header
		token := auth.ParseAuthorizationHeader(authHeader)

		// Validate either ephemeral token or registry owner token
		valid, ephemeralClaims, err := authService.ValidateEphemeralOrOwnerToken(r.Context(), token)
		if err != nil {
			log.Printf("publish-oss: Authentication failed from %s: %v", r.RemoteAddr, err)
			http.Error(w, "Authentication failed: "+err.Error(), http.StatusUnauthorized)
			return
		}

		if !valid {
			log.Printf("publish-oss: Invalid authentication token from %s", r.RemoteAddr)
			http.Error(w, "Invalid authentication token", http.StatusForbidden)
			return
		}

		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("publish-oss: Error reading request body from %s: %v", r.RemoteAddr, err)
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Parse request body into PublishOSSRequest struct
		var ossReq model.PublishOSSRequest
		err = json.Unmarshal(body, &ossReq)
		if err != nil {
			log.Printf("publish-oss: Invalid request payload from %s: %v", r.RemoteAddr, err)
			http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Validate required fields
		if ossReq.RepositoryURL == "" {
			log.Printf("publish-oss: Missing repository URL from %s", r.RemoteAddr)
			http.Error(w, "Repository URL is required", http.StatusBadRequest)
			return
		}

		// Validate that at least one package is provided
		if len(ossReq.Packages) == 0 {
			log.Printf("publish-oss: No packages provided from %s for repo %s", r.RemoteAddr, ossReq.RepositoryURL)
			http.Error(w, "At least one package is required", http.StatusBadRequest)
			return
		}

		// Validate package fields
		for i, pkg := range ossReq.Packages {
			if pkg.RegistryName == "" {
				log.Printf("publish-oss: Package %d missing registry_name from %s for repo %s", i, r.RemoteAddr, ossReq.RepositoryURL)
				http.Error(w, fmt.Sprintf("Package %d: registry_name is required", i), http.StatusBadRequest)
				return
			}
			if pkg.Name == "" {
				log.Printf("publish-oss: Package %d missing name from %s for repo %s", i, r.RemoteAddr, ossReq.RepositoryURL)
				http.Error(w, fmt.Sprintf("Package %d: name is required", i), http.StatusBadRequest)
				return
			}
			if pkg.Version == "" {
				log.Printf("publish-oss: Package %d missing version from %s for repo %s", i, r.RemoteAddr, ossReq.RepositoryURL)
				http.Error(w, fmt.Sprintf("Package %d: version is required", i), http.StatusBadRequest)
				return
			}
		}

		// Check if owner and repo are provided in the request body
		var owner, repo string
		if ossReq.Owner != "" && ossReq.Repo != "" {
			owner = ossReq.Owner
			repo = ossReq.Repo
		} else {
			// Extract owner and repo from GitHub URL
			var err error
			owner, repo, err = extractGitHubRepo(ossReq.RepositoryURL)
			if err != nil {
				log.Printf("publish-oss: Invalid GitHub URL from %s: %s - %v", r.RemoteAddr, ossReq.RepositoryURL, err)
				http.Error(w, "Invalid GitHub repository URL: "+err.Error(), http.StatusBadRequest)
				return
			}
		}

		// Check if a server with this name already exists in the registry
		expectedServerName := fmt.Sprintf("io.github.%s/%s", owner, repo)
		existingServers, _, err := registry.Search(expectedServerName, "", "", "", 1)
		if err != nil {
			log.Printf("publish-oss: Failed to check existing servers for %s: %v", expectedServerName, err)
			http.Error(w, "Failed to check existing servers: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// If we found any servers with this exact name, return a conflict error
		for _, server := range existingServers {
			if server.Name == expectedServerName {
				log.Printf("publish-oss: Server already exists from %s: %s", r.RemoteAddr, expectedServerName)
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
			log.Printf("publish-oss: Internal authentication service error - type assertion failed")
			http.Error(w, "Internal authentication service error", http.StatusInternalServerError)
			return
		}

		githubAuth := authServiceImpl.GetGitHubAuth()
		// When using ephemeral tokens, we pass empty string as token since we can't use ephemeral tokens with GitHub API
		// The FetchRepositoryInfo method will handle fetching public repos without auth
		githubToken := ""
		if ephemeralClaims == nil {
			// Registry owner is using a real GitHub token
			githubToken = token
		}
		repoInfo, err := githubAuth.FetchRepositoryInfo(r.Context(), githubToken, owner, repo)
		if err != nil {
			log.Printf("publish-oss: Failed to fetch GitHub repo info for %s/%s from %s: %v", owner, repo, r.RemoteAddr, err)
			http.Error(w, "Failed to fetch repository information: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Generate a unique server ID
		serverID, err := generateServerID()
		if err != nil {
			log.Printf("publish-oss: Failed to generate server ID: %v", err)
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
			Packages: ossReq.Packages,
		}

		// Call the publish method on the registry service
		err = registry.Publish(&serverDetail)
		if err != nil {
			// Check for specific error types and return appropriate HTTP status codes
			if database.ErrInvalidVersion != nil && strings.Contains(err.Error(), "invalid version") {
				log.Printf("publish-oss: Invalid version error for %s from %s: %v", serverDetail.Name, r.RemoteAddr, err)
				http.Error(w, "Failed to publish server details: "+err.Error(), http.StatusBadRequest)
				return
			}
			if database.ErrAlreadyExists != nil && strings.Contains(err.Error(), "already exists") {
				log.Printf("publish-oss: Server already exists error for %s from %s: %v", serverDetail.Name, r.RemoteAddr, err)
				http.Error(w, "Server already exists in registry", http.StatusConflict)
				return
			}
			log.Printf("publish-oss: Failed to publish server %s from %s: %v", serverDetail.Name, r.RemoteAddr, err)
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

		// Log successful publication
		log.Printf("publish-oss: Successfully published server %s (ID: %s) by %s from %s", serverDetail.Name, serverDetail.ID, publishedBy, r.RemoteAddr)

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"message":      "OSS server publication successful",
			"id":           serverDetail.ID,
			"name":         serverDetail.Name,
			"repository":   serverDetail.Repository,
			"published_by": publishedBy,
		}); err != nil {
			log.Printf("publish-oss: Failed to encode response for %s: %v", serverDetail.Name, err)
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

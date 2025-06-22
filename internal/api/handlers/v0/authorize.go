package v0

import (
	"encoding/json"
	"net/http"

	"github.com/modelcontextprotocol/registry/internal/auth"
)

// AuthorizeRequest represents the request body for the authorize endpoint
type AuthorizeRequest struct {
	GitHubToken string `json:"github_token"`
}

// AuthorizeResponse represents the response from the authorize endpoint
type AuthorizeResponse struct {
	EphemeralToken string `json:"ephemeral_token"`
	ExpiresIn      int    `json:"expires_in"` // seconds
}

// AuthorizeHandler handles requests to generate ephemeral tokens for GitHub users
func AuthorizeHandler(authService auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request body
		var req AuthorizeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Validate GitHub token is provided
		if req.GitHubToken == "" {
			http.Error(w, "GitHub token is required", http.StatusBadRequest)
			return
		}

		// Generate ephemeral token
		ephemeralToken, err := authService.GenerateEphemeralTokenForGitHubUser(r.Context(), req.GitHubToken)
		if err != nil {
			http.Error(w, "Failed to authorize: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Return response
		resp := AuthorizeResponse{
			EphemeralToken: ephemeralToken,
			ExpiresIn:      3600, // 1 hour in seconds
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

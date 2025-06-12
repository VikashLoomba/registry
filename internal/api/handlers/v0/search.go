// Package v0 contains API handlers for version 0 of the API
package v0

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/registry/internal/service"
)

// SearchHandler returns a handler for searching registry items
func SearchHandler(registry service.RegistryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse query parameters
		query := r.URL.Query().Get("q")
		registryName := r.URL.Query().Get("registry_name")
		cursor := r.URL.Query().Get("cursor")
		limitStr := r.URL.Query().Get("limit")

		// Validate cursor if provided
		if cursor != "" {
			_, err := uuid.Parse(cursor)
			if err != nil {
				http.Error(w, "Invalid cursor parameter", http.StatusBadRequest)
				return
			}
		}

		// Default limit if not specified
		limit := 30

		// Try to parse limit from query param
		if limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err != nil {
				http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
				return
			}

			// Check if limit is within reasonable bounds
			if parsedLimit <= 0 {
				http.Error(w, "Limit must be greater than 0", http.StatusBadRequest)
				return
			}

			if parsedLimit > 100 {
				// Cap maximum limit to prevent excessive queries
				limit = 100
			} else {
				limit = parsedLimit
			}
		}

		// Use the Search method to get filtered results
		registries, nextCursor, err := registry.Search(query, registryName, cursor, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create paginated response (reusing the same structure as ServersHandler)
		response := PaginatedResponse{
			Data: registries,
		}

		// Add metadata if there's a next cursor
		if nextCursor != "" {
			response.Metadata = Metadata{
				NextCursor: nextCursor,
				Count:      len(registries),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}
package service

import (
	"context"
	"regexp"
	"time"

	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/model"
)

// registryServiceImpl implements the RegistryService interface using our Database
type registryServiceImpl struct {
	db database.Database
}

// NewRegistryServiceWithDB creates a new registry service with the provided database
//
//nolint:ireturn // Factory function intentionally returns interface for dependency injection
func NewRegistryServiceWithDB(db database.Database) RegistryService {
	return &registryServiceImpl{
		db: db,
	}
}

// GetAll returns all registry entries
func (s *registryServiceImpl) GetAll() ([]model.Server, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the database's List method with no filters to get all entries
	entries, _, err := s.db.List(ctx, nil, "", 30)
	if err != nil {
		return nil, err
	}

	// Convert from []*model.Server to []model.Server
	result := make([]model.Server, len(entries))
	for i, entry := range entries {
		result[i] = *entry
	}

	return result, nil
}

// List returns registry entries with cursor-based pagination
func (s *registryServiceImpl) List(cursor string, limit int) ([]model.Server, string, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// If limit is not set or negative, use a default limit
	if limit <= 0 {
		limit = 30
	}

	// Use the database's List method with pagination
	entries, nextCursor, err := s.db.List(ctx, nil, cursor, limit)
	if err != nil {
		return nil, "", err
	}

	// Convert from []*model.Server to []model.Server
	result := make([]model.Server, len(entries))
	for i, entry := range entries {
		result[i] = *entry
	}

	return result, nextCursor, nil
}

// GetByID retrieves a specific server detail by its ID
func (s *registryServiceImpl) GetByID(id string) (*model.ServerDetail, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the database's GetByID method to retrieve the server detail
	serverDetail, err := s.db.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return serverDetail, nil
}

// Publish adds a new server detail to the registry
func (s *registryServiceImpl) Publish(serverDetail *model.ServerDetail) error {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if serverDetail == nil {
		return database.ErrInvalidInput
	}

	err := s.db.Publish(ctx, serverDetail)
	if err != nil {
		return err
	}

	return nil
}

// Search searches for servers by name with optional registry_name filter
func (s *registryServiceImpl) Search(query string, registryName string, url string, cursor string, limit int) ([]model.Server, string, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// If limit is not set or negative, use a default limit
	if limit <= 0 {
		limit = 30
	}

	// Build the filter map
	filter := make(map[string]interface{})

	// Use MongoDB text search instead of regex to prevent ReDoS attacks
	if query != "" {
		filter["$text"] = map[string]interface{}{
			"$search": query,
		}
	}

	// Add registry_name filter if provided
	if registryName != "" {
		filter["packages.registry_name"] = registryName
	}

	// Add URL filter if provided - exact match for security
	if url != "" {
		filter["repository.url"] = url
	}

	// Use the database's List method with search filters
	entries, nextCursor, err := s.db.List(ctx, filter, cursor, limit)
	if err != nil {
		return nil, "", err
	}

	// Convert from []*model.Server to []model.Server
	result := make([]model.Server, len(entries))
	for i, entry := range entries {
		result[i] = *entry
	}

	return result, nextCursor, nil
}

// SearchDetails searches for servers by name with optional registry_name filter and returns full details
func (s *registryServiceImpl) SearchDetails(query string, registryName string, url string, cursor string, limit int) ([]model.ServerDetail, string, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// If limit is not set or negative, use a default limit
	if limit <= 0 {
		limit = 30
	}

	// Build the filter map
	filter := make(map[string]interface{})

	// Use MongoDB text search for full-word matches
	if query != "" {
		filter["$text"] = map[string]interface{}{
			"$search": query,
		}
	}

	// Add registry_name filter if provided
	if registryName != "" {
		filter["packages.registry_name"] = registryName
	}

	// Add URL filter if provided - exact match for security
	if url != "" {
		filter["repository.url"] = url
	}

	// Use the database's ListDetails method with search filters
	entries, nextCursor, err := s.db.ListDetails(ctx, filter, cursor, limit)
	if err != nil {
		return nil, "", err
	}

	// If text search returned no results and we have a query, try with a case-insensitive regex
	// This helps with partial matches and compound words
	if len(entries) == 0 && query != "" {
		// Remove text search and add regex search
		delete(filter, "$text")
		
		// Escape special regex characters to prevent regex injection
		escapedQuery := escapeRegex(query)
		
		// Create a safe regex pattern with case-insensitive search on multiple fields
		filter["$or"] = []map[string]interface{}{
			{"name": map[string]interface{}{
				"$regex": escapedQuery,
				"$options": "i",
			}},
			{"description": map[string]interface{}{
				"$regex": escapedQuery,
				"$options": "i",
			}},
			{"packages.name": map[string]interface{}{
				"$regex": escapedQuery,
				"$options": "i",
			}},
		}
		
		// Retry with regex search
		entries, nextCursor, err = s.db.ListDetails(ctx, filter, cursor, limit)
		if err != nil {
			return nil, "", err
		}
	}

	// Convert from []*model.ServerDetail to []model.ServerDetail
	result := make([]model.ServerDetail, len(entries))
	for i, entry := range entries {
		result[i] = *entry
	}

	return result, nextCursor, nil
}

// escapeRegex escapes special regex characters to prevent regex injection
func escapeRegex(input string) string {
	// Escape all special regex characters
	return regexp.QuoteMeta(input)
}
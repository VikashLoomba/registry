package service

import "github.com/modelcontextprotocol/registry/internal/model"

// RegistryService defines the interface for registry operations
type RegistryService interface {
	List(cursor string, limit int) ([]model.Server, string, error)
	GetByID(id string) (*model.ServerDetail, error)
	Publish(serverDetail *model.ServerDetail) error
	Search(query string, registryName string, url string, cursor string, limit int) ([]model.Server, string, error)
	SearchDetails(query string, registryName string, url string, cursor string, limit int) ([]model.ServerDetail, string, error)
}

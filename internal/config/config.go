package config

import (
	"fmt"
	"strings"

	env "github.com/caarlos0/env/v11"
)

type DatabaseType string

const (
	DatabaseTypeMongoDB DatabaseType = "mongodb"
	DatabaseTypeMemory  DatabaseType = "memory"
)

// Config holds the application configuration
type Config struct {
	ServerAddress               string       `env:"SERVER_ADDRESS" envDefault:":8080"`
	Environment                 string       `env:"ENVIRONMENT" envDefault:"development"`
	DatabaseType                DatabaseType `env:"DATABASE_TYPE" envDefault:"mongodb"`
	DatabaseURL                 string       `env:"DATABASE_URL" envDefault:"mongodb://localhost:27017"`
	DatabaseName                string       `env:"DATABASE_NAME" envDefault:"mcp-registry"`
	CollectionName              string       `env:"COLLECTION_NAME" envDefault:"servers_v2"`
	LogLevel                    string       `env:"LOG_LEVEL" envDefault:"info"`
	SeedFilePath                string       `env:"SEED_FILE_PATH" envDefault:"data/seed.json"`
	SeedImport                  bool         `env:"SEED_IMPORT" envDefault:"true"`
	Version                     string       `env:"VERSION" envDefault:"dev"`
	GithubClientID              string       `env:"GITHUB_CLIENT_ID" envDefault:""`
	GithubClientSecret          string       `env:"GITHUB_CLIENT_SECRET" envDefault:""`
	RegistryOwnerGithubUsername string       `env:"REGISTRY_OWNER_GITHUB_USERNAME" envDefault:""`
	EphemeralTokenSecret        string       `env:"EPHEMERAL_TOKEN_SECRET" envDefault:""`
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	var cfg Config
	err := env.ParseWithOptions(&cfg, env.Options{
		Prefix: "MCP_REGISTRY_",
	})
	if err != nil {
		panic(err)
	}
	return &cfg
}

// Validate checks that all required environment variables are set
func (c *Config) Validate() error {
	var missingVars []string

	// Check required GitHub configuration
	if c.GithubClientID == "" {
		missingVars = append(missingVars, "MCP_REGISTRY_GITHUB_CLIENT_ID")
	}
	if c.GithubClientSecret == "" {
		missingVars = append(missingVars, "MCP_REGISTRY_GITHUB_CLIENT_SECRET")
	}
	if c.RegistryOwnerGithubUsername == "" {
		missingVars = append(missingVars, "MCP_REGISTRY_REGISTRY_OWNER_GITHUB_USERNAME")
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missingVars, ", "))
	}

	return nil
}

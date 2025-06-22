package v0_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	v0 "github.com/modelcontextprotocol/registry/internal/api/handlers/v0"
	"github.com/modelcontextprotocol/registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRegistryService Search and SearchDetails methods are defined in publish_test.go

func TestSearchHandler(t *testing.T) {
	testCases := []struct {
		name            string
		method          string
		queryParams     string
		setupMocks      func(*MockRegistryService)
		expectedStatus  int
		expectedServers []model.ServerDetail
		expectedMeta    *v0.Metadata
		expectedError   string
	}{
		{
			name:        "successful search with query only",
			method:      http.MethodGet,
			queryParams: "?q=test",
			setupMocks: func(registry *MockRegistryService) {
				servers := []model.ServerDetail{
					{
						Server: model.Server{
							ID:          "550e8400-e29b-41d4-a716-446655440001",
							Name:        "test-server-1",
							Description: "First test server",
							Repository: model.Repository{
								URL:    "https://github.com/example/test-server-1",
								Source: "github",
								ID:     "example/test-server-1",
							},
							VersionDetail: model.VersionDetail{
								Version:     "1.0.0",
								ReleaseDate: "2025-05-25T00:00:00Z",
								IsLatest:    true,
							},
						},
						Packages: []model.Package{
							{
								RegistryName: "npm",
								Name:         "@test/server-1",
								Version:      "1.0.0",
							},
						},
					},
				}
				registry.Mock.On("SearchDetails", "test", "", "", "", 30).Return(servers, "", nil)
			},
			expectedStatus: http.StatusOK,
			expectedServers: []model.ServerDetail{
				{
					Server: model.Server{
						ID:          "550e8400-e29b-41d4-a716-446655440001",
						Name:        "test-server-1",
						Description: "First test server",
						Repository: model.Repository{
							URL:    "https://github.com/example/test-server-1",
							Source: "github",
							ID:     "example/test-server-1",
						},
						VersionDetail: model.VersionDetail{
							Version:     "1.0.0",
							ReleaseDate: "2025-05-25T00:00:00Z",
							IsLatest:    true,
						},
					},
					Packages: []model.Package{
						{
							RegistryName: "npm",
							Name:         "@test/server-1",
							Version:      "1.0.0",
						},
					},
				},
			},
		},
		{
			name:        "successful search with registry_name filter",
			method:      http.MethodGet,
			queryParams: "?q=server&registry_name=npm",
			setupMocks: func(registry *MockRegistryService) {
				servers := []model.ServerDetail{
					{
						Server: model.Server{
							ID:          "550e8400-e29b-41d4-a716-446655440002",
							Name:        "npm-server",
							Description: "NPM server",
							Repository: model.Repository{
								URL:    "https://github.com/example/npm-server",
								Source: "github",
								ID:     "example/npm-server",
							},
							VersionDetail: model.VersionDetail{
								Version:     "2.0.0",
								ReleaseDate: "2025-05-26T00:00:00Z",
								IsLatest:    true,
							},
						},
						Packages: []model.Package{
							{
								RegistryName: "npm",
								Name:         "npm-server",
								Version:      "2.0.0",
							},
						},
					},
				}
				registry.Mock.On("SearchDetails", "server", "npm", "", "", 30).Return(servers, "", nil)
			},
			expectedStatus: http.StatusOK,
			expectedServers: []model.ServerDetail{
				{
					Server: model.Server{
						ID:          "550e8400-e29b-41d4-a716-446655440002",
						Name:        "npm-server",
						Description: "NPM server",
						Repository: model.Repository{
							URL:    "https://github.com/example/npm-server",
							Source: "github",
							ID:     "example/npm-server",
						},
						VersionDetail: model.VersionDetail{
							Version:     "2.0.0",
							ReleaseDate: "2025-05-26T00:00:00Z",
							IsLatest:    true,
						},
					},
					Packages: []model.Package{
						{
							RegistryName: "npm",
							Name:         "npm-server",
							Version:      "2.0.0",
						},
					},
				},
			},
		},
		{
			name:        "successful search with pagination",
			method:      http.MethodGet,
			queryParams: "?q=test&cursor=550e8400-e29b-41d4-a716-446655440000&limit=10",
			setupMocks: func(registry *MockRegistryService) {
				servers := []model.ServerDetail{
					{
						Server: model.Server{
							ID:          "550e8400-e29b-41d4-a716-446655440003",
							Name:        "test-server-3",
							Description: "Third test server",
							Repository: model.Repository{
								URL:    "https://github.com/example/test-server-3",
								Source: "github",
								ID:     "example/test-server-3",
							},
							VersionDetail: model.VersionDetail{
								Version:     "1.5.0",
								ReleaseDate: "2025-05-27T00:00:00Z",
								IsLatest:    true,
							},
						},
					},
				}
				nextCursor := uuid.New().String()
				registry.Mock.On("SearchDetails", "test", "", "", mock.AnythingOfType("string"), 10).Return(servers, nextCursor, nil)
			},
			expectedStatus: http.StatusOK,
			expectedServers: []model.ServerDetail{
				{
					Server: model.Server{
						ID:          "550e8400-e29b-41d4-a716-446655440003",
						Name:        "test-server-3",
						Description: "Third test server",
						Repository: model.Repository{
							URL:    "https://github.com/example/test-server-3",
							Source: "github",
							ID:     "example/test-server-3",
						},
						VersionDetail: model.VersionDetail{
							Version:     "1.5.0",
							ReleaseDate: "2025-05-27T00:00:00Z",
							IsLatest:    true,
						},
					},
				},
			},
			expectedMeta: &v0.Metadata{
				NextCursor: "", // Will be dynamically set in the test
				Count:      1,
			},
		},
		{
			name:        "search with no results",
			method:      http.MethodGet,
			queryParams: "?q=nonexistent",
			setupMocks: func(registry *MockRegistryService) {
				registry.Mock.On("SearchDetails", "nonexistent", "", "", "", 30).Return([]model.ServerDetail{}, "", nil)
			},
			expectedStatus:  http.StatusOK,
			expectedServers: []model.ServerDetail{},
		},
		{
			name:           "invalid cursor parameter",
			method:         http.MethodGet,
			queryParams:    "?q=test&cursor=invalid-uuid",
			setupMocks:     func(_ *MockRegistryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid cursor parameter",
		},
		{
			name:           "invalid limit parameter - non-numeric",
			method:         http.MethodGet,
			queryParams:    "?q=test&limit=abc",
			setupMocks:     func(_ *MockRegistryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid limit parameter",
		},
		{
			name:           "invalid limit parameter - zero",
			method:         http.MethodGet,
			queryParams:    "?q=test&limit=0",
			setupMocks:     func(_ *MockRegistryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Limit must be greater than 0",
		},
		{
			name:           "invalid limit parameter - negative",
			method:         http.MethodGet,
			queryParams:    "?q=test&limit=-5",
			setupMocks:     func(_ *MockRegistryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Limit must be greater than 0",
		},
		{
			name:        "registry service error",
			method:      http.MethodGet,
			queryParams: "?q=test",
			setupMocks: func(registry *MockRegistryService) {
				registry.Mock.On("SearchDetails", "test", "", "", "", 30).Return([]model.ServerDetail{}, "", errors.New("database connection error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "database connection error",
		},
		{
			name:           "method not allowed",
			method:         http.MethodPost,
			setupMocks:     func(_ *MockRegistryService) {},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "Method not allowed",
		},
		{
			name:        "successful search with limit capping at 100",
			method:      http.MethodGet,
			queryParams: "?q=test&limit=150",
			setupMocks: func(registry *MockRegistryService) {
				servers := []model.ServerDetail{}
				registry.Mock.On("SearchDetails", "test", "", "", "", 100).Return(servers, "", nil)
			},
			expectedStatus:  http.StatusOK,
			expectedServers: []model.ServerDetail{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock registry service
			mockRegistry := new(MockRegistryService)
			tc.setupMocks(mockRegistry)

			// Create handler
			handler := v0.SearchHandler(mockRegistry)

			// Create request
			url := "/v0/search" + tc.queryParams
			req, err := http.NewRequestWithContext(context.Background(), tc.method, url, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call the handler
			handler.ServeHTTP(rr, req)

			// Check status code
			assert.Equal(t, tc.expectedStatus, rr.Code)

			if tc.expectedStatus == http.StatusOK {
				// Check content type
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

				// Parse response body
				var resp v0.PaginatedResponseDetails
				err = json.NewDecoder(rr.Body).Decode(&resp)
				assert.NoError(t, err)

				// Check the response data
				assert.Equal(t, tc.expectedServers, resp.Data)

				// Check metadata if expected
				if tc.expectedMeta != nil {
					assert.Equal(t, tc.expectedMeta.Count, resp.Metadata.Count)
					if tc.expectedMeta.NextCursor != "" {
						assert.NotEmpty(t, resp.Metadata.NextCursor)
					}
				}
			} else if tc.expectedError != "" {
				// Check error message for non-200 responses
				assert.Contains(t, rr.Body.String(), tc.expectedError)
			}

			// Verify mock expectations
			mockRegistry.Mock.AssertExpectations(t)
		})
	}
}

// TestSearchHandlerIntegration tests the search handler with actual HTTP requests
func TestSearchHandlerIntegration(t *testing.T) {
	// Create mock registry service
	mockRegistry := new(MockRegistryService)

	servers := []model.ServerDetail{
		{
			Server: model.Server{
				ID:          "550e8400-e29b-41d4-a716-446655440004",
				Name:        "integration-test-server",
				Description: "Integration test server",
				Repository: model.Repository{
					URL:    "https://github.com/example/integration-test",
					Source: "github",
					ID:     "example/integration-test",
				},
				VersionDetail: model.VersionDetail{
					Version:     "1.0.0",
					ReleaseDate: "2025-05-27T00:00:00Z",
					IsLatest:    true,
				},
			},
			Packages: []model.Package{
				{
					RegistryName: "npm",
					Name:         "@integration/test-server",
					Version:      "1.0.0",
				},
			},
		},
	}

	mockRegistry.Mock.On("SearchDetails", "integration", "", "", "", 30).Return(servers, "", nil)

	// Create test server
	server := httptest.NewServer(v0.SearchHandler(mockRegistry))
	defer server.Close()

	// Send request to the test server
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"?q=integration", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Check content type
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Parse response body
	var paginatedResp v0.PaginatedResponseDetails
	err = json.NewDecoder(resp.Body).Decode(&paginatedResp)
	assert.NoError(t, err)

	// Check the response data
	assert.Equal(t, servers, paginatedResp.Data)
	assert.Empty(t, paginatedResp.Metadata.NextCursor)

	// Verify mock expectations
	mockRegistry.Mock.AssertExpectations(t)
}

#!/bin/bash

set -e 

echo "=================================================="
echo "MCP Registry Search Endpoint Test Script"
echo "=================================================="
echo "This script expects the MCP Registry server to be running locally."
echo "Please ensure the server is started using one of the following methods:"
echo "  • Docker Compose: docker compose up"
echo "  • Direct execution: go run cmd/registry/main.go"
echo "  • Built binary: ./build/registry"
echo "=================================================="
echo ""

# Default values
HOST="http://localhost:8085"
QUERY=""
URL=""

# Display usage information
function show_usage {
  echo "Usage: $0 [options]"
  echo "Options:"
  echo "  -h, --host      Base URL of the MCP Registry service (default: http://localhost:8085)"
  echo "  -q, --query     Search query to test (default: context7)"
  echo "  -u, --url       Repository URL to filter by (optional)"
  echo "  --help          Show this help message"
  exit 1
}

# Check if jq is installed
if ! command -v jq &> /dev/null; then
  echo "Error: jq is required but not installed."
  echo "Please install jq using your package manager, for example:"
  echo "  brew install jq (macOS)"
  echo "  apt-get install jq (Debian/Ubuntu)"
  echo "  yum install jq (CentOS/RHEL)"
  exit 1
fi

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
  case $1 in
    -h|--host) HOST="$2"; shift ;;
    -q|--query) QUERY="$2"; shift ;;
    -u|--url) URL="$2"; shift ;;
    --help) show_usage ;;
    *) echo "Unknown parameter: $1"; show_usage ;;
  esac
  shift
done

# Test search endpoint
test_search() {
  # Build query string
  query_params="q=$QUERY"
  
  # Add URL parameter if provided
  if [[ -n "$URL" ]]; then
    query_params="${query_params}&url=$(printf '%s' "$URL" | jq -sRr @uri)"
  fi
  
  search_url="$HOST/v0/search?$query_params"
  
  echo "Testing search endpoint: $search_url"
  
  # Get response and status code
  http_response=$(curl -s "$search_url")
  status_code=$(curl -s -o /dev/null -w "%{http_code}" "$search_url")
  
  echo "Status Code: $status_code"
  
  if [[ $status_code == 2* ]]; then
    # Parse and display JSON with jq
    echo "Response Summary:"
    
    # Check if we have valid JSON response
    if echo "$http_response" | jq empty 2>/dev/null; then
      # Count results
      result_count=$(echo "$http_response" | jq '.servers | length' 2>/dev/null || echo "0")
      echo "Total search results: $result_count"
      
      # Display server names if any results found
      if [[ $result_count -gt 0 ]]; then
        echo "Server Names:"
        echo "$http_response" | jq -r '.servers[].name' 2>/dev/null || echo "Could not extract server names"
        
        # Show pagination metadata if available
        echo -e "\nPagination Metadata:"
        echo "$http_response" | jq '.metadata // "No pagination metadata"'
      else
        search_criteria="query: $QUERY"
        if [[ -n "$URL" ]]; then
          search_criteria="$search_criteria, URL: $URL"
        fi
        echo "No search results found for $search_criteria"
      fi
      
      # Show the full response
      echo -e "\nFull Response:"
      echo "$http_response" | jq '.'
      
    else
      echo "Response is not valid JSON:"
      echo "$http_response"
      return 1
    fi
    
    echo "Search request successful"
    echo "-------------------------------------"
    return 0
  else
    echo "Response:"
    echo "$http_response" | jq '.' 2>/dev/null || echo "$http_response"
    echo "Search request failed"
    echo "-------------------------------------"
    return 1
  fi
}

# Run the search test
echo "Running search test with query: '$QUERY'"
if [[ -n "$URL" ]]; then
  echo "Filtering by URL: '$URL'"
fi
echo "Against server: $HOST"
echo ""

test_search

# Check the result
if [[ $? -eq 0 ]]; then
  echo "Search test passed successfully!"
  exit 0
else
  echo "Search test failed!"
  exit 1
fi 
#!/bin/bash

# Test script for publish-oss endpoint with owner and repo fields

echo "Testing publish-oss endpoint with owner and repo fields..."

# Test 1: Using owner and repo fields instead of extracting from URL
echo -e "\n1. Testing with owner and repo fields provided:"
curl -X POST http://localhost:8080/v0/publish-oss \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "repository_url": "https://github.com/test/invalid-url-format",
    "owner": "modelcontextprotocol",
    "repo": "test-server",
    "packages": [
      {
        "registry_name": "npm",
        "name": "@modelcontextprotocol/test-server",
        "version": "1.0.0"
      }
    ]
  }' -v

# Test 2: Using URL extraction (original behavior)
echo -e "\n\n2. Testing without owner/repo fields (URL extraction):"
curl -X POST http://localhost:8080/v0/publish-oss \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "repository_url": "https://github.com/modelcontextprotocol/test-server",
    "packages": [
      {
        "registry_name": "npm",
        "name": "@modelcontextprotocol/test-server",
        "version": "1.0.0"
      }
    ]
  }' -v

# Test 3: Only owner provided (should fall back to URL extraction)
echo -e "\n\n3. Testing with only owner field (should use URL extraction):"
curl -X POST http://localhost:8080/v0/publish-oss \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "repository_url": "https://github.com/modelcontextprotocol/test-server",
    "owner": "different-owner",
    "packages": [
      {
        "registry_name": "npm",
        "name": "@modelcontextprotocol/test-server",
        "version": "1.0.0"
      }
    ]
  }' -v

echo -e "\nTests completed."
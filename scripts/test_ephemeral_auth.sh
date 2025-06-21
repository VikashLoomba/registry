#!/bin/bash

# Test script for ephemeral token authorization flow
# This demonstrates how a VSCode extension would authorize and publish

echo "=== Ephemeral Token Authorization Flow Test ==="
echo

# Base URL - adjust if your server is running on a different port
BASE_URL="http://localhost:8080"

# Test GitHub token (replace with a valid token for testing)
GITHUB_TOKEN="${TEST_GITHUB_TOKEN:-your_github_token_here}"

if [ "$GITHUB_TOKEN" = "your_github_token_here" ]; then
    echo "Error: Please set TEST_GITHUB_TOKEN environment variable with a valid GitHub token"
    echo "Example: export TEST_GITHUB_TOKEN='gho_xxxxx'"
    exit 1
fi

echo "Step 1: Authorize with GitHub token to get ephemeral token"
echo "POST ${BASE_URL}/v0/authorize"
echo

RESPONSE=$(curl -s -X POST "${BASE_URL}/v0/authorize" \
    -H "Content-Type: application/json" \
    -d "{\"github_token\": \"${GITHUB_TOKEN}\"}")

echo "Response: ${RESPONSE}"
echo

# Extract ephemeral token from response using jq (if available) or sed
if command -v jq >/dev/null 2>&1; then
    EPHEMERAL_TOKEN=$(echo "$RESPONSE" | jq -r '.ephemeral_token')
else
    EPHEMERAL_TOKEN=$(echo "$RESPONSE" | sed -n 's/.*"ephemeral_token":"\([^"]*\)".*/\1/p')
fi

if [ -z "$EPHEMERAL_TOKEN" ] || [ "$EPHEMERAL_TOKEN" = "null" ]; then
    echo "Error: Failed to get ephemeral token"
    exit 1
fi

echo "Ephemeral token received (truncated): ${EPHEMERAL_TOKEN:0:50}..."
echo

echo "Step 2: Use ephemeral token to publish an OSS server"
echo "POST ${BASE_URL}/v0/publish-oss"
echo

# Example repository URL - you can change this
REPO_URL="https://github.com/modelcontextprotocol/example-server"

PUBLISH_RESPONSE=$(curl -s -X POST "${BASE_URL}/v0/publish-oss" \
    -H "Authorization: Bearer ${EPHEMERAL_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"repository_url\": \"${REPO_URL}\"}")

echo "Publish Response: ${PUBLISH_RESPONSE}"
echo

# Check if publication was successful
if echo "$PUBLISH_RESPONSE" | grep -q "OSS server publication successful"; then
    echo "✅ Success! Server was published using ephemeral token"
else
    echo "❌ Failed to publish server"
fi

echo
echo "=== Test Complete ===" 
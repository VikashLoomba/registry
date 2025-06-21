# Ephemeral Token Authorization for VSCode Extension

This document describes the ephemeral token authorization flow that allows VSCode extension users to publish OSS servers to the registry.

## Overview

The ephemeral token system allows users who have authenticated with GitHub in their VSCode extension to publish OSS servers without being the registry owner. The flow works as follows:

1. User authenticates with GitHub in VSCode extension
2. Extension calls the `/v0/authorize` endpoint with the user's GitHub token
3. Registry validates the GitHub token and issues a short-lived ephemeral token
4. Extension uses the ephemeral token to call `/v0/publish-oss`

## API Endpoints

### POST /v0/authorize

Generates an ephemeral token for a GitHub user.

**Request:**
```json
{
  "github_token": "gho_xxxxxxxxxxxx"
}
```

**Response:**
```json
{
  "ephemeral_token": "base64_encoded_token",
  "expires_in": 3600  // seconds
}
```

### POST /v0/publish-oss

Publishes an OSS server. Now accepts either:
- Registry owner's GitHub token
- Ephemeral token from `/v0/authorize`

**Request:**
```http
Authorization: Bearer <ephemeral_token>
Content-Type: application/json

{
  "repository_url": "https://github.com/owner/repo"
}
```

**Response:**
```json
{
  "message": "OSS server publication successful",
  "id": "server-id",
  "name": "io.github.owner/repo",
  "repository": {...},
  "published_by": "github_username"  // Shows who published it
}
```

## Security Features

1. **Token Expiration**: Ephemeral tokens expire after 1 hour
2. **HMAC Signature**: Tokens are signed with HMAC-SHA256 to prevent tampering
3. **User Attribution**: The response includes who published the server
4. **GitHub Validation**: Only valid GitHub users can obtain ephemeral tokens

## Configuration

Set the following environment variable to use a consistent secret for token signing:

```bash
export MCP_REGISTRY_EPHEMERAL_TOKEN_SECRET="your-secret-key"
```

If not set, a random secret will be generated at startup (tokens won't persist across restarts).

## Testing

Use the provided test script to test the flow:

```bash
export TEST_GITHUB_TOKEN="your_github_token"
./scripts/test_ephemeral_auth.sh
```

## VSCode Extension Integration

The VSCode extension should:

1. On initialization, when the user is authenticated with GitHub:
   ```typescript
   const response = await fetch(`${REGISTRY_URL}/v0/authorize`, {
     method: 'POST',
     headers: { 'Content-Type': 'application/json' },
     body: JSON.stringify({ github_token: userGitHubToken })
   });
   const { ephemeral_token, expires_in } = await response.json();
   ```

2. Store the ephemeral token and refresh before expiration

3. Use the ephemeral token for publishing:
   ```typescript
   const publishResponse = await fetch(`${REGISTRY_URL}/v0/publish-oss`, {
     method: 'POST',
     headers: {
       'Authorization': `Bearer ${ephemeralToken}`,
       'Content-Type': 'application/json'
     },
     body: JSON.stringify({ repository_url: repoUrl })
   });
   
# phenotype-go-auth

Shared Go module for authentication and token management across Phenotype services.

## Features

- **TokenStorage Interface**: Generic interface for OAuth2 token persistence
- **BaseTokenStorage**: Base implementation with common token fields
- **PKCE Support**: RFC 7636 compliant PKCE code generation
- **OAuth2 Server**: Local HTTP server for handling OAuth callbacks
- **Token Management**: Load, save, and clear token data securely

## Installation

```bash
go get github.com/KooshaPari/phenotype-go-auth
```

## Quick Start

```go
import "github.com/KooshaPari/phenotype-go-auth"

// Create token storage
storage := auth.NewBaseTokenStorage("/path/to/token.json")

// Load from file
if err := storage.Load(); err != nil {
    log.Fatal(err)
}

// Generate PKCE codes for OAuth
codes, err := auth.GeneratePKCECodes()
if err != nil {
    log.Fatal(err)
}

// Start OAuth callback server
server := auth.NewOAuthServer(8080)
if err := server.Start(); err != nil {
    log.Fatal(err)
}

// Wait for callback
result, err := server.WaitForCallback(5 * time.Minute)
```

## License

MIT

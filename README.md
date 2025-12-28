# Vanish

> An ephemeral messaging platform designed for secure, single-use information transfer.

Vanish is a zero-knowledge, burn-on-read messaging system for sharing sensitive information (passwords, API keys, certificates) in enterprise environments. Messages are encrypted client-side, stored in volatile memory, and permanently destroyed after a single read.

## Features

- **Zero-Knowledge Architecture**: Server never sees plaintext data
- **Client-Side Encryption**: AES-256-GCM encryption before transmission
- **Burn-on-Read**: Messages automatically destroyed after viewing
- **No DOM Rendering**: Secrets copied directly to clipboard for maximum security
- **Ephemeral Storage**: Redis in-memory storage with no persistence
- **Time-Limited**: Configurable TTL (1 hour to 7 days)
- **Atomic Operations**: Lua scripts ensure messages can only be read once

## Security Model

### Client-Side Security
- **Encryption**: AES-256-GCM with 256-bit keys
- **Key Management**: Encryption key stays in URL fragment (#), never sent to server
- **No Visual Exposure**: Sensitive data never rendered to DOM
- **Memory Cleanup**: Best-effort memory clearing after clipboard write

### Server-Side Security
- **No Logging**: Request bodies never logged (NFR-02)
- **Volatile Storage**: Redis configured with no RDB/AOF persistence
- **Atomic Delete**: Lua scripts prevent race conditions
- **Security Headers**: HSTS, CSP, X-Frame-Options, etc.

## Technology Stack

### Backend
- **Language**: Go 1.21
- **Framework**: Gin
- **Storage**: Redis 7 (in-memory only)
- **Encryption**: Go crypto/aes standard library

### Frontend
- **Framework**: React 18
- **Build Tool**: Vite
- **Styling**: Tailwind CSS (dark theme)
- **Encryption**: Web Crypto API

## Quick Start

### Prerequisites
- Docker and Docker Compose
- (Optional) Go 1.21+ and Node.js 20+ for local development

### Run with Docker Compose

```bash
# Clone the repository
git clone https://github.com/milkiss/vanish.git
cd vanish

# Start all services
docker-compose up -d

# Access the application
open http://localhost:3000
```

Services will be available at:
- **Frontend**: http://localhost:3000
- **Backend API**: http://localhost:8080
- **PostgreSQL**: localhost:5432
- **Redis**: localhost:6379
- **Vault** (optional): http://localhost:8200

### Local Development

#### Backend
```bash
cd backend

# Install dependencies
go mod download

# Copy environment template
cp .env.example .env

# Start Redis (or use docker-compose.dev.yml)
docker-compose -f ../docker-compose.dev.yml up -d redis

# Run the server
go run cmd/server/main.go

# Run tests
go test ./tests/unit/...
go test ./tests/integration/...
```

#### Frontend
```bash
cd frontend

# Install dependencies
npm install

# Start development server
npm run dev

# Run tests
npm test

# Build for production
npm run build
```

## API Endpoints

### POST /api/messages
Create a new encrypted message.

**Request:**
```json
{
  "ciphertext": "base64-encoded-encrypted-data",
  "iv": "base64-encoded-initialization-vector",
  "ttl": 86400
}
```

**Response:**
```json
{
  "id": "message-id",
  "expiresAt": "2025-12-26T10:00:00Z"
}
```

### GET /api/messages/:id
Retrieve and burn a message (atomic operation).

**Response:**
```json
{
  "ciphertext": "base64-encoded-encrypted-data",
  "iv": "base64-encoded-initialization-vector"
}
```

### HEAD /api/messages/:id
Check if a message exists without burning it.

**Response:** 200 (exists) or 404 (not found)

### GET /health
Health check endpoint.

**Response:**
```json
{
  "status": "healthy"
}
```

## Configuration

### Backend Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | HTTP server port |
| `SERVER_HOST` | `0.0.0.0` | Server bind address |
| `REDIS_ADDRESS` | `localhost:6379` | Redis connection string |
| `REDIS_PASSWORD` | `` | Redis password (if any) |
| `REDIS_DB` | `0` | Redis database number |
| `ALLOWED_ORIGINS` | `http://localhost:5173,http://localhost:3000` | CORS origins |
| `DEFAULT_TTL` | `86400` | Default TTL (24 hours) |
| `MAX_TTL` | `604800` | Maximum TTL (7 days) |
| `MIN_TTL` | `3600` | Minimum TTL (1 hour) |

### Redis Configuration

Redis is configured with:
- `maxmemory 256mb`: Memory limit
- `maxmemory-policy volatile-ttl`: Evict expired keys first
- `save ""`: Disable RDB snapshots
- `appendonly no`: Disable AOF persistence

This ensures **no persistent storage** per NFR-01.

## Security Considerations

### Production Deployment

1. **HTTPS Required**: Clipboard API requires HTTPS
2. **Reverse Proxy**: Use nginx/Caddy for SSL termination
3. **Rate Limiting**: Implement rate limiting on API endpoints
4. **Monitoring**: Monitor Redis memory usage and request rates
5. **Audit Logs**: Log metadata only (never request bodies)

### Known Limitations

1. **Browser Compatibility**: Requires modern browser with Web Crypto API
2. **Memory Cleanup**: JavaScript memory cleanup is best-effort
3. **Screenshot Protection**: Cannot prevent screenshots/screen recording
4. **Clipboard History**: Some clipboard managers may cache clipboard content

## Testing

### Backend Tests
```bash
cd backend

# Unit tests
go test ./tests/unit/... -v

# Integration tests (requires Redis)
go test ./tests/integration/... -v

# Coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Frontend Tests
```bash
cd frontend

# Run tests
npm test

# Coverage
npm run test:coverage

# Watch mode
npm test -- --watch
```

## Project Structure

```
vanish/
├── backend/
│   ├── cmd/server/          # Application entry point
│   ├── internal/
│   │   ├── api/             # HTTP handlers, middleware, routes
│   │   ├── config/          # Configuration management
│   │   ├── models/          # Data models
│   │   └── storage/         # Redis storage implementation
│   ├── tests/               # Unit and integration tests
│   ├── Dockerfile
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── components/      # React components
│   │   ├── lib/             # Crypto, clipboard, API client
│   │   └── utils/           # URL helpers
│   ├── tests/               # Frontend tests
│   ├── Dockerfile
│   ├── nginx.conf
│   └── package.json
├── docker-compose.yml       # Production setup
├── docker-compose.dev.yml   # Development setup
└── README.md
```

## Enterprise Features ✅

Vanish now includes enterprise-grade integrations:

- ✅ **Okta SSO**: OpenID Connect authentication with MFA support
- ✅ **HashiCorp Vault**: Secure secrets management for credentials
- ✅ **Slack Integration**: Automatic DM notifications when secrets are sent
- ✅ **Email Notifications**: SMTP-based email alerts with HTML templates
- ✅ **User Authentication**: JWT-based auth with recipient verification
- ✅ **Recipient Targeting**: Messages locked to specific recipients
- ✅ **Metadata Audit Trail**: PostgreSQL tracking of who/when (not content)
- ✅ **Message History API**: Track sent/received messages

**See [ENTERPRISE_SETUP.md](ENTERPRISE_SETUP.md) for complete configuration guide.**

### Quick Enterprise Setup

```bash
# Enable Vault (stores all secrets securely)
VAULT_ENABLED=true
VAULT_ADDR=http://vault:8200
VAULT_TOKEN=your-token

# Enable Okta SSO
OKTA_ENABLED=true
OKTA_DOMAIN=your-domain.okta.com
OKTA_CLIENT_ID=your-client-id
OKTA_CLIENT_SECRET=your-secret

# Enable Slack notifications
SLACK_ENABLED=true
SLACK_BOT_TOKEN=xoxb-your-token

# Enable email notifications
EMAIL_ENABLED=true
SMTP_HOST=smtp.gmail.com
SMTP_USER=your-email@gmail.com
```

## License

See [LICENSE](LICENSE) file.

## Contributing

This is currently a private project. Contributions guidelines will be added if/when open-sourced.

## Support

For issues or questions, please contact the project maintainer.

---

**⚠️ Security Notice**: This system provides ephemeral messaging with client-side encryption. However, it cannot prevent:
- Recipient copying/saving the secret after viewing
- Screenshots or screen recording
- Compromised recipient systems
- Social engineering attacks

Use in conjunction with other security practices and policies.

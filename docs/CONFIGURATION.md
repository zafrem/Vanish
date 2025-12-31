# Configuration Guide

## Backend Environment Variables

### Server Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | HTTP server port |
| `SERVER_HOST` | `0.0.0.0` | Server bind address |
| `GIN_MODE` | `debug` | Gin mode: `debug`, `release`, `test` |

### Redis Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDRESS` | `localhost:6379` | Redis connection string |
| `REDIS_PASSWORD` | `` | Redis password (if any) |
| `REDIS_DB` | `0` | Redis database number (0-15) |

### PostgreSQL Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `vanish` | Database username |
| `DB_PASSWORD` | `vanish` | Database password |
| `DB_NAME` | `vanish` | Database name |
| `DB_SSLMODE` | `disable` | SSL mode: `disable`, `require`, `verify-full` |

### Security Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | `change-me-in-production` | JWT signing secret (CHANGE IN PROD!) |
| `JWT_DURATION` | `24` | JWT expiration in hours |
| `ALLOWED_ORIGINS` | `http://localhost:5173,http://localhost:3000` | CORS allowed origins |

### Message TTL Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DEFAULT_TTL` | `86400` | Default TTL in seconds (24 hours) |
| `MAX_TTL` | `604800` | Maximum TTL in seconds (7 days) |
| `MIN_TTL` | `3600` | Minimum TTL in seconds (1 hour) |

### HashiCorp Vault Integration

| Variable | Default | Description |
|----------|---------|-------------|
| `VAULT_ENABLED` | `false` | Enable Vault integration |
| `VAULT_ADDR` | `http://localhost:8200` | Vault server address |
| `VAULT_TOKEN` | `` | Vault authentication token |

### Okta SSO Integration

| Variable | Default | Description |
|----------|---------|-------------|
| `OKTA_ENABLED` | `false` | Enable Okta SSO |
| `OKTA_DOMAIN` | `` | Okta domain (e.g., `company.okta.com`) |
| `OKTA_CLIENT_ID` | `` | Okta OAuth2 client ID |
| `OKTA_CLIENT_SECRET` | `` | Okta OAuth2 client secret |
| `OKTA_REDIRECT_URL` | `` | OAuth2 redirect URL |

### Slack Integration

| Variable | Default | Description |
|----------|---------|-------------|
| `SLACK_ENABLED` | `false` | Enable Slack notifications |
| `SLACK_BOT_TOKEN` | `` | Slack bot token (xoxb-...) |
| `SLACK_SIGNING_SECRET` | `` | Slack signing secret |

### Email Integration

| Variable | Default | Description |
|----------|---------|-------------|
| `EMAIL_ENABLED` | `false` | Enable email notifications |
| `SMTP_HOST` | `` | SMTP server host |
| `SMTP_PORT` | `587` | SMTP server port |
| `SMTP_USER` | `` | SMTP username |
| `SMTP_PASSWORD` | `` | SMTP password |
| `EMAIL_FROM_ADDRESS` | `noreply@vanish.local` | From email address |
| `EMAIL_FROM_NAME` | `Vanish` | From display name |

## Redis Configuration

### Production Settings

```bash
redis-server \
  --maxmemory 256mb \
  --maxmemory-policy volatile-ttl \
  --save "" \
  --appendonly no
```

**Explanation:**
- `maxmemory 256mb`: Limit memory usage to 256 MB
- `maxmemory-policy volatile-ttl`: Evict keys with TTL when memory full
- `save ""`: Disable RDB snapshots (no persistence)
- `appendonly no`: Disable AOF (no persistence)

### Development Settings

For development, you can use the default Redis configuration or the provided docker-compose.dev.yml.

## Docker Compose Configuration

### Environment Variables in docker-compose.yml

All environment variables can be overridden using a `.env` file in the project root:

```bash
# .env file example
SERVER_PORT=8080
JWT_SECRET=super-secret-change-me
OKTA_ENABLED=true
OKTA_DOMAIN=mycompany.okta.com
```

Docker Compose will automatically load these variables.

### Volume Mounts

**PostgreSQL Data**
```yaml
volumes:
  - postgres-data:/var/lib/postgresql/data
```

This ensures PostgreSQL data persists across container restarts.

## Frontend Configuration

### Build-time Variables

The frontend uses Vite for building. Configure via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_URL` | `http://localhost:8080` | Backend API URL |

### Production Build

```bash
cd frontend
VITE_API_URL=https://api.vanish.example.com npm run build
```

## nginx Configuration

For production deployments, use nginx as a reverse proxy:

```nginx
server {
    listen 80;
    server_name vanish.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name vanish.example.com;

    ssl_certificate /etc/ssl/certs/vanish.crt;
    ssl_certificate_key /etc/ssl/private/vanish.key;

    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Frontend
    location / {
        root /usr/share/nginx/html;
        try_files $uri $uri/ /index.html;
    }

    # Backend API
    location /api/ {
        proxy_pass http://backend:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Security Best Practices

1. **Change Default Secrets**: Always change `JWT_SECRET` in production
2. **Use Strong Passwords**: For PostgreSQL and Redis (if enabled)
3. **Enable SSL/TLS**: Use HTTPS in production (required for clipboard API)
4. **Restrict Origins**: Set `ALLOWED_ORIGINS` to your actual domain
5. **Regular Updates**: Keep Docker images and dependencies updated
6. **Monitor Logs**: Set up log aggregation and monitoring
7. **Rate Limiting**: Implement at nginx/reverse proxy level
8. **Backup Strategy**: Regular PostgreSQL backups (metadata only)

## Troubleshooting

### Backend won't start

Check:
1. Redis is running and accessible
2. PostgreSQL is running and accessible
3. Environment variables are set correctly
4. Ports are not already in use

### Frontend can't connect to backend

Check:
1. `VITE_API_URL` is set correctly
2. CORS `ALLOWED_ORIGINS` includes frontend URL
3. Backend is running and accessible
4. Network/firewall rules allow connection

### Clipboard API not working

Requirements:
1. HTTPS (or localhost for development)
2. Modern browser with Web Crypto API support
3. User granted clipboard permission

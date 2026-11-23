# Vanish

**Ephemeral, zero-knowledge secret sharing.** Share passwords, API keys and certificates through a one-time link that destroys the secret the moment it is read.

Secrets are encrypted in the browser with AES-256-GCM before they leave the device. The server only ever stores ciphertext in Redis with a TTL, and deletes it atomically on first read. The decryption key lives in the URL fragment (`#...`) and is never sent to the server.

Source code, docs and issues: **https://github.com/zafrem/Vanish**

## Features

- **Zero-knowledge**: the server never sees plaintext or the decryption key
- **Client-side AES-256-GCM** via the Web Crypto API
- **Burn on read**: atomic read-and-delete, so a message can be opened exactly once
- **Truly ephemeral**: ciphertext lives in Redis memory only, with a TTL of 1 hour to 7 days (configurable)
- **Clipboard protection**: retrieved secrets go straight to the clipboard and are never rendered in the page
- **Optional enterprise integrations**: Okta SSO, HashiCorp Vault, Slack, SMTP email (all off by default)

## What's in this image

A single container that bundles:

| Component | Details |
|-----------|---------|
| Frontend | React SPA served by nginx on port `80` |
| Backend | Go (Gin) API on port `8080`, proxied by nginx under `/api` |
| Process manager | supervisord runs nginx and the backend |

**Not included:** Redis and PostgreSQL. The container needs both, so run them alongside it (see the Compose example below).

## Tags

| Tag | Description |
|-----|-------------|
| `latest` | Latest build of the `main` branch |
| `X.Y.Z`, `X.Y`, `X` | Release versions (from `vX.Y.Z` git tags) |
| `main` | Same source as `latest`, tagged by branch name |

For production, pin a version tag rather than `latest`.

## Quick start

The easiest way is Docker Compose with Redis and PostgreSQL:

```yaml
services:
  redis:
    image: redis:7-alpine
    # In-memory only: no snapshots, no append-only file
    command: redis-server --maxmemory 256mb --maxmemory-policy volatile-ttl --save "" --appendonly no

  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: vanish
      POSTGRES_PASSWORD: change-me
      POSTGRES_DB: vanish
    volumes:
      - postgres-data:/var/lib/postgresql/data

  vanish:
    image: zafrem/vanish:latest
    ports:
      - "3000:80"
    environment:
      REDIS_ADDRESS: redis:6379
      DB_HOST: postgres
      DB_USER: vanish
      DB_PASSWORD: change-me
      DB_NAME: vanish
      JWT_SECRET: replace-with-a-long-random-string
      BASE_URL: http://localhost:3000
      ALLOWED_ORIGINS: http://localhost:3000
    depends_on:
      - redis
      - postgres
    restart: unless-stopped

volumes:
  postgres-data:
```

```bash
docker compose up -d
```

Then open **http://localhost:3000**.

Run it standalone against existing Redis and PostgreSQL:

```bash
docker run -d --name vanish -p 3000:80 \
  -e REDIS_ADDRESS=redis.example.com:6379 \
  -e DB_HOST=postgres.example.com \
  -e DB_USER=vanish -e DB_PASSWORD=... -e DB_NAME=vanish \
  -e JWT_SECRET=... \
  -e BASE_URL=https://vanish.example.com \
  -e ALLOWED_ORIGINS=https://vanish.example.com \
  zafrem/vanish:latest
```

## Ports

| Port | Purpose |
|------|---------|
| `80` | Web UI and API (via nginx). This is the one to publish |
| `8080` | Backend API directly (optional) |

## Environment variables

### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDRESS` | `localhost:6379` | Redis host:port for encrypted messages |
| `REDIS_PASSWORD` | _(empty)_ | Redis password |
| `REDIS_DB` | `0` | Redis database index |
| `DB_HOST` / `DB_PORT` | `localhost` / `5432` | PostgreSQL host and port |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | `vanish` | PostgreSQL credentials and database |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `JWT_SECRET` | _(insecure placeholder)_ | **Set this.** Secret used to sign auth tokens |
| `JWT_DURATION` | `24` | Token lifetime in hours |
| `BASE_URL` | `http://localhost:5173` | Public URL used to build links (Slack/email) |
| `ALLOWED_ORIGINS` | `http://localhost:5173,http://localhost:3000` | Comma-separated CORS origins |
| `SERVER_PORT` / `SERVER_HOST` | `8080` / `0.0.0.0` | Backend bind address |

### Message TTL (seconds)

| Variable | Default |
|----------|---------|
| `DEFAULT_TTL` | `86400` (24 h) |
| `MIN_TTL` | `3600` (1 h) |
| `MAX_TTL` | `604800` (7 days) |

### Optional integrations (all disabled by default)

| Flag | Related variables |
|------|-------------------|
| `VAULT_ENABLED` | `VAULT_ADDR`, `VAULT_TOKEN`, `VAULT_NAMESPACE` |
| `OKTA_ENABLED` | `OKTA_DOMAIN`, `OKTA_CLIENT_ID`, `OKTA_CLIENT_SECRET`, `OKTA_REDIRECT_URL` |
| `SLACK_ENABLED` | `SLACK_BOT_TOKEN`, `SLACK_SIGNING_SECRET`, `SLACK_WEBHOOK_URL` |
| `EMAIL_ENABLED` | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `EMAIL_FROM_ADDRESS`, `EMAIL_FROM_NAME` |

On first run the backend creates a default admin account. Check the [documentation](https://github.com/zafrem/Vanish/tree/main/docs) for its credentials and change them immediately.

## Health check

The image includes a built-in `HEALTHCHECK` that requests `http://localhost/` every 30 seconds. The backend also exposes `GET /health` on port `8080`.

## Security notes

- Always set a strong, unique `JWT_SECRET` and change the default database password.
- Put the container behind HTTPS (reverse proxy or load balancer). Browsers require a secure context for the Web Crypto API on anything other than `localhost`.
- Keep Redis persistence **off** (`--save "" --appendonly no`) so ciphertext never touches disk.
- The server does not log request bodies by design. Don't add middleware that does.
- The Slack `/vanishPW` command encrypts on the server, so that path is not zero-knowledge. The web app, CLI and Chrome extension are.

## Integrations

You don't have to use the web UI. Every route below talks to the same container. They all create the same kind of link, `https://<host>/m/<id>#<key>`, which is destroyed after the first read.

| Integration | Where it runs | Zero-knowledge? | Server config needed |
|-------------|---------------|-----------------|----------------------|
| [Shareable links](#shareable-links) | Any browser | Yes | None |
| [CLI](#cli) | Your terminal or scripts | Yes | None |
| [Chrome extension](#chrome-extension) | Chrome / Edge | Yes | None |
| [Slack `/vanishPW`](#slack) | Slack | **No** (server-side encryption) | `SLACK_*` |
| [Email notifications](#email-notifications) | Recipient's inbox | Link only | `EMAIL_*` |
| [REST API](#rest-api) | Anything that speaks HTTP | Depends on your client | None |

The CLI and Chrome extension are not in this image. They are separate clients that point at your running container.

### Shareable links

The link is the integration. Paste it into any chat, ticket, or email. The key after `#` never reaches the server, and the link stops working after one read. `BASE_URL` must be your public address, or generated links will point to the wrong host.

### CLI

Source: `cli/` in the repository.

```bash
git clone https://github.com/zafrem/Vanish && cd Vanish/cli
make build && sudo mv vanish /usr/local/bin/

vanish config   # server URL (e.g. https://vanish.example.com) + your JWT
```

```bash
vanish send user@example.com "MyPassword123"               # inline
echo "API_KEY=secret" | vanish send user@example.com        # from stdin
vanish send user@example.com -ttl 3600 "temporary"          # custom expiry (seconds)
vanish send user@example.com -file ./credentials.pdf        # file, up to 5 MiB

vanish get "https://vanish.example.com/m/abc123#KEY"        # text to stdout, files saved
vanish get -o ./out.pdf "https://vanish.example.com/m/abc123#KEY"
```

Good fits: CI/CD jobs, onboarding scripts, and piping credentials without ever writing them to a chat. Config is stored in `~/.vanish/config.json` with `0600` permissions.

### Chrome extension

Source: `chrome-extension/`. It scans pages for keywords or regexes you define (passwords, API keys), highlights matches, and creates a Vanish link in one click. Encryption happens in the browser.

1. Open `chrome://extensions`, enable **Developer mode**, and choose **Load unpacked**. Select the `chrome-extension/` folder.
2. Log in to your Vanish web app once, so the extension can use your session.
3. Point the extension's options at your Vanish URL.

### Slack

Adds a `/vanishPW` slash command. The user fills in a private modal, and the recipient gets a DM with a one-time link, so the secret never appears in channel history.

1. **Expose Vanish over public HTTPS.** Slack must be able to reach `/api/slack/*`.
2. **Create a Slack app** at api.slack.com/apps:
   - Bot scopes: `commands`, `chat:write`, `users:read`, `users:read.email`, `im:write`
   - Slash command `/vanishPW`, Request URL `https://<your-domain>/api/slack/command`
   - Interactivity on, Request URL `https://<your-domain>/api/slack/interaction`
   - Install to your workspace, then copy the Bot token (`xoxb-...`) and the Signing Secret
3. **Configure the container:**
   ```yaml
   environment:
     SLACK_ENABLED: "true"
     SLACK_BOT_TOKEN: xoxb-...
     SLACK_SIGNING_SECRET: ...
     BASE_URL: https://<your-domain>
   ```
4. Restart the container, then run `/vanishPW` in any channel or DM.

Notes:
- The recipient's email must belong to a registered Vanish user.
- The Slack path encrypts **on the server**, so the server briefly sees the plaintext. Use the web app, CLI, or extension when you need strict zero-knowledge.

### Email notifications

Vanish can email the recipient a link when a message is sent.

```yaml
environment:
  EMAIL_ENABLED: "true"
  SMTP_HOST: smtp.example.com
  SMTP_PORT: "587"
  SMTP_USER: ...
  SMTP_PASSWORD: ...
  EMAIL_FROM_ADDRESS: noreply@example.com
  EMAIL_FROM_NAME: Vanish
```

### REST API

Authenticated with a Bearer JWT from `POST /api/auth/login`.

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/auth/login` | Get a JWT |
| `POST` | `/api/messages` | Store an encrypted message (ciphertext + IV) |
| `GET` | `/api/messages/:id` | Read and destroy a message |
| `HEAD` | `/api/messages/:id` | Check whether a message still exists, without reading it |
| `GET` | `/api/history` | Metadata of your sent messages |
| `POST` | `/api/notifications/send-email` | Email a link (needs `EMAIL_ENABLED`) |
| `POST` | `/api/notifications/send-slack` | DM a link (needs `SLACK_ENABLED`) |
| `GET` | `/health` | Liveness check |

The API only accepts **ciphertext**. To keep the zero-knowledge guarantee, your client must encrypt with AES-256-GCM (256-bit key, 96-bit IV, base64) and keep the key in the link's `#fragment`. See the [API reference](https://github.com/zafrem/Vanish/blob/main/docs/API_REFERENCE.md) for request bodies.

### Single sign-on

Optionally set `OKTA_ENABLED=true` (plus the `OKTA_*` variables) to log in with Okta, and `VAULT_ENABLED=true` to load secrets such as `JWT_SECRET` from HashiCorp Vault. See the [enterprise setup guide](https://github.com/zafrem/Vanish/blob/main/docs/ENTERPRISE_SETUP.md).

## Documentation

- [Architecture and security model](https://github.com/zafrem/Vanish/blob/main/docs/ARCHITECTURE.md)
- [Configuration](https://github.com/zafrem/Vanish/blob/main/docs/CONFIGURATION.md)
- [Deployment](https://github.com/zafrem/Vanish/blob/main/docs/DEPLOYMENT.md)
- [Enterprise setup (Okta, Vault, Slack, Email)](https://github.com/zafrem/Vanish/blob/main/docs/ENTERPRISE_SETUP.md)
- [API reference](https://github.com/zafrem/Vanish/blob/main/docs/API_REFERENCE.md)

## License

See [LICENSE](https://github.com/zafrem/Vanish/blob/main/LICENSE).

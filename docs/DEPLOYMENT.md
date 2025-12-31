# Deployment Guide

This guide will walk you through getting Vanish up and running. We cover three main scenarios to suit your needs:

1.  **Development Mode**: Perfect for local testing and experimentation.
2.  **Enterprise Mode**: Adds powerful integrations like Vault, Okta, and Slack.
3.  **Production Deployment**: Best practices for a secure, public-facing setup.

---

## 1. Development Mode

The quickest way to get started is by running the core services using Docker Compose. This includes PostgreSQL, Redis, the Backend API, and the Frontend.

### Start the Services
Run the following command to spin up the basic infrastructure:

```bash
docker-compose up -d
```

### Access the Application
Once the containers are running, you can access the app in your browser:

*   **URL**: [http://localhost:3000](http://localhost:3000)

**Tip:** Go ahead and create your first account by registering at [http://localhost:3000/register](http://localhost:3000/register).

---

## 2. Enterprise Mode (Vault + Okta + Slack)

For a more robust setup that mimics a corporate environment, you can enable enterprise integrations. This adds HashiCorp Vault for secrets management, Okta for SSO, and Slack for notifications.

### Step 1: Start All Services
First, ensure all containers, including Vault, are running:

```bash
docker-compose up -d
```

### Step 2: Configure Vault
You need to set up Vault to store your sensitive configuration.

**Access Vault:**
*   **UI**: [http://localhost:8200](http://localhost:8200)
*   **Token**: `dev-root-token`

**Configure via CLI:**
You can also configure Vault using the command line:

```bash
# Set environment variables for the Vault CLI
export VAULT_ADDR='http://localhost:8200'
export VAULT_TOKEN='dev-root-token'

# Store the JWT secret
vault kv put secret/vanish/jwt secret="your-jwt-secret-here"

# Store Okta configuration
vault kv put secret/vanish/okta \
  domain="your-domain.okta.com" \
  client_id="your-id" \
  client_secret="your-secret"
```

### Step 3: Configure Third-Party Services
*   **Okta**: Follow the [Okta SSO Configuration guide](ENTERPRISE_SETUP.md#okta-sso-configuration).
*   **Slack**: Follow the [Slack Bot Setup guide](ENTERPRISE_SETUP.md#slack-bot-setup).

### Step 4: Enable Integrations
Update your `docker-compose.yml` or create a `.env` file to toggle these features on.

```bash
# Enable Vault
VAULT_ENABLED=true

# Enable Okta
OKTA_ENABLED=true
OKTA_DOMAIN=your-domain.okta.com
# ... (See Okta dashboard for details)

# Enable Slack
SLACK_ENABLED=true
SLACK_BOT_TOKEN=xoxb-...
# ... (See Slack app settings)

# Enable Email Notifications
EMAIL_ENABLED=true
SMTP_HOST=smtp.gmail.com
# ... (Your SMTP credentials)
```

### Step 5: Restart Services
Apply your changes by restarting the containers:

```bash
docker-compose down
docker-compose up -d
```

---

## 3. Production Deployment

Deploying to production requires additional security measures. Here is a checklist and guide to ensure a secure environment.

### Prerequisites
*   A domain name with DNS configured.
*   An SSL certificate (we recommend Let's Encrypt).
*   A production Okta organization.
*   A production Slack workspace.
*   An SMTP server or email service (like SendGrid or AWS SES).

### Deployment Steps

#### 1. Set up a Reverse Proxy
Use Nginx or Caddy to handle SSL termination and route traffic. Here is an example Nginx configuration:

```nginx
server {
    listen 443 ssl http2;
    server_name vanish.yourcompany.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # Frontend
    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Backend API
    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

#### 2. Configure Vault for Production
Do not use the dev server in production.
*   Use a proper backend like **Consul** for High Availability (HA).
*   Start Vault with a production config: `vault server -config=/path/to/vault-config.hcl`
*   **Important**: Enable auto-unseal using AWS KMS or a similar service to ensure Vault recovers automatically after a restart.

#### 3. Secure Environment Variables
Update your environment settings for security:

```bash
# Production URLs
ALLOWED_ORIGINS=https://vanish.yourcompany.com
OKTA_REDIRECT_URL=https://vanish.yourcompany.com/auth/callback

# Generate a strong, random JWT secret
JWT_SECRET=$(openssl rand -base64 32)
```

#### 4. Database Backups
Set up automated backups for PostgreSQL. Note that you only need to back up metadata (users, audit logs), as the actual secrets are ephemeral and stored in Redis.

```bash
# Example cron job: Backup every day at 2 AM
0 2 * * * pg_dump vanish > backup-$(date +%Y%m%d).sql
```

#### 5. Monitoring
Keep an eye on your system health:
*   **Logs**: `docker-compose logs -f backend`
*   **Audit**: Check Vault audit logs regularly.
*   **Metrics**: Monitor Database CPU/Memory and Okta dashboard for login anomalies.

---

## Kubernetes Deployment

*Coming Soon:* We are working on Helm charts to simplify Kubernetes deployment. Planned features include:
*   StatefulSet for PostgreSQL.
*   Deployments for Backend/Frontend.
*   Vault Operator integration.
*   External Secrets Operator for managing Okta/Slack credentials.

---

## Security Checklist

Before going live, verify the following:

- [ ] **HTTPS** is enabled with a valid certificate.
- [ ] **Vault** is unsealed and has a backup strategy.
- [ ] **Okta MFA** is enforced for all users.
- [ ] **Database credentials** are rotated and managed via Vault.
- [ ] **Rate limiting** is configured to prevent abuse.
- [ ] **Monitoring and Alerting** are active.
- [ ] **Incident Response Plan** is documented and understood.

---

## Troubleshooting

### Issue: Backend won't start
**Check**:
1.  Run `docker-compose logs backend` to see the error.
2.  **Database**: Is PostgreSQL ready? The app might be timing out while waiting for it.
3.  **Vault**: Is Vault sealed? You must unseal it before the backend can retrieve secrets.
4.  **Config**: Are your Okta credentials correct?

### Issue: Vault is sealed
If Vault restarts, it seals itself. Unseal it using your unseal keys:

```bash
docker exec vanish-vault vault operator unseal <key1>
docker exec vanish-vault vault operator unseal <key2>
docker exec vanish-vault vault operator unseal <key3>
```

### Issue: Slack notifications not working
**Check**:
1.  Verify the token: `curl -H "Authorization: Bearer xoxb-your-token" https://slack.com/api/auth.test`
2.  Ensure the email address in Vanish matches the user's email in Slack.
3.  Check that the bot has the necessary OAuth scopes.

---

## Support

*   For detailed enterprise setup, see the [Enterprise Setup Guide](ENTERPRISE_SETUP.md).
*   For general questions, check the [README](../README.md).
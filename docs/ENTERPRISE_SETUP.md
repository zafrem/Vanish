# Enterprise Setup Guide

This guide will help you configure Vanish for a corporate environment. By enabling these integrations, you transform Vanish from a simple secure messaging tool into a fully managed enterprise solution.

We will cover four key integrations:
1.  **HashiCorp Vault**: For secure secrets management (the "brain" of the operation).
2.  **Okta SSO**: To manage user access and Single Sign-On.
3.  **Slack**: For real-time notifications when messages are sent.
4.  **Email**: For standard notifications via SMTP.

---

## 1. HashiCorp Vault Setup

Vanish uses HashiCorp Vault to securely store its own configuration (like API keys and credentials) and to manage database access. Think of it as the secure locker for the application's secrets.

### Step 1: Install and Start Vault

You can run Vault easily using Docker. If you are using our `docker-compose.yml`, it's already included.

```bash
# Standalone Docker run
docker run --cap-add=IPC_LOCK -d -p 8200:8200 --name=vault vault

# Or, if you are using the project's docker-compose
docker-compose up -d vault
```

### Step 2: Initialize and Unseal
When Vault starts for the first time, it is "sealed" and uninitialized. You need to unlock it.

```bash
# Initialize Vault
docker exec vault vault operator init
```

**⚠️ Important:** This command will output **Unseal Keys** and a **Root Token**. Save these securely! You cannot recover them if lost.

Now, use three of those keys to unseal the vault:

```bash
docker exec vault vault operator unseal <key1>
docker exec vault vault operator unseal <key2>
docker exec vault vault operator unseal <key3>
```

### Step 3: Configure Vault for Vanish
Now we need to tell Vault about the secrets Vanish needs. We'll enable the Key-Value (KV) store and add our secrets.

```bash
# Set up your environment for the CLI
export VAULT_ADDR='http://localhost:8200'
export VAULT_TOKEN='<root-token-from-init>'

# Enable the version 2 Key-Value secrets engine
vault secrets enable -path=secret kv-v2

# 1. Store the JWT signing secret (used for user sessions)
vault kv put secret/vanish/jwt secret="your-super-secret-jwt-key-here"

# 2. Store Okta credentials (if using Okta)
vault kv put secret/vanish/okta \
  domain="your-domain.okta.com" \
  client_id="okta-client-id" \
  client_secret="okta-client-secret"

# 3. Store Slack credentials (if using Slack)
vault kv put secret/vanish/slack \
  bot_token="xoxb-your-slack-bot-token" \
  signing_secret="slack-signing-secret"

# 4. Store SMTP credentials (if using Email)
vault kv put secret/vanish/smtp \
  host="smtp.gmail.com" \
  port="587" \
  user="noreply@yourcompany.com" \
  password="smtp-password"
```

### Step 4: Dynamic Database Credentials (Optional)
For advanced security, Vault can rotate database passwords automatically so the application never uses a static password.

```bash
# Enable the database engine
vault secrets enable database

# Configure the connection to PostgreSQL
vault write database/config/vanish-postgres \
    plugin_name=postgresql-database-plugin \
    allowed_roles="vanish-app" \
    connection_url="postgresql://{{username}}:{{password}}@postgres:5432/vanish?sslmode=disable" \
    username="vanish_admin" \
    password="admin_password"

# Create a role that generates temporary credentials
vault write database/roles/vanish-app \
    db_name=vanish-postgres \
    creation_statements="CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}'; \ 
        GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO \"{{name}}\";" \
    default_ttl="1h" \
    max_ttl="24h"
```

---

## 2. Okta SSO Configuration

Integrate with Okta to allow your employees to log in with their existing corporate accounts.

### Step 1: Create the App in Okta
1.  Log in to your **Okta Admin Dashboard**.
2.  Navigate to **Applications** > **Create App Integration**.
3.  Select **OIDC - OpenID Connect** as the sign-in method.
4.  Choose **Web Application** as the application type.

### Step 2: Configure Settings
Use the following settings to ensure Vanish can communicate with Okta:

*   **Application Name**: Vanish
*   **Grant Types**: Check `Authorization Code` and `Refresh Token`.
*   **Sign-in redirect URIs**:
    *   `http://localhost:3000/auth/callback` (Local dev)
    *   `https://vanish.yourcompany.com/auth/callback` (Production)
*   **Sign-out redirect URIs**:
    *   `http://localhost:3000`
    *   `https://vanish.yourcompany.com`

### Step 3: Assignments & Credentials
1.  Go to the **Assignments** tab and assign the users or groups who need access.
2.  On the **General** tab, note down your **Client ID** and **Client Secret**. You'll need these for the Vault configuration above.

---

## 3. Slack Bot Setup

Enable Slack notifications so users can send secret links directly to a colleague's DM.

### Step 1: Create the App
1.  Go to [api.slack.com/apps](https://api.slack.com/apps).
2.  Click **Create New App** > **From scratch**.
3.  Name it **Vanish Secret Messenger** (or similar) and select your workspace.

### Step 2: Permissions (Scopes)
In the sidebar, go to **OAuth & Permissions** and add the following **Bot Token Scopes**:
*   `chat:write` (To send messages)
*   `users:read` (To look up users)
*   `users:read.email` (To match Slack users with Vanish users)
*   `im:write` (To start direct messages)

### Step 3: Install & Keys
1.  Scroll up and click **Install to Workspace**.
2.  Copy the **Bot User OAuth Token** (starts with `xoxb-`).
3.  Go to **Basic Information** in the sidebar to find your **Signing Secret**.

**How it works:** When you send a secret to a user, Vanish looks up their email in Slack. If a match is found, the bot DMs them the link.

---

## 4. Email SMTP Configuration

If you prefer email notifications, you can configure any standard SMTP provider.

### Gmail Example
1.  Enable 2-Step Verification on your Google Account.
2.  Generate an **App Password** (Security > 2-Step Verification > App passwords).
3.  Use these settings:
    *   **Host**: `smtp.gmail.com`
    *   **Port**: `587`
    *   **User**: `your-email@gmail.com`
    *   **Password**: `your-app-password`

### AWS SES Example
*   **Host**: `email-smtp.us-east-1.amazonaws.com` (or your region)
*   **Port**: `587`
*   **User/Password**: Your SES SMTP credentials (not your IAM keys).

---

## Deployment Reference

When deploying, your `.env` file or environment variables should look something like this.

```bash
# Core Server Config
SERVER_PORT=8080
ALLOWED_ORIGINS=https://vanish.yourcompany.com

# Database & Storage
REDIS_ADDRESS=redis:6379
DB_HOST=postgres
DB_NAME=vanish
DB_USER=vanish

# Security
JWT_SECRET=change-me-in-production
# Or enable Vault to manage this:
VAULT_ENABLED=true
VAULT_ADDR=http://vault:8200
VAULT_TOKEN=your-vault-token

# SSO & Integrations
OKTA_ENABLED=true
OKTA_DOMAIN=your-domain.okta.com
# ... (Client ID/Secret stored in Vault recommended)

SLACK_ENABLED=true
# ... (Token stored in Vault recommended)

EMAIL_ENABLED=true
# ... (SMTP creds stored in Vault recommended)
```

## Security Best Practices for Production

*   **Vault**: Always use a production-grade backend (like Consul) and enable TLS. Never use the dev server in production.
*   **Secrets**: Never commit secrets to git. Use Vault or environment variables.
*   **Network**: Keep your database and Redis inside a private network, accessible only by the backend.
*   **Audit**: Regularly check Vault audit logs and Vanish access logs.

---

## Troubleshooting

*   **Vault Sealed?** If the backend fails to start, check if Vault is sealed (`vault status`). You must unseal it after every restart unless you configure auto-unseal.
*   **Okta Login Fails?** Double-check your Redirect URIs in the Okta dashboard. They must match exactly what the browser sends.
*   **No Slack Messages?** Ensure the user's email in Vanish matches their email in Slack exactly. The bot can't find them otherwise.

Need more help? Check the [README](../README.md) or the [Deployment Guide](DEPLOYMENT.md).
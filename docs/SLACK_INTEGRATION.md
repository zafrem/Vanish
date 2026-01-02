# Slack Integration - `/vanishPW` Command

This document explains how to set up and use the Vanish Slack integration for secure password sharing.

## Overview

The `/vanishPW` Slack slash command allows users to securely share passwords and secrets directly from Slack without the password ever appearing in Slack history or logs.

### Security Features

- **Ephemeral Modal**: Password input happens in a private Slack modal (not visible in channels)
- **Server-Side AES-256-GCM Encryption**: Messages are encrypted immediately upon submission
- **Zero Slack History**: Passwords never appear in Slack messages or logs
- **One-Time Access**: Recipients can only view the message once
- **Burn-on-Read**: Messages are automatically destroyed after reading
- **Time-Based Expiration**: Messages expire after a configurable time (1 hour to 7 days)
- **Recipient Verification**: Only the intended recipient can decrypt and view the message

## Setup Instructions

### 1. Create a Slack App

1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Click **"Create New App"** → **"From scratch"**
3. Enter app name: `Vanish` (or your preferred name)
4. Select your workspace
5. Click **"Create App"**

### 2. Configure OAuth Scopes

Navigate to **"OAuth & Permissions"** in the sidebar and add these Bot Token Scopes:

**Required Scopes:**
- `chat:write` - Send messages to users
- `chat:write.public` - Send messages to channels (optional)
- `users:read` - View people in the workspace
- `users:read.email` - View email addresses of people in the workspace
- `im:write` - Start direct messages with people
- `commands` - Add slash commands

### 3. Create Slash Command

1. Navigate to **"Slash Commands"** in the sidebar
2. Click **"Create New Command"**
3. Fill in the details:
   - **Command**: `/vanishPW`
   - **Request URL**: `https://your-vanish-domain.com/api/slack/command`
   - **Short Description**: `Send a secure, ephemeral password`
   - **Usage Hint**: `[no parameters needed]`
4. Click **"Save"**

### 4. Enable Interactivity

1. Navigate to **"Interactivity & Shortcuts"** in the sidebar
2. Toggle **"Interactivity"** to **On**
3. Set **"Request URL"**: `https://your-vanish-domain.com/api/slack/interaction`
4. Click **"Save Changes"**

### 5. Install App to Workspace

1. Navigate to **"OAuth & Permissions"**
2. Click **"Install to Workspace"**
3. Review permissions and click **"Allow"**
4. Copy the **"Bot User OAuth Token"** (starts with `xoxb-`)

### 6. Get Signing Secret

1. Navigate to **"Basic Information"**
2. Scroll to **"App Credentials"**
3. Copy the **"Signing Secret"**

### 7. Configure Environment Variables

Add these environment variables to your Vanish backend:

```bash
# Slack Configuration
SLACK_ENABLED=true
SLACK_BOT_TOKEN=xoxb-your-bot-token-here
SLACK_SIGNING_SECRET=your-signing-secret-here

# Base URL (used to generate message links)
BASE_URL=https://your-vanish-domain.com
```

### 8. Restart Vanish Backend

```bash
# Docker
docker-compose restart backend

# Or if running locally
cd backend && go run cmd/server/main.go
```

## Usage

### Sending a Secure Message

1. In any Slack channel or DM, type: `/vanishPW`
2. A private modal will appear with the following fields:
   - **Recipient Email**: The email address of the recipient (must be registered in Vanish)
   - **Secret Message**: The password or secret you want to share
   - **Expires In**: Choose when the message should expire (1 hour, 24 hours, 3 days, or 7 days)
3. Click **"Send"**
4. The recipient will receive a Slack DM with a secure link
5. You'll receive a confirmation message

### Viewing a Secure Message

1. Recipient receives a DM from the Vanish bot:
   ```
   🔒 New Secure Message from John Doe

   You have received a secure, ephemeral message.

   Click here to view (one-time access only):
   https://vanish.example.com/m/abc123#key

   ⚠️ This message will be permanently destroyed after you read it.
   ```
2. Click the link to view the message in your browser
3. Message is displayed once and immediately destroyed
4. Link becomes invalid after viewing

## Security Considerations

### What Gets Encrypted

- **Encrypted**: The password/secret message content
- **Encrypted**: Stored in Redis with AES-256-GCM encryption
- **Metadata Only**: Sender, recipient, timestamps (stored in PostgreSQL)

### What Slack Sees

- Slack **NEVER** sees the password content
- Slash command invocation (command name only, no parameters)
- Modal interaction (Slack knows a modal was opened, but content is encrypted before transmission completes)
- DM delivery confirmation

### Server-Side Encryption

Unlike the web frontend (which uses client-side encryption), the Slack integration uses **server-side encryption**:

- Password is encrypted immediately upon modal submission
- Uses the same AES-256-GCM algorithm as client-side encryption
- Encryption happens in memory before storage
- Plaintext is never written to disk or logs
- Encryption key is generated server-side and stored in the database for recipient access

### Trust Model

For Slack integration, you are trusting:
1. **Your Vanish server**: Briefly sees plaintext during encryption
2. **Slack infrastructure**: For secure modal transmission (HTTPS)
3. **Recipient authentication**: Vanish verifies recipient identity via database

If you require **zero-knowledge** security (server never sees plaintext), use the web frontend instead.

## Troubleshooting

### "Slack integration disabled" error

- Ensure `SLACK_ENABLED=true` is set
- Verify `SLACK_BOT_TOKEN` and `SLACK_SIGNING_SECRET` are configured
- Restart the backend after changing environment variables

### "Recipient not found" error

- Recipient must be registered in Vanish first
- Email address in Slack must match email in Vanish database
- Ask recipient to register at your Vanish web URL

### "Failed to open modal" error

- Check that Request URL in Slack app settings is correct
- Ensure your Vanish backend is publicly accessible
- Verify firewall/network allows Slack to reach your server

### "Invalid signature" error

- Verify `SLACK_SIGNING_SECRET` matches the one in Slack app settings
- Check system clock - time drift can cause signature verification failures
- Ensure backend is receiving requests from Slack (not being proxied/modified)

## API Endpoints

The Slack integration adds these endpoints to the Vanish API:

- `POST /api/slack/command` - Handles `/vanishPW` slash command
- `POST /api/slack/interaction` - Handles modal submissions and interactions

Both endpoints:
- Are publicly accessible (authentication via Slack signature)
- Verify requests using HMAC-SHA256 signature
- Reject requests older than 5 minutes (replay attack protection)

## Architecture

```
┌─────────┐           ┌──────────┐          ┌────────────┐
│  User   │─/vanishPW→│  Slack   │─webhook→│   Vanish   │
│         │           │          │          │   Backend  │
└─────────┘           └──────────┘          └────────────┘
                           │                      │
                           │ Opens Modal          │ Verifies Signature
                           │←─────────────────────│
                           │                      │
                      ┌────▼────┐                 │
                      │  Modal  │                 │
                      │  Form   │                 │
                      └────┬────┘                 │
                           │ Submit               │
                           │──────────────────────▶
                           │                      │
                           │                ┌─────▼──────┐
                           │                │  Encrypt   │
                           │                │  Message   │
                           │                └─────┬──────┘
                           │                      │
                           │                ┌─────▼──────┐
                           │                │   Store    │
                           │                │   Redis    │
                           │                └─────┬──────┘
                           │                      │
                      ┌────▼────┐                 │
                      │   DM    │◀────────────────┘
                      │Recipient│   Send Notification
                      └─────────┘
```

## Future Enhancements

Potential improvements for future versions:

- [ ] User search dropdown in modal (instead of typing email)
- [ ] Message status tracking ("delivered", "read")
- [ ] Slack Block Kit buttons for clipboard copy
- [ ] Support for file attachments
- [ ] Group message sharing (multiple recipients)
- [ ] Custom expiration times
- [ ] Read receipts and notifications

## Support

For issues or questions:
- GitHub Issues: https://github.com/milkiss/vanish/issues
- Documentation: https://github.com/milkiss/vanish/docs

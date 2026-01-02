# OTP Integration Architecture Design

## Overview

Vanish's burn-on-read architecture makes it ideal for One-Time Password (OTP) systems. This document outlines architectural approaches for integrating OTP generation, delivery, and verification with other platforms.

## Why Vanish is Perfect for OTP

### Current Strengths
- ✅ **Burn-on-read**: Messages self-destruct after one use
- ✅ **Time-based expiration**: Built-in TTL (configurable 1 hour - 7 days)
- ✅ **Multi-channel delivery**: Slack, Email (SMTP), Web
- ✅ **Recipient verification**: Access control ensures only intended user receives OTP
- ✅ **Encryption**: AES-256-GCM protects OTPs in transit and at rest
- ✅ **Atomic operations**: Race-condition-free read-and-delete
- ✅ **RESTful API**: Easy integration with other platforms

### OTP-Specific Enhancements Needed
- 🔨 Shorter TTL support (1-15 minutes for OTPs)
- 🔨 Standardized OTP generation (TOTP, HOTP, numeric codes)
- 🔨 Verification endpoint for third-party platforms
- 🔨 Webhook delivery channel
- 🔨 SMS delivery channel (via Twilio/AWS SNS)
- 🔨 Rate limiting per user/application
- 🔨 Attempt tracking (max 3 failed attempts, etc.)

---

## Proposed Architectures

### **Option 1: API-First OTP Service**

Transform Vanish into a general-purpose OTP platform that other applications can use via API.

#### Architecture

```
┌──────────────┐
│ Your App     │
│ (Login page) │
└──────┬───────┘
       │
       │ POST /api/otp/generate
       │ {user_email, delivery_channel, ttl}
       │
┌──────▼───────────┐
│  Vanish API      │
│  OTP Service     │
└──────┬───────────┘
       │
       ├─────────────┐
       │             │
┌──────▼──┐    ┌────▼─────┐
│  Redis  │    │ Delivery │
│  Store  │    │ Channels │
│  OTP    │    └────┬─────┘
└─────────┘         │
                    ├─ Slack DM
                    ├─ Email (SMTP)
                    ├─ SMS (Twilio)
                    └─ Webhook

User receives OTP → Enters in your app

Your app validates:
POST /api/otp/verify
{otp_id, code}
→ Returns: {valid: true/false}
```

#### API Endpoints

**Generate OTP**
```http
POST /api/otp/generate
Authorization: Bearer <api_key>
Content-Type: application/json

{
  "recipient_email": "user@example.com",
  "delivery_channel": "slack|email|sms|webhook",
  "purpose": "login|password_reset|2fa",
  "ttl_seconds": 300,  // 5 minutes
  "code_length": 6,
  "code_type": "numeric|alphanumeric|totp"
}

Response:
{
  "otp_id": "uuid",
  "expires_at": "2025-12-31T10:05:00Z",
  "delivery_status": "sent|pending|failed"
}
```

**Verify OTP**
```http
POST /api/otp/verify
Authorization: Bearer <api_key>
Content-Type: application/json

{
  "otp_id": "uuid",
  "code": "123456"
}

Response:
{
  "valid": true,
  "consumed": true,
  "attempts_remaining": 0
}
```

**Check OTP Status**
```http
GET /api/otp/{otp_id}/status
Authorization: Bearer <api_key>

Response:
{
  "status": "pending|consumed|expired",
  "expires_at": "2025-12-31T10:05:00Z",
  "attempts_used": 1,
  "attempts_remaining": 2
}
```

#### Use Cases

1. **2FA/MFA for Your App**
   - User logs in with username/password
   - App requests OTP via Vanish API
   - User receives OTP via Slack/Email/SMS
   - User enters OTP in app
   - App verifies OTP via Vanish API

2. **Password Reset Flows**
   - User clicks "Forgot Password"
   - App generates OTP via Vanish
   - User receives reset code
   - User enters code + new password
   - App verifies OTP before allowing reset

3. **Transaction Confirmation**
   - User initiates sensitive action (wire transfer, data deletion)
   - App sends OTP for confirmation
   - User must enter OTP to proceed

---

### **Option 2: TOTP/HOTP Provider**

Implement standard TOTP (Time-based) or HOTP (Counter-based) OTP compatible with authenticator apps.

#### Architecture

```
┌──────────────┐
│  Your App    │
└──────┬───────┘
       │
       │ POST /api/totp/setup
       │ {user_id}
       │
┌──────▼───────────┐
│  Vanish TOTP     │
│  Generator       │
└──────┬───────────┘
       │
       │ Returns QR Code + Secret
       │
┌──────▼───────┐
│ User scans   │
│ QR with      │
│ Google Auth  │
└──────────────┘

Later: User enters TOTP code from app

POST /api/totp/verify
{user_id, code}
→ Vanish validates against stored secret
```

#### Features

- **Standard TOTP/HOTP**: Compatible with Google Authenticator, Authy, 1Password
- **QR Code Generation**: Easy enrollment
- **Backup Codes**: Generate single-use recovery codes
- **Secret Storage**: Encrypted TOTP secrets in PostgreSQL

#### API Endpoints

**Setup TOTP**
```http
POST /api/totp/setup
{
  "user_id": "uuid",
  "issuer": "YourApp",
  "account_name": "user@example.com"
}

Response:
{
  "secret": "BASE32ENCODEDSECRET",
  "qr_code_url": "data:image/png;base64,...",
  "backup_codes": ["12345678", "87654321", ...]
}
```

**Verify TOTP Code**
```http
POST /api/totp/verify
{
  "user_id": "uuid",
  "code": "123456"
}

Response:
{
  "valid": true
}
```

---

### **Option 3: Webhook/Integration Framework**

Create a plugin system where other platforms can register webhooks to receive OTPs.

#### Architecture

```
┌─────────────────────┐
│  Platform Registry  │
│  (Your integrations)│
└──────────┬──────────┘
           │
           │ POST /api/integrations/register
           │ {name, webhook_url, auth_token}
           │
┌──────────▼────────────┐
│  Vanish Integration   │
│  Manager              │
└──────────┬────────────┘
           │
When OTP generated:
           │
           ├─► Platform A Webhook
           ├─► Platform B Webhook
           └─► Platform C Webhook

Each webhook receives:
{
  "otp_id": "uuid",
  "code": "123456",
  "expires_at": "...",
  "metadata": {...}
}
```

#### Integration Types

1. **Webhook Delivery**
   - Platform registers webhook URL
   - Vanish posts OTP to webhook when generated
   - Platform handles delivery to end user

2. **API Key Authentication**
   - Platform gets API key from Vanish
   - Can generate/verify OTPs via API
   - Rate limited per API key

3. **OAuth 2.0 Integration**
   - Platforms authenticate via OAuth
   - Scoped permissions (generate_otp, verify_otp, etc.)

---

### **Option 4: Microservice OTP Gateway**

Create a dedicated OTP microservice that sits alongside Vanish.

#### Architecture

```
┌──────────────┐
│  Your Apps   │
└──────┬───────┘
       │
       │ gRPC / REST
       │
┌──────▼────────────┐         ┌──────────────┐
│  OTP Gateway      │────────►│  Vanish API  │
│  (New Service)    │         │  (Storage)   │
└──────┬────────────┘         └──────────────┘
       │
       │
┌──────▼────────────┐
│  Delivery Plugins │
│  - Slack          │
│  - Email          │
│  - SMS (Twilio)   │
│  - WhatsApp       │
│  - Webhook        │
└───────────────────┘
```

#### Benefits

- **Separation of Concerns**: OTP logic separate from message storage
- **Scalability**: Scale OTP generation independently
- **Plugin Architecture**: Easy to add new delivery channels
- **Multiple Backends**: Could use Vanish, Redis, or database

---

## Recommended Implementation: Hybrid Approach

I recommend a **phased implementation** combining the best aspects:

### **Phase 1: OTP API Extension (Quick Win)**

Extend current Vanish API with OTP-specific endpoints:

```go
// New model: OTP
type OTP struct {
    ID              string
    Code            string  // The actual OTP code
    RecipientEmail  string
    DeliveryChannel string  // slack, email, sms
    Purpose         string  // login, 2fa, password_reset
    AttemptsLeft    int     // Default: 3
    ExpiresAt       time.Time
    ConsumedAt      *time.Time
}

// Endpoints
POST   /api/otp/generate     // Generate and send OTP
POST   /api/otp/verify       // Verify OTP code
GET    /api/otp/:id/status   // Check status
DELETE /api/otp/:id          // Invalidate OTP
```

**What to build:**
1. OTP generation service (numeric codes, configurable length)
2. Verification endpoint with attempt tracking
3. Short TTL support (1-15 minutes)
4. Rate limiting per recipient
5. Delivery via existing channels (Slack, Email)

**Storage:**
- Store in Redis with short TTL (reuse existing storage)
- Metadata in PostgreSQL (attempts, consumption status)

### **Phase 2: Multi-Platform Integration**

Add integration framework for external platforms:

```go
// Platform API Keys
type APIKey struct {
    Key         string
    PlatformName string
    Permissions  []string  // generate_otp, verify_otp
    RateLimit    int       // Requests per minute
}

// Webhook Integrations
type WebhookIntegration struct {
    PlatformName string
    WebhookURL   string
    AuthToken    string
    Events       []string  // otp_generated, otp_verified
}
```

**What to build:**
1. API key management system
2. Webhook delivery system
3. OAuth 2.0 provider (optional)
4. Rate limiting per API key
5. Audit logging for external requests

### **Phase 3: TOTP/Standard OTP Support**

Add industry-standard OTP algorithms:

```go
// TOTP Secret
type TOTPSecret struct {
    UserID    string
    Secret    string  // Encrypted
    Algorithm string  // SHA1, SHA256
    Digits    int     // 6 or 8
    Period    int     // 30 seconds
}
```

**What to build:**
1. TOTP/HOTP generator (using standard libraries)
2. QR code generation for enrollment
3. Backup code generation
4. Compatible with Google Authenticator, Authy, etc.

### **Phase 4: Advanced Delivery Channels**

Expand beyond Slack/Email:

1. **SMS via Twilio**
   ```go
   type TwilioConfig struct {
       AccountSID string
       AuthToken  string
       FromNumber string
   }
   ```

2. **WhatsApp Business API**

3. **Push Notifications** (Firebase Cloud Messaging)

4. **Voice Calls** (automated OTP readout)

---

## Data Model Extensions

### OTP Table (PostgreSQL)

```sql
CREATE TABLE otps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(20) NOT NULL,
    code_hash VARCHAR(255) NOT NULL,  -- Bcrypt hash of code
    recipient_id UUID REFERENCES users(id),
    purpose VARCHAR(50) NOT NULL,
    delivery_channel VARCHAR(50) NOT NULL,
    max_attempts INTEGER DEFAULT 3,
    attempts_used INTEGER DEFAULT 0,
    consumed BOOLEAN DEFAULT FALSE,
    consumed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    metadata JSONB  -- Flexible storage for platform-specific data
);

CREATE INDEX idx_otps_code_hash ON otps(code_hash);
CREATE INDEX idx_otps_expires_at ON otps(expires_at);
```

### API Keys Table

```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(255) NOT NULL,
    platform_name VARCHAR(255) NOT NULL,
    permissions TEXT[] DEFAULT '{}',
    rate_limit_per_minute INTEGER DEFAULT 60,
    created_at TIMESTAMP DEFAULT NOW(),
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    active BOOLEAN DEFAULT TRUE
);
```

### Webhook Integrations Table

```sql
CREATE TABLE webhook_integrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform_name VARCHAR(255) NOT NULL,
    webhook_url TEXT NOT NULL,
    auth_token_hash VARCHAR(255),
    events TEXT[] DEFAULT '{}',
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW()
);
```

---

## Security Considerations

### OTP Storage
- **Hash OTP codes**: Store bcrypt hash, not plaintext
- **Encrypt sensitive data**: Encrypt TOTP secrets at rest
- **Short TTL**: OTPs expire in 5-15 minutes (not hours/days)

### Rate Limiting
- **Per user**: Max 5 OTP requests per 15 minutes
- **Per API key**: Configurable rate limits
- **Per IP**: Global rate limit to prevent abuse

### Attempt Tracking
- **Max attempts**: 3 failed verification attempts
- **Lockout**: Temporary lockout after max attempts
- **Invalidation**: Auto-invalidate after max attempts

### Delivery Security
- **Channel verification**: Verify email/phone before sending OTP
- **No OTP in logs**: Never log OTP codes
- **Secure channels**: HTTPS for webhooks, TLS for email/SMS

---

## Integration Examples

### Example 1: Login with OTP (Your App)

```javascript
// Step 1: User enters email/username
const response = await fetch('https://vanish.example.com/api/otp/generate', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer YOUR_API_KEY',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    recipient_email: 'user@example.com',
    delivery_channel: 'slack',
    purpose: 'login',
    ttl_seconds: 300,  // 5 minutes
    code_length: 6
  })
});

const { otp_id } = await response.json();

// Step 2: Show OTP input form
// User receives OTP via Slack, enters it

// Step 3: Verify OTP
const verifyResponse = await fetch('https://vanish.example.com/api/otp/verify', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer YOUR_API_KEY',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    otp_id: otp_id,
    code: userInputCode
  })
});

const { valid } = await verifyResponse.json();

if (valid) {
  // Grant access, create session
} else {
  // Show error, allow retry
}
```

### Example 2: TOTP Setup (2FA Enrollment)

```javascript
// Step 1: Setup TOTP for user
const setupResponse = await fetch('https://vanish.example.com/api/totp/setup', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer USER_SESSION_TOKEN',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    user_id: currentUserId,
    issuer: 'MyApp',
    account_name: currentUserEmail
  })
});

const { qr_code_url, backup_codes } = await setupResponse.json();

// Step 2: Display QR code for user to scan
displayQRCode(qr_code_url);

// Step 3: User scans with Google Authenticator

// Step 4: Verify setup with test code
const code = prompt('Enter code from authenticator app');

const verifyResponse = await fetch('https://vanish.example.com/api/totp/verify', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer USER_SESSION_TOKEN',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    user_id: currentUserId,
    code: code
  })
});

if (verifyResponse.json().valid) {
  // TOTP enrollment complete
  displayBackupCodes(backup_codes);
}
```

### Example 3: Webhook Integration (Third-Party Platform)

```javascript
// Platform registers webhook
await fetch('https://vanish.example.com/api/integrations/register', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer PLATFORM_API_KEY',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    platform_name: 'MyPlatform',
    webhook_url: 'https://myplatform.com/webhooks/otp',
    events: ['otp_generated', 'otp_verified']
  })
});

// When Vanish generates OTP, it POSTs to your webhook:
// POST https://myplatform.com/webhooks/otp
// {
//   "event": "otp_generated",
//   "otp_id": "uuid",
//   "code": "123456",
//   "recipient_email": "user@example.com",
//   "expires_at": "2025-12-31T10:05:00Z",
//   "metadata": {...}
// }

// Your platform can then:
// - Deliver OTP via custom channel
// - Log the event
// - Send confirmation to user
```

---

## Questions for You

Before I start implementing, I need to understand your use case better:

### **1. Primary Use Case**
What's your main goal?
- **A)** Add OTP/2FA to Vanish itself (user login)
- **B)** Build OTP service for YOUR other applications to use
- **C)** Offer OTP-as-a-Service to external developers (SaaS)
- **D)** Something else?

### **2. Delivery Channels**
Which channels do you need?
- Slack (already have)
- Email (already have)
- SMS (need Twilio integration)
- WhatsApp
- Webhook (for custom delivery)
- Push notifications

### **3. OTP Type**
What kind of OTPs?
- **Simple numeric codes** (123456) - easiest
- **TOTP** (Google Authenticator compatible) - standard
- **HOTP** (counter-based)
- **Alphanumeric codes** (A3X9K2)

### **4. Integration Complexity**
How will other platforms integrate?
- **API Keys** (simple, like Stripe API)
- **OAuth 2.0** (more complex, better security)
- **Webhooks** (push model)
- **All of the above**

### **5. Scale Expectations**
How many OTPs do you expect to generate?
- Personal use (< 100/day)
- Small business (< 10,000/day)
- Enterprise (> 100,000/day)

---

## Next Steps

Based on your answers, I can:

1. **Design the specific architecture** that fits your needs
2. **Implement Phase 1** (OTP API extension) immediately
3. **Create integration SDKs** (JavaScript, Python, Go client libraries)
4. **Build example integrations** (demo apps showing how to use it)
5. **Write comprehensive docs** for external developers

Let me know which direction resonates with you!

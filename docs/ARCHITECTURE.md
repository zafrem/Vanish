# Vanish Architecture

## Overview

Vanish implements a zero-knowledge, burn-on-read messaging system with client-side encryption and ephemeral storage.

## Security Model

### Client-Side Security

**Encryption**
- Algorithm: AES-256-GCM
- Key Size: 256 bits (32 bytes)
- IV Size: 96 bits (12 bytes) - recommended size for GCM
- Key Management: Encryption key stays in URL fragment (#), never sent to server

**Key Generation**
```javascript
// Generate 256-bit key
const key = await crypto.subtle.generateKey(
  { name: 'AES-GCM', length: 256 },
  true,
  ['encrypt', 'decrypt']
);

// Export as raw bytes and encode to base64
const keyBytes = await crypto.subtle.exportKey('raw', key);
const keyBase64 = btoa(String.fromCharCode(...new Uint8Array(keyBytes)));
```

**No Visual Exposure**
- Sensitive data never rendered to DOM
- Direct clipboard write via Clipboard API
- Best-effort memory cleanup after use

### Server-Side Security

**No Logging**
- Request bodies never logged (NFR-02)
- Only metadata logged (timestamp, IP, message ID)
- No sensitive data in logs

**Volatile Storage**
- Redis configured with no RDB/AOF persistence
- `save ""`: Disable RDB snapshots
- `appendonly no`: Disable AOF persistence
- `maxmemory-policy volatile-ttl`: Evict expired keys first

**Atomic Operations**
- Lua scripts ensure messages can only be read once
- No race conditions between check and delete
- WATCH/MULTI/EXEC transactions where needed

**Security Headers**
- HSTS (HTTP Strict Transport Security)
- CSP (Content Security Policy)
- X-Frame-Options: DENY
- X-Content-Type-Options: nosniff
- X-XSS-Protection: 1; mode=block

## System Architecture

### Components

```
┌─────────────┐
│   Browser   │
│  (React)    │
└──────┬──────┘
       │ HTTPS (TLS)
       │
┌──────▼──────┐
│   nginx     │  SSL Termination
│  (Reverse   │  Static Files
│   Proxy)    │
└──────┬──────┘
       │
┌──────▼──────┐
│   Go API    │  Message handling
│   Server    │  JWT auth
│   (Gin)     │
└──┬───┬───┬──┘
   │   │   │
   │   │   └────────┐
   │   │            │
┌──▼───▼──┐    ┌────▼─────┐
│  Redis  │    │PostgreSQL│
│Messages │    │ Metadata │
└─────────┘    └──────────┘
```

### Data Flow

**Creating a Message**
1. User enters secret in browser
2. Client generates random 256-bit AES-GCM key
3. Client encrypts secret with key
4. Client sends ciphertext + IV to server (key stays local)
5. Server stores in Redis with TTL
6. Server returns message ID
7. Client constructs URL with key in fragment: `https://vanish.local/m/{id}#{key}`

**Reading a Message**
1. User opens URL (key in # fragment never sent to server)
2. Client extracts message ID and key from URL
3. Client requests ciphertext from server
4. Server retrieves and atomically deletes message from Redis
5. Client decrypts ciphertext with key from URL
6. Client writes plaintext directly to clipboard (never to DOM)
7. Client attempts to clear sensitive data from memory

## Storage Architecture

### Redis Schema

**Message Storage**
```
Key: message:{uuid}
Value: JSON {
  "ciphertext": "base64-encoded-data",
  "iv": "base64-encoded-iv",
  "createdAt": "2025-12-31T10:00:00Z"
}
TTL: User-specified (1 hour to 7 days)
```

**Atomic Read-and-Delete**
```lua
-- Lua script executed atomically
local message = redis.call('GET', KEYS[1])
if message then
  redis.call('DEL', KEYS[1])
  return message
else
  return nil
end
```

### PostgreSQL Schema

**Users Table**
```sql
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username VARCHAR(255) UNIQUE NOT NULL,
  email VARCHAR(255) UNIQUE NOT NULL,
  password_hash VARCHAR(255),
  okta_id VARCHAR(255),
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

**Message Metadata Table**
```sql
CREATE TABLE message_metadata (
  id UUID PRIMARY KEY,
  sender_id UUID REFERENCES users(id),
  recipient_id UUID REFERENCES users(id),
  created_at TIMESTAMP DEFAULT NOW(),
  expires_at TIMESTAMP NOT NULL,
  burned BOOLEAN DEFAULT FALSE,
  burned_at TIMESTAMP,
  notification_sent BOOLEAN DEFAULT FALSE
);
```

## Security Considerations

### Threat Model

**Protected Against**
- Server compromise (zero-knowledge design)
- Network eavesdropping (HTTPS + client-side encryption)
- Message replay (burn-on-read)
- Brute force (cryptographically random UUIDs + keys)

**NOT Protected Against**
- Compromised recipient device
- Screenshots/screen recording
- Recipient copying secret after viewing
- Clipboard history managers
- Browser extensions with clipboard access
- Social engineering

### Known Limitations

1. **Browser Compatibility**: Requires Web Crypto API support
2. **Memory Cleanup**: JavaScript memory clearing is best-effort
3. **Clipboard Security**: Cannot prevent clipboard managers from caching
4. **HTTPS Requirement**: Clipboard API requires secure context

### Production Requirements

1. **HTTPS Only**: Clipboard API requires HTTPS
2. **Rate Limiting**: Implement at reverse proxy level
3. **Monitoring**: Track Redis memory, request rates, error rates
4. **Audit Logs**: Log metadata only (never request bodies)
5. **Key Rotation**: Regular JWT secret rotation
6. **Security Updates**: Keep dependencies updated

## Performance Considerations

**Redis Sizing**
- Average message size: ~1-5 KB (encrypted)
- 256 MB Redis = ~50,000-250,000 messages
- Adjust based on expected load and TTL

**Connection Pooling**
- Go: Use `redis.NewClient()` with default pool settings
- PostgreSQL: GORM manages connection pool

**Response Times**
- Message create: <50ms (single Redis write)
- Message read: <50ms (single Redis read+delete)
- Metadata queries: <100ms (PostgreSQL indexed lookups)

## Scalability

**Horizontal Scaling**
- Stateless API servers (scale with load balancer)
- Redis: Single instance for atomicity
- PostgreSQL: Read replicas for metadata queries

**Vertical Scaling**
- Increase Redis memory for more messages
- Increase CPU for more concurrent requests

**Limits**
- Max message size: 10 MB (configurable)
- Max TTL: 7 days (configurable)
- Min TTL: 1 hour (configurable)

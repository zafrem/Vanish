# Vanish API Reference

Complete API documentation for all Vanish endpoints.

**Base URL**: `http://localhost:8080` (development)
**Authentication**: Bearer token in `Authorization` header

---

## Table of Contents

1. [Public Endpoints](#public-endpoints)
2. [Authentication Endpoints](#authentication-endpoints)
3. [User Endpoints](#user-endpoints)
4. [Message Endpoints](#message-endpoints)
5. [History Endpoints](#history-endpoints)
6. [Profile Management](#profile-management)
7. [Admin Endpoints](#admin-endpoints)

---

## Public Endpoints

### Health Check
Check if the API is running.

```http
GET /health
```

**Response 200**:
```json
{
  "status": "healthy"
}
```

---

## Authentication Endpoints

All authentication endpoints are public (no token required).

### Register User
Create a new user account.

```http
POST /api/auth/register
Content-Type: application/json
```

**Request Body**:
```json
{
  "email": "user@example.com",
  "name": "John Doe",
  "password": "securepassword123"
}
```

**Response 201**:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "email": "user@example.com",
    "name": "John Doe",
    "is_admin": false
  }
}
```

**Response 409** (Email exists):
```json
{
  "error": "User with this email already exists"
}
```

---

### Login
Authenticate and get a JWT token.

```http
POST /api/auth/login
Content-Type: application/json
```

**Request Body**:
```json
{
  "email": "user@example.com",
  "password": "securepassword123"
}
```

**Response 200**:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "email": "user@example.com",
    "name": "John Doe",
    "is_admin": false
  }
}
```

**Response 401** (Invalid credentials):
```json
{
  "error": "invalid email or password"
}
```

---

## User Endpoints

All user endpoints require authentication.

### Get Current User
Get information about the currently logged-in user.

```http
GET /api/auth/me
Authorization: Bearer {token}
```

**Response 200**:
```json
{
  "id": 1,
  "email": "user@example.com",
  "name": "John Doe",
  "is_admin": false
}
```

---

### List All Users
Get list of all users (for recipient selection).

```http
GET /api/users
Authorization: Bearer {token}
```

**Response 200**:
```json
[
  {
    "id": 1,
    "email": "user@example.com",
    "name": "John Doe",
    "is_admin": false
  },
  {
    "id": 2,
    "email": "admin@vanish.local",
    "name": "Admin",
    "is_admin": true
  }
]
```

---

## Message Endpoints

All message endpoints require authentication.

### Create Message
Create an encrypted message for a specific recipient.

```http
POST /api/messages
Authorization: Bearer {token}
Content-Type: application/json
```

**Request Body**:
```json
{
  "ciphertext": "base64-encoded-encrypted-data",
  "iv": "base64-encoded-initialization-vector",
  "encryption_key": "client-side-encryption-key",
  "recipient_id": 2,
  "ttl": 86400
}
```

**Response 201**:
```json
{
  "id": "message-id-here",
  "expires_at": "2025-12-31T10:00:00Z"
}
```

**Response 400** (Validation error):
```json
{
  "error": "Invalid request: ..."
}
```

---

### Get Message
Retrieve and burn a message (one-time read).

```http
GET /api/messages/:id
Authorization: Bearer {token}
```

**Response 200**:
```json
{
  "ciphertext": "base64-encoded-encrypted-data",
  "iv": "base64-encoded-initialization-vector"
}
```

**Response 403** (Not recipient):
```json
{
  "error": "You are not the intended recipient of this message"
}
```

**Response 404** (Already read or not found):
```json
{
  "error": "Message has already been read and burned"
}
```

---

### Check Message Exists
Check if a message exists without burning it.

```http
HEAD /api/messages/:id
Authorization: Bearer {token}
```

**Response 200**: Message exists
**Response 404**: Message not found or already burned

---

## History Endpoints

### Get Message History
Get current user's message history (sent and received).

```http
GET /api/history?limit=50
Authorization: Bearer {token}
```

**Query Parameters**:
- `limit` (optional, default: 50): Maximum number of messages to return

**Response 200**:
```json
[
  {
    "message_id": "abc123",
    "sender_name": "John Doe",
    "recipient_name": "Jane Smith",
    "status": "pending",
    "created_at": "2025-12-30T10:00:00Z",
    "expires_at": "2025-12-31T10:00:00Z",
    "is_sender": true,
    "is_recipient": false,
    "encryption_key": "key-here-if-recipient-and-pending"
  }
]
```

**Status values**: `pending`, `read`, `expired`

---

## Profile Management

Endpoints for users to manage their own profile.

### Update Profile
Update your name or email.

```http
PUT /api/profile
Authorization: Bearer {token}
Content-Type: application/json
```

**Request Body** (all fields optional):
```json
{
  "email": "newemail@example.com",
  "name": "New Name"
}
```

**Response 200**:
```json
{
  "id": 1,
  "email": "newemail@example.com",
  "name": "New Name",
  "is_admin": false
}
```

---

### Change Password
Change your password.

```http
POST /api/profile/password
Authorization: Bearer {token}
Content-Type: application/json
```

**Request Body**:
```json
{
  "current_password": "oldpassword",
  "new_password": "newpassword123"
}
```

**Response 200**:
```json
{
  "message": "Password updated successfully"
}
```

**Response 401** (Wrong current password):
```json
{
  "error": "Current password is incorrect"
}
```

---

### Delete Account
Delete your own account (requires password confirmation).

```http
DELETE /api/profile
Authorization: Bearer {token}
Content-Type: application/json
```

**Request Body**:
```json
{
  "password": "yourpassword"
}
```

**Response 200**:
```json
{
  "message": "Account deleted successfully"
}
```

**Response 401** (Wrong password):
```json
{
  "error": "Password is incorrect"
}
```

---

## Admin Endpoints

All admin endpoints require authentication + admin role.

### Get System Statistics
Get statistics about users and messages.

```http
GET /api/admin/statistics
Authorization: Bearer {admin-token}
```

**Response 200**:
```json
{
  "users": {
    "total": 10,
    "admins": 2,
    "regular": 8
  },
  "messages": {
    "total": 150,
    "pending": 20,
    "read": 100,
    "expired": 30
  }
}
```

**Response 403** (Not admin):
```json
{
  "error": "Admin access required"
}
```

---

### Create User (Admin)
Admin creates a new user.

```http
POST /api/admin/users
Authorization: Bearer {admin-token}
Content-Type: application/json
```

**Request Body**:
```json
{
  "email": "newuser@example.com",
  "name": "New User",
  "password": "password123",
  "is_admin": false
}
```

**Response 201**:
```json
{
  "id": 5,
  "email": "newuser@example.com",
  "name": "New User",
  "is_admin": false
}
```

---

### Update User (Admin)
Admin updates any user.

```http
PUT /api/admin/users/:id
Authorization: Bearer {admin-token}
Content-Type: application/json
```

**Request Body** (all fields optional):
```json
{
  "email": "updated@example.com",
  "name": "Updated Name",
  "password": "newpassword",
  "is_admin": true
}
```

**Response 200**:
```json
{
  "id": 5,
  "email": "updated@example.com",
  "name": "Updated Name",
  "is_admin": true
}
```

---

### Delete User (Admin)
Admin deletes a user.

```http
DELETE /api/admin/users/:id
Authorization: Bearer {admin-token}
```

**Response 200**:
```json
{
  "message": "User deleted successfully"
}
```

**Response 400** (Cannot delete yourself):
```json
{
  "error": "Cannot delete your own account"
}
```

---

### Import Users from CSV
Import multiple users from CSV file.

```http
POST /api/admin/users/import
Authorization: Bearer {admin-token}
Content-Type: multipart/form-data
```

**Form Data**:
- `file`: CSV file

**CSV Format**:
```csv
email,name,password,is_admin
user1@example.com,User One,password123,false
user2@example.com,User Two,password456,true
```

**Response 200**:
```json
{
  "created": 2,
  "failed": 0,
  "errors": []
}
```

**Response 200** (with errors):
```json
{
  "created": 1,
  "failed": 1,
  "errors": [
    "Row 3 (user3@example.com): user with this email already exists"
  ]
}
```

---

### Cleanup Expired Messages
Manually trigger cleanup of expired messages.

```http
POST /api/admin/cleanup
Authorization: Bearer {admin-token}
```

**Response 200**:
```json
{
  "message": "Cleanup completed",
  "expired_count": 5
}
```

---

## Error Responses

All endpoints may return these common error responses:

**401 Unauthorized**:
```json
{
  "error": "Authorization header required"
}
```

**403 Forbidden**:
```json
{
  "error": "Admin access required"
}
```

**404 Not Found**:
```json
{
  "error": "Resource not found"
}
```

**500 Internal Server Error**:
```json
{
  "error": "Internal server error"
}
```

---

## Rate Limiting

Currently no rate limiting is implemented. For production deployment, consider adding rate limiting middleware.

---

## CORS

CORS is configured to allow requests from:
- `http://localhost:5173` (Vite dev server)
- `http://localhost:3000` (React production build)

Additional origins can be configured via the `ALLOWED_ORIGINS` environment variable.

---

## Security Notes

1. **Zero-Knowledge**: Server never sees plaintext message content
2. **Burn-on-Read**: Messages are permanently deleted after first read
3. **Admin Limitations**: Even admins cannot read encrypted message content
4. **Password Hashing**: All passwords are hashed with bcrypt
5. **JWT Tokens**: Tokens expire after 24 hours (configurable)
6. **HTTPS Required**: Use HTTPS in production for clipboard API and security

---

## Example Usage

### Complete Flow: Send and Read Message

```bash
# 1. Register two users
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"sender@test.com","name":"Sender","password":"pass123"}'

curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"recipient@test.com","name":"Recipient","password":"pass123"}'

# 2. Login as sender
SENDER_TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"sender@test.com","password":"pass123"}' \
  | jq -r '.token')

# 3. Create encrypted message
curl -X POST http://localhost:8080/api/messages \
  -H "Authorization: Bearer $SENDER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "ciphertext":"encrypted-data",
    "iv":"iv-data",
    "encryption_key":"key-123",
    "recipient_id":2,
    "ttl":3600
  }'

# 4. Login as recipient and view history
RECIPIENT_TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"recipient@test.com","password":"pass123"}' \
  | jq -r '.token')

curl -X GET "http://localhost:8080/api/history" \
  -H "Authorization: Bearer $RECIPIENT_TOKEN"
```

---

**Generated**: 2025-12-31
**API Version**: 1.0
**Documentation**: Complete ✅

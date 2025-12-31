# Testing Guide

## Backend Tests

### Prerequisites

- Go 1.21+
- Redis running (for integration tests)
- PostgreSQL running (for integration tests)

### Running Tests

#### Unit Tests

Unit tests don't require external dependencies:

```bash
cd backend

# Run all unit tests
go test ./tests/unit/... -v

# Run specific test file
go test ./tests/unit/storage_test.go -v

# Run with coverage
go test ./tests/unit/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

#### Integration Tests

Integration tests require Redis and PostgreSQL:

```bash
cd backend

# Start test dependencies
docker-compose -f ../docker-compose.dev.yml up -d

# Run integration tests
go test ./tests/integration/... -v

# Stop test dependencies
docker-compose -f ../docker-compose.dev.yml down
```

#### All Tests

```bash
cd backend

# Run all tests (unit + integration)
go test ./... -v

# With coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# With race detection
go test ./... -race

# Verbose output
go test ./... -v -race -coverprofile=coverage.out
```

### Test Structure

```
backend/
├── tests/
│   ├── unit/              # Unit tests (no dependencies)
│   │   ├── storage_test.go
│   │   ├── jwt_test.go
│   │   ├── models_test.go
│   │   ├── middleware_test.go
│   │   └── handlers_test.go
│   └── integration/       # Integration tests (requires Redis/PostgreSQL)
│       └── api_test.go
```

### Writing Tests

#### Unit Test Example

```go
package unit

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestMessageValidation(t *testing.T) {
    tests := []struct {
        name    string
        message Message
        wantErr bool
    }{
        {
            name: "valid message",
            message: Message{
                Ciphertext: "valid-base64-data",
                IV:         "valid-base64-iv",
                TTL:        86400,
            },
            wantErr: false,
        },
        {
            name: "invalid TTL",
            message: Message{
                Ciphertext: "valid-base64-data",
                IV:         "valid-base64-iv",
                TTL:        100, // Too low
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.message.Validate()
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

#### Integration Test Example

```go
package integration

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestCreateMessage(t *testing.T) {
    // Setup test server
    router := setupTestRouter()

    // Create test request
    req, _ := http.NewRequest("POST", "/api/messages", strings.NewReader(`{
        "ciphertext": "test-data",
        "iv": "test-iv",
        "ttl": 86400
    }`))
    req.Header.Set("Content-Type", "application/json")

    // Execute request
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    // Assert response
    assert.Equal(t, http.StatusCreated, w.Code)
}
```

## Frontend Tests

### Prerequisites

- Node.js 20+
- npm or yarn

### Running Tests

#### Unit Tests

```bash
cd frontend

# Install dependencies
npm install

# Run tests
npm test

# Watch mode
npm test -- --watch

# With coverage
npm run test:coverage
```

#### Test Files

```
frontend/
├── src/
│   └── __tests__/
│       ├── components/
│       │   ├── MessageCreate.test.jsx
│       │   └── MessageView.test.jsx
│       ├── lib/
│       │   ├── crypto.test.js
│       │   └── clipboard.test.js
│       └── utils/
│           └── url.test.js
```

### Writing Frontend Tests

#### Component Test Example

```javascript
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import MessageCreate from '../components/MessageCreate';

describe('MessageCreate', () => {
  it('renders secret input', () => {
    render(<MessageCreate />);
    expect(screen.getByPlaceholderText(/enter secret/i)).toBeInTheDocument();
  });

  it('encrypts and creates message on submit', async () => {
    render(<MessageCreate />);

    const input = screen.getByPlaceholderText(/enter secret/i);
    const button = screen.getByText(/create link/i);

    fireEvent.change(input, { target: { value: 'test-secret' } });
    fireEvent.click(button);

    // Assert API call was made
    // Assert URL was generated
  });
});
```

#### Crypto Test Example

```javascript
import { describe, it, expect } from 'vitest';
import { generateKey, encrypt, decrypt } from '../lib/crypto';

describe('Crypto', () => {
  it('generates valid AES-GCM key', async () => {
    const key = await generateKey();
    expect(key.type).toBe('secret');
    expect(key.algorithm.name).toBe('AES-GCM');
  });

  it('encrypts and decrypts data', async () => {
    const plaintext = 'secret-message';
    const key = await generateKey();

    const { ciphertext, iv } = await encrypt(plaintext, key);
    const decrypted = await decrypt(ciphertext, iv, key);

    expect(decrypted).toBe(plaintext);
  });
});
```

## End-to-End Tests

For E2E testing, you can use Playwright or Cypress:

### Playwright Example

```bash
# Install Playwright
npm install -D @playwright/test

# Run E2E tests
npx playwright test
```

```javascript
// e2e/message-flow.spec.js
import { test, expect } from '@playwright/test';

test('complete message flow', async ({ page }) => {
  // Navigate to app
  await page.goto('http://localhost:3000');

  // Create message
  await page.fill('textarea[placeholder*="secret"]', 'test-secret');
  await page.click('button:has-text("Create Link")');

  // Get generated URL
  const url = await page.locator('[data-testid="message-url"]').textContent();

  // Open in new page
  const newPage = await page.context().newPage();
  await newPage.goto(url);

  // Verify clipboard
  await newPage.click('button:has-text("Copy to Clipboard")');
  const clipboardText = await newPage.evaluate(() => navigator.clipboard.readText());
  expect(clipboardText).toBe('test-secret');

  // Verify message burned
  await newPage.reload();
  await expect(newPage.locator('text=Message not found')).toBeVisible();
});
```

## CI/CD Testing

### GitHub Actions Example

```yaml
name: Tests

on: [push, pull_request]

jobs:
  backend:
    runs-on: ubuntu-latest
    services:
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
      postgres:
        image: postgres:15-alpine
        env:
          POSTGRES_PASSWORD: test
        ports:
          - 5432:5432

    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run tests
        run: |
          cd backend
          go test ./... -v -race -coverprofile=coverage.out

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./backend/coverage.out

  frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '20'

      - name: Install and test
        run: |
          cd frontend
          npm ci
          npm run test:coverage

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./frontend/coverage/coverage-final.json
```

## Test Coverage Goals

- **Backend**: >80% coverage
- **Frontend**: >70% coverage
- **Critical paths** (encryption, storage, auth): >95% coverage

## Manual Testing Checklist

### Message Creation Flow
- [ ] Can create message with valid secret
- [ ] URL is generated correctly
- [ ] Key is in URL fragment (#)
- [ ] Expiration time is displayed

### Message Viewing Flow
- [ ] Can view message from URL
- [ ] Secret copied to clipboard
- [ ] Message marked as burned
- [ ] Second view shows "not found"

### Security Tests
- [ ] HTTPS required in production
- [ ] Key never sent to server
- [ ] Secret never in DOM
- [ ] Message deleted after read

### Edge Cases
- [ ] Very long secrets (>1MB)
- [ ] Special characters in secrets
- [ ] Expired messages
- [ ] Invalid message IDs
- [ ] Invalid encryption keys

## Performance Testing

### Load Testing with k6

```javascript
// load-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
  stages: [
    { duration: '30s', target: 20 },
    { duration: '1m', target: 50 },
    { duration: '30s', target: 0 },
  ],
};

export default function () {
  // Create message
  let res = http.post('http://localhost:8080/api/messages', JSON.stringify({
    ciphertext: 'test-ciphertext',
    iv: 'test-iv',
    ttl: 86400,
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  check(res, {
    'status is 201': (r) => r.status === 201,
    'response time < 100ms': (r) => r.timings.duration < 100,
  });

  sleep(1);
}
```

Run with:
```bash
k6 run load-test.js
```

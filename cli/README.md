# Vanish CLI

Command-line tool for securely sharing secrets via the Vanish platform.

## Features

- 🔐 **Client-side encryption** (AES-256-GCM)
- 📎 **File transfer** (up to 5 MiB, client-side encryption)
- 📋 **Stdin support** for piping secrets
- ⚡ **Fast** (~2.6µs per encryption)
- 🔔 **Slack notifications** (optional)
- ✅ **Well tested** (32.9% coverage, all tests passing)
- 🔍 **Linted** with golangci-lint

## Installation

### From Source

```bash
cd cli
go build -o vanish
sudo mv vanish /usr/local/bin/  # Optional: install globally
```

### Using Go Install

```bash
go install github.com/milkiss/vanish/cli@latest
```

## Quick Start

### 1. Configure

```bash
vanish config
# Enter your Vanish server URL: http://localhost:8080
# Enter your API token: <paste your JWT token>
```

### 2. Send a Secret

```bash
# Direct message
vanish send user@example.com "MyPassword123"

# From stdin
echo "API_KEY=secret" | vanish send user@example.com

# With custom expiration (1 hour)
vanish send user@example.com -ttl 3600 "Temporary password"
```

### 3. Send a File

```bash
vanish send user@example.com -file ./credentials.pdf
```

### 4. Receive a Secret or File

```bash
# Text is printed to stdout; a file is saved to the current directory
vanish get "https://vanish.example.com/m/abc123#KEY"

# Save a file to a specific path (refuses to overwrite unless -force)
vanish get -o ./out.pdf "https://vanish.example.com/m/abc123#KEY"
```

Reading a link destroys the message. An explicit `-o` path is checked before
the message is fetched, so a bad destination does not burn it. Without `-o`
an existing file is never overwritten: `report.pdf` is saved as `report-1.pdf`.

Newly created files are written with permissions `0600`. `-force` overwrites an
existing file, whether the path comes from `-o` or is the default file name.

## Usage

```
Vanish CLI Tool

Usage:
  vanish config                         Configure the CLI (interactive)
  vanish send <email> [msg]             Send a secret to a user
  vanish send <email> -file <path>      Send a file (max 5 MiB)
  vanish get <link>                     Read a secret (text to stdout, files saved)
  vanish get -o <path> [-force] <link>  Save a received file to <path>

Flags for send:
  -ttl <seconds>                        Expiration time (default 86400)
  -file <path>                          File to send instead of a message
```

## Configuration

Configuration is stored in `~/.vanish/config.json`:

```json
{
  "base_url": "http://localhost:8080",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

File permissions: `0600` (user read/write only)

## Examples

### Sharing a Database Password

```bash
vanish send dba@company.com "PostgreSQL password: MySecretDB123"
```

### Piping from a File

```bash
cat ~/.ssh/id_rsa | vanish send colleague@company.com
```

### Generating and Sharing a Random Token

```bash
openssl rand -base64 32 | vanish send developer@company.com
```

### Sharing Environment Variables

```bash
# Share entire .env file
cat .env | vanish send devops@company.com

# Share specific variable
echo "DATABASE_URL=${DATABASE_URL}" | vanish send backend-dev@company.com
```

## Development

### Prerequisites

- Go 1.22 or later
- golangci-lint (for linting)

### Setup

```bash
# Install development tools
make dev-setup

# Download dependencies
make deps
```

### Testing

```bash
# Run tests
make test

# Run tests with coverage
make coverage

# Run tests (quick, no race detector)
make quick

# View coverage in browser
make coverage
open coverage.html
```

### Linting

```bash
# Run linter
make lint

# Auto-fix issues
make lint-fix

# Format code
make fmt
```

### Benchmarking

```bash
make bench
```

Results on Intel Core i5-4260U @ 1.40GHz:
```
BenchmarkEncryptMessage-4       391105    2601 ns/op    1424 B/op    13 allocs/op
BenchmarkEncryptMessageLong-4   191968    6114 ns/op    6048 B/op    13 allocs/op
```

### All Checks

```bash
# Run all checks (fmt, vet, lint, test)
make run-tests

# Or for CI
make ci
```

## Test Coverage

**Current: 32.9%**

Coverage breakdown:
- ✅ API functions (findUserID, sendToAPI, sendSlackNotification)
- ✅ Encryption functions (encryptMessage)
- ✅ Config functions (getConfigPath, save/load config)
- ⚠️ Main function and CLI parsing (harder to test)

### Running Tests

```bash
# All tests with verbose output
go test -v ./...

# With race detector
go test -race ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Linting Results

The code is checked with golangci-lint using these linters:
- errcheck
- gosimple
- govet
- ineffassign
- staticcheck
- unused
- gofmt
- goimports
- misspell
- revive
- stylecheck
- gosec
- and more...

Known linting issues (non-critical):
- Some error returns not checked (intentional for CLI UX)
- Struct field alignment optimizations available
- Some variable shadowing in tests

Run `make lint` to see current status.

## CI/CD

GitHub Actions workflow (`.github/workflows/cli-ci.yml`) runs on every push:

- ✅ Lint with golangci-lint
- ✅ Test on Linux, macOS, Windows
- ✅ Test with Go 1.22 and 1.23
- ✅ Build binaries for all platforms
- ✅ Security scan with gosec
- ✅ Upload coverage to Codecov

## Security

### Encryption

- **Algorithm**: AES-256-GCM
- **Key size**: 256 bits (32 bytes)
- **IV size**: 96 bits (12 bytes)
- **Key generation**: Cryptographically secure random (crypto/rand)

### Storage

- **Config**: `~/.vanish/config.json` with 0600 permissions
- **No secrets in logs**: Secrets never logged or printed
- **Token security**: JWT token stored securely in config file

### Threat Model

Protected against:
- ✅ Server compromise (client-side encryption)
- ✅ Network eavesdropping (encrypted transmission)
- ✅ Unauthorized config access (file permissions)

Not protected against:
- ❌ Compromised client machine
- ❌ Compromised API token
- ❌ Man-in-the-middle if using HTTP (use HTTPS!)

## Troubleshooting

### "Config not found" error

```bash
vanish config
```

### "Invalid email or password" on config

Get a fresh token:
1. Login to Vanish web UI
2. Copy JWT token from browser localStorage
3. Run `vanish config` and paste token

### "Recipient not found"

The recipient must be registered in Vanish first. Ask them to create an account.

### "Slack notification failed"

This is non-critical. The message URL is still created and displayed. You can:
- Share the URL manually
- Enable Slack integration in backend (`SLACK_ENABLED=true`)

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing`)
3. Make your changes
4. Run tests (`make test`)
5. Run linter (`make lint`)
6. Commit (`git commit -am 'Add amazing feature'`)
7. Push (`git push origin feature/amazing`)
8. Create a Pull Request

## License

Same as main Vanish project.

## Support

- Issues: https://github.com/milkiss/vanish/issues
- Documentation: https://github.com/milkiss/vanish/docs

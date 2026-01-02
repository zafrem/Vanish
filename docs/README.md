# Vanish Documentation

This directory contains comprehensive documentation for the Vanish ephemeral messaging platform.

## Documentation Index

### Getting Started
- **[Main README](../README.md)** - Quick start and overview

### API & Integration
- **[API Reference](API_REFERENCE.md)** - Complete REST API documentation with examples
- **[Enterprise Setup](ENTERPRISE_SETUP.md)** - Okta SSO, Vault, Slack, and Email integration guide
- **[Automation Use Cases](AUTOMATION_USE_CASES.md)** - Patterns for Cloud-Init, CI/CD, and Honeytokens

### Architecture & Design
- **[Architecture](ARCHITECTURE.md)** - System design, security model, and data flow
- **[Configuration](CONFIGURATION.md)** - Environment variables and settings reference

### Operations
- **[Deployment Guide](DEPLOYMENT.md)** - Production deployment instructions and best practices
- **[Testing](TESTING.md)** - How to run and write tests for backend and frontend

## Quick Links

### For Developers
- [Local Development Setup](../README.md#local-development)
- [Running Tests](TESTING.md)
- [API Endpoints](API_REFERENCE.md#endpoints)

### For Operators
- [Docker Deployment](DEPLOYMENT.md#docker-deployment)
- [Configuration Options](CONFIGURATION.md)
- [Security Best Practices](ARCHITECTURE.md#security-considerations)

### For Enterprise
- [Okta SSO Setup](ENTERPRISE_SETUP.md#okta-sso-integration)
- [HashiCorp Vault](ENTERPRISE_SETUP.md#hashicorp-vault-integration)
- [Slack Notifications](ENTERPRISE_SETUP.md#slack-integration)
- [Email Alerts](ENTERPRISE_SETUP.md#email-integration)

## Architecture Overview

```
┌─────────────┐
│   Browser   │  Client-side encryption (AES-256-GCM)
│  (React)    │  Key stays in URL fragment (#)
└──────┬──────┘
       │ HTTPS
┌──────▼──────┐
│   Go API    │  Zero-knowledge server
│   Server    │  Burn-on-read logic
└──┬───┬───┬──┘
   │   │   │
┌──▼───▼──┐ └──▼──────┐
│  Redis  │ │PostgreSQL│
│Messages │ │ Metadata │
└─────────┘ └──────────┘
```

## Key Features

- **Zero-Knowledge**: Server never sees plaintext
- **Burn-on-Read**: Messages destroyed after single view
- **Client-Side Encryption**: AES-256-GCM in browser
- **Ephemeral Storage**: No persistent message storage
- **Enterprise Ready**: SSO, audit trails, notifications

## Support

For issues or questions:
1. Check the relevant documentation above
2. Review [Architecture](ARCHITECTURE.md) for design decisions
3. See [Testing](TESTING.md) for troubleshooting
4. Contact project maintainer

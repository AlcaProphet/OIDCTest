# OIDC Playground

[![Build and Push Docker Image](https://github.com/AlcaProphet/OIDCTest/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/AlcaProphet/OIDCTest/actions/workflows/docker-publish.yml)

A lightweight OIDC debugging tool for testing self-hosted Keycloak SSO login flows. Inspired by Auth0 OIDC Playground — **zero config files**, everything via web UI.

## Features

- **Zero Config** — All settings via web form; no `.env` or config files needed
- **Auto Discovery** — One-click OIDC endpoint detection from Issuer URL
- **Keycloak Helper** — Auto-generates Root URL, Redirect URI, Web Origins — click to copy
- **Step Debugging** — Every HTTP request logged with method, URL, status code, and duration
- **Three Flows** — Authorization Code + PKCE / Authorization Code (no PKCE) / Client Credentials
- **Token Viewer** — Decodes JWT Header and Payload; structured UserInfo display
- **Persistent** — SQLite storage; configuration survives container restarts
- **Lightweight** — Single Go binary; Docker image ~15MB

## Quick Start

### Prerequisites

- Docker and Docker Compose
- A configured Keycloak instance (or any OIDC-compatible provider)
- (Optional) External NGINX for HTTPS reverse proxy

### Option 1: Build from Source

```bash
git clone <your-repo-url> KyleworksOidcTest
cd KyleworksOidcTest
docker compose up -d --build
```

### Option 2: Pull Pre-built Image (Recommended)

```bash
mkdir KyleworksOidcTest && cd KyleworksOidcTest
# Download docker-compose.yml, then:
docker compose pull && docker compose up -d
```

Visit `http://<server-ip>:61000` to see the configuration page.

### Keycloak Client Setup

When creating a client in Keycloak, use these values (also available on the tool's homepage under the "Keycloak Client Configuration Reference" card):

| Setting | Value |
|--------|-----|
| Protocol | OpenID Connect |
| Access Type | confidential |
| Standard Flow | ✅ Enabled |
| Valid Redirect URIs | `https://<your-domain>/callback` |
| Post Logout Redirect URIs | `https://<your-domain>/` |

## Usage

### 1. Configure

On first visit, fill in the form:

- **Issuer URL** — Click "Detect Endpoints" to auto-discover OIDC endpoints
- **Client ID / Client Secret** — Keycloak client credentials
- **Scopes** — Default: `openid profile email`
- **Flow** — Recommended: Authorization Code + PKCE
- **Base URL** — Auto-detected; can be overridden manually

### 2. Test

Once configured, click:

- **「Start Login」** — Initiates Authorization Code flow, redirects to Keycloak, displays full token info on return
- **「Client Credentials」** — Directly obtains an Access Token (M2M scenario)

### 3. View Results

The results page displays:

- **ID Token** — Raw JWT + decoded Header / Payload tables
- **Access Token** — Raw value + JWT decoding (if applicable)
- **Refresh Token** — Raw value (if present)
- **UserInfo** — Formatted JSON
- **Debug Timeline** — Method, URL, status code, and duration (ms) for each step; errors highlighted in red

## Supported OIDC Flows

| Flow | Description | PKCE |
|------|-------------|------|
| Authorization Code + PKCE | Auth code flow with PKCE (recommended) | ✅ |
| Authorization Code (no PKCE) | Auth code flow for comparison testing | ❌ |
| Client Credentials | Machine-to-machine, direct token acquisition | — |

## Routes

| Route | Method | Purpose |
|------|--------|---------|
| `/` | GET | Home: config form or action buttons |
| `/config` | POST | Save OIDC configuration |
| `/discover` | GET | Auto-detect OIDC endpoints (`?issuer=...`) |
| `/login` | GET | Initiate OIDC login → 302 Keycloak |
| `/callback` | GET | OIDC callback handler |
| `/result` | GET | Token viewer + debug timeline |
| `/logout` | GET | Logout (with Keycloak single sign-out) |
| `/client-credentials` | GET | Client Credentials flow |

## External NGINX Reverse Proxy

```nginx
server {
    listen 443 ssl;
    server_name oidc-test.example.com;

    location / {
        proxy_pass http://127.0.0.1:61000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Tech Stack

| Layer | Choice | Notes |
|-------|--------|-------|
| Language | Go 1.22+ | Compiles to a single static binary |
| HTTP | `net/http` stdlib | No third-party web framework |
| Templates | `html/template` | Server-side rendering, zero JS framework |
| OIDC | **Manual implementation** | Constructs HTTP requests directly, no OIDC library |
| Storage | SQLite | `modernc.org/sqlite`, pure Go, no CGO |
| Container | Multi-stage build | Final image ~15MB |

## Project Structure

```
KyleworksOidcTest/
├── main.go              # HTTP routing + sessions + template rendering
├── oidc.go              # Manual OIDC protocol implementation
├── store.go             # SQLite storage layer
├── go.mod / go.sum      # Go module definition
├── templates/
│   ├── index.html       # Config form / action buttons + Keycloak helper
│   └── result.html      # Token viewer + debug timeline
├── Dockerfile           # Multi-stage build
├── docker-compose.yml   # Docker Compose deployment
├── .github/workflows/   # GitHub Actions: auto-build and push image to GHCR
└── nginx-example.conf   # NGINX reverse proxy reference
```

## Notes

- This is a **testing/debugging tool** for non-production environments (1–5 internal users)
- Cookies use `HttpOnly` without `Secure` flag (supports HTTP testing)
- No CSRF / Rate Limiting / Security Headers (not production-grade)
- JWT is base64-decoded for display only — **signature is not verified**
- No concurrency locks / request retries / graceful shutdown

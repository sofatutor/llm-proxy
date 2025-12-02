---
title: Admin Quickstart
parent: Admin UI
nav_order: 1
---

# Admin UI Quickstart

Get started with the LLM Proxy Admin UI in minutes.

## Prerequisites

- LLM Proxy server running (see [Installation Guide](../installation.md))
- Management token set via `MANAGEMENT_TOKEN` environment variable

## Step 1: Access the Admin UI

### Integrated Mode (Default)

The Admin UI is built into the proxy server. Access it at:

```
http://localhost:8080/admin/
```

### Standalone Mode (Docker Compose)

If running the admin as a separate service:

```bash
docker compose up admin
```

Access at `http://localhost:8081/`

## Step 2: Log In

1. Open the Admin UI URL in your browser
2. Enter your management token
3. Click **Login**

![Login](../assets/screenshots/login.png)

## Step 3: Create a Project

1. Click **Projects** in the sidebar
2. Click **New Project**
3. Enter:
   - **Name**: A descriptive name (e.g., "My App")
   - **OpenAI API Key**: Your OpenAI API key (sk-...)
4. Click **Create**

![Create Project](../assets/screenshots/project-new.png)

## Step 4: Generate a Token

1. Go to your new project's detail page
2. Click **Generate Token**
3. Configure:
   - **Duration**: How long the token should be valid (hours)
   - **Max Requests**: Optional request limit
4. Click **Generate**
5. **Copy the token** (shown only once!)

![Token Created](../assets/screenshots/token-created.png)

## Step 5: Use the Token

Test your token with a curl request:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer <your-generated-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## What's Next?

- **[Project Management](projects.md)** - Learn all project features
- **[Token Management](tokens.md)** - Master token lifecycle
- **[Screenshots](screens.md)** - Visual tour of all screens

## Quick Reference

### Key URLs

| Service | URL |
|---------|-----|
| Proxy API | `http://localhost:8080` |
| Admin UI (integrated) | `http://localhost:8080/admin/` |
| Admin UI (standalone) | `http://localhost:8081/` |
| Health Check | `http://localhost:8080/health` |

### Common Actions

| Action | Location |
|--------|----------|
| View all projects | Sidebar → Projects |
| Create project | Projects → New Project |
| Generate token | Project Details → Generate Token |
| Revoke token | Tokens → Token Row → Revoke |
| View audit logs | Sidebar → Audit |

## Troubleshooting

### Cannot Access Admin UI

1. Verify proxy is running: `curl http://localhost:8080/health`
2. Check `ADMIN_UI_ENABLED=true` in environment
3. Ensure correct URL and port

### Login Fails

1. Verify management token is correct
2. Check for extra whitespace in token
3. Try clearing browser cache

### Token Not Working

1. Check token was copied completely
2. Verify token hasn't expired
3. Confirm project is active

See [Troubleshooting Guide](../troubleshooting.md) for more issues.

# Railway Deployment Guide — Production

## Overview

| Service | How it's deployed |
|---|---|
| Go backend | Railway Web Service from `Dockerfile` |
| PostgreSQL | Railway managed Postgres plugin |
| Migrations | Auto-run on startup (embedded in binary) |
| pgAdmin | Not deployed — use TablePlus locally or Railway's DB UI |

**Health check:** `GET /health` (pings DB — returns 200 when ready)

---

## Prerequisites

- [ ] GitHub account with this repo pushed
- [ ] Railway account — [railway.app](https://railway.app) (sign in with GitHub)
- [ ] Google Cloud project with OAuth credentials (for Google/Gmail integration)
- [ ] Groq API key (for LLM features)

---

## Step 1 — Create Railway Project

1. Go to [railway.app/new](https://railway.app/new)
2. Click **Deploy from GitHub repo**
3. Authorize Railway to access your GitHub account
4. Select the `mengu-backend` repository
5. Railway detects the `Dockerfile` automatically — click **Deploy Now**

> The first build will fail (no database yet). That's expected — continue to Step 2.

---

## Step 2 — Add PostgreSQL Database

1. Inside your Railway project, click **+ New**
2. Select **Database** → **Add PostgreSQL**
3. Railway creates a managed Postgres 17 instance

Railway automatically creates a `DATABASE_URL` variable and **links it to your app service**. No manual copy-paste needed.

To verify: click your app service → **Variables** tab → you should see `DATABASE_URL` already present.

---

## Step 3 — Set Environment Variables

Click your **app service** → **Variables** tab → **Raw Editor**, then paste:

```
# Auth — generate a strong secret: openssl rand -base64 32
JWT_SECRET=<your-strong-random-secret-min-32-chars>
JWT_ACCESS_TTL=1h
JWT_REFRESH_TTL=168h

# LLM / AI
LLM_API_URL=https://api.groq.com/openai/v1
LLM_API_KEY=<your-groq-api-key>
LLM_MODEL=meta-llama/llama-4-scout-17b-16e-instruct
LLM_TIMEOUT=30s

# Google OAuth (from Google Cloud Console → Credentials)
GOOGLE_CLIENT_ID=<your-google-client-id>
GOOGLE_CLIENT_SECRET=<your-google-client-secret>

# Set these AFTER you get your Railway URL in Step 5
OAUTH_REDIRECT_URI=https://<your-app>.railway.app/api/v1/auth/oauth/callback
FRONTEND_URL=https://<your-frontend-url>
CORS_ALLOWED_ORIGINS=https://<your-frontend-url>

# Gmail push notifications (optional — only if using Gmail watch)
GMAIL_TOPIC_NAME=projects/<gcp-project-id>/topics/<topic-name>
GMAIL_SUBSCRIPTION_NAME=projects/<gcp-project-id>/subscriptions/<sub-name>
GMAIL_SERVICE_ACCOUNT=<base64-encoded-service-account-json>

# Microsoft OAuth (optional)
MICROSOFT_CLIENT_ID=<your-ms-client-id>
MICROSOFT_CLIENT_SECRET=<your-ms-client-secret>

# Server
LOG_LEVEL=info
LOG_FORMAT=json
CORS_ALLOWED_ORIGINS=https://<your-frontend-url>
WORKER_POLL_INTERVAL=5s
WORKER_SHUTDOWN_TIMEOUT=30s
SHUTDOWN_TIMEOUT=15s
```

> `DATABASE_URL` and `PORT` are injected by Railway automatically — do NOT set them manually.

**Generate JWT_SECRET:**
```bash
openssl rand -base64 32
```

---

## Step 4 — Trigger Redeploy

After setting variables:

1. Click your app service → **Deployments** tab
2. Click the three-dot menu on the latest deployment → **Redeploy**

OR push any commit to `main` — Railway redeploys automatically on every push.

---

## Step 5 — Get Your Public URL

1. Click your app service → **Settings** tab → **Networking** section
2. Click **Generate Domain** — Railway gives you `https://<random-name>.railway.app`
3. Go back to **Variables** and update:
   ```
   OAUTH_REDIRECT_URI=https://<your-app>.railway.app/api/v1/auth/oauth/callback
   ```
4. Redeploy again (Railway auto-redeploys when variables change)

---

## Step 6 — Update Google Cloud OAuth Settings

Go to [Google Cloud Console](https://console.cloud.google.com) → **APIs & Services** → **Credentials** → your OAuth client:

**Authorized redirect URIs — add:**
```
https://<your-app>.railway.app/api/v1/auth/oauth/callback
```

**Authorized JavaScript origins — add:**
```
https://<your-app>.railway.app
```

Save. Changes take up to 5 minutes to propagate.

---

## Step 7 — Verify Deployment

```bash
# Health check — should return 200 with {"status":"ok","db":"connected"}
curl https://<your-app>.railway.app/health

# API is live
curl https://<your-app>.railway.app/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"testpass123","name":"Test"}'
```

Swagger docs: `https://<your-app>.railway.app/swagger/index.html`

---

## Step 8 — Custom Domain (Optional)

1. App service → **Settings** → **Networking** → **Custom Domain**
2. Add your domain (e.g. `api.mengu.ai`)
3. Add the CNAME record Railway shows you to your DNS provider
4. Wait for DNS propagation (~5–30 min)
5. Update `OAUTH_REDIRECT_URI` and `CORS_ALLOWED_ORIGINS` to use the custom domain

---

## Database — Production Tips

### Connect locally to prod DB

```bash
# Install Railway CLI
npm install -g @railway/cli

# Login
railway login

# Open a psql shell to prod DB
railway connect postgres
```

### Run a manual migration (if ever needed)

Migrations run automatically on every deploy. If you ever need to run them manually:

```bash
railway run -- /server  # migrations run as part of startup
```

### Backups

Railway Postgres includes automatic daily backups on paid plans. On the free tier, export manually:

```bash
railway connect postgres
# Inside psql:
\copy (SELECT * FROM users) TO 'users_backup.csv' CSV HEADER;
```

---

## Environment Variables Reference

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | Auto | Injected by Railway Postgres plugin |
| `PORT` | Auto | Injected by Railway |
| `JWT_SECRET` | Yes | Min 32 chars, random |
| `JWT_ACCESS_TTL` | No | Default: `1h` |
| `JWT_REFRESH_TTL` | No | Default: `168h` |
| `LLM_API_URL` | Yes | Groq or OpenAI-compatible base URL |
| `LLM_API_KEY` | Yes | API key for LLM provider |
| `LLM_MODEL` | No | Default: `gpt-4` |
| `GOOGLE_CLIENT_ID` | Yes* | For Google OAuth |
| `GOOGLE_CLIENT_SECRET` | Yes* | For Google OAuth |
| `OAUTH_REDIRECT_URI` | Yes* | Must match Google Cloud Console |
| `FRONTEND_URL` | Yes | Used for OAuth redirects back to frontend |
| `CORS_ALLOWED_ORIGINS` | Yes | Set to your frontend URL in prod |
| `MICROSOFT_CLIENT_ID` | No | For Microsoft OAuth |
| `MICROSOFT_CLIENT_SECRET` | No | For Microsoft OAuth |
| `GMAIL_TOPIC_NAME` | No | For Gmail push notifications |
| `GMAIL_SUBSCRIPTION_NAME` | No | For Gmail push notifications |
| `GMAIL_SERVICE_ACCOUNT` | No | Base64 JSON service account |
| `LOG_LEVEL` | No | `info` or `debug` |
| `LOG_FORMAT` | No | `json` (recommended for prod) |
| `WORKER_POLL_INTERVAL` | No | Default: `5s` |
| `SHUTDOWN_TIMEOUT` | No | Default: `10s` |

*Required if using Google OAuth or Gmail integration.

---

## Troubleshooting

**Build fails: `golang:1.26-alpine` not found**
→ Docker Hub doesn't have that tag yet. Change `Dockerfile` line 1 to `golang:1.23-alpine`.

**App crashes on start: `unable to connect to database`**
→ Check that Railway Postgres plugin is linked to your app service (Variables tab should show `DATABASE_URL`).

**502 Bad Gateway after deploy**
→ App is still starting or health check failed. Check **Deployments → Build Logs** and **Deploy Logs**.

**OAuth callback returns error**
→ `OAUTH_REDIRECT_URI` in env vars doesn't exactly match what's in Google Cloud Console. Both must be identical including trailing slash.

**CORS errors from frontend**
→ `CORS_ALLOWED_ORIGINS` must exactly match your frontend's origin (no trailing slash), e.g. `https://app.mengu.ai`.

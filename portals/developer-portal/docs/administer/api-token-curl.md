# Getting a Bearer Token via curl (IDP Mode)

When the Developer Portal is configured with an external IDP (e.g. Asgardeo), REST API calls to `/devportal/v1/*` must include an `Authorization: Bearer <token>` header. This guide shows how to obtain that token from the terminal without a browser.

## Prerequisites

- IDP is configured (`identityProvider.clientId` is set in `config.yaml`)
- The `dp:*` scopes are registered in the IDP and assigned to your user (see [asgardeo-setup.md](asgardeo-setup.md) sections 3–4)
- You have the **client ID** and **client secret** from your IDP application
- You know your org's UUID (the `ORG_ID` column in the `DP_ORGANIZATION` table, or ask the admin)

---

## Flow: Authorization Code + PKCE

The devportal application is a confidential Traditional Web App — it uses authorization code flow with PKCE and a client secret. You need to:

1. Generate a PKCE code verifier and challenge
2. Open the authorization URL (paste into browser or use a redirect capture trick)
3. Exchange the authorization code for a token

---

## Step 1 — Generate PKCE values

```bash
# Code verifier: 43–128 random URL-safe characters
CODE_VERIFIER=$(openssl rand -base64 64 | tr -d '=+/' | cut -c1-64)

# Code challenge: SHA-256 of the verifier, base64url-encoded
CODE_CHALLENGE=$(echo -n "$CODE_VERIFIER" | openssl dgst -sha256 -binary | base64 | tr '+/' '-_' | tr -d '=')

echo "CODE_VERIFIER=$CODE_VERIFIER"
echo "CODE_CHALLENGE=$CODE_CHALLENGE"
```

---

## Step 2 — Start a local redirect listener

The IDP will redirect back to a callback URI with the authorization code. Use `nc` to capture it:

```bash
PORT=8080
nc -l $PORT &
NC_PID=$!
```

> **Note:** Register `http://localhost:8080` as an authorized redirect URI in your IDP application before proceeding.

---

## Step 3 — Build the authorization URL and open it

Set your IDP and application values:

```bash
TENANT=<your-tenant>                           # e.g. dev1234
CLIENT_ID=<devportal-app-client-id>
ORG_IDENTIFIER=<org-identifier>                # ORGANIZATION_IDENTIFIER value, e.g. "sub"
STATE=$(openssl rand -hex 16)

AUTH_URL="https://api.asgardeo.io/t/${TENANT}/oauth2/authorize\
?response_type=code\
&client_id=${CLIENT_ID}\
&redirect_uri=http://localhost:${PORT}\
&scope=openid%20profile%20email%20dp:api_manage%20dp:app_manage%20dp:org_manage%20dp:subscription_manage\
&code_challenge=${CODE_CHALLENGE}\
&code_challenge_method=S256\
&state=${STATE}\
&org=${ORG_IDENTIFIER}"

echo "Open this URL in your browser:"
echo "$AUTH_URL"
```

Open the URL in a browser, log in, and approve. The browser is redirected to `http://localhost:8080?code=...&state=...`. The `nc` process captures the raw HTTP request.

---

## Step 4 — Extract the authorization code

From the `nc` output, copy the `code` parameter value:

```bash
# nc prints something like:
# GET /?code=abc123xyz&state=... HTTP/1.1

CODE=<paste-code-value-here>
kill $NC_PID 2>/dev/null
```

---

## Step 5 — Exchange the code for a token

```bash
TOKEN_URL="https://api.asgardeo.io/t/${TENANT}/oauth2/token"
CLIENT_SECRET=<devportal-app-client-secret>

RESPONSE=$(curl -s -X POST "$TOKEN_URL" \
  -u "${CLIENT_ID}:${CLIENT_SECRET}" \
  -d "grant_type=authorization_code" \
  -d "code=${CODE}" \
  -d "redirect_uri=http://localhost:${PORT}" \
  -d "code_verifier=${CODE_VERIFIER}")

echo "$RESPONSE" | jq .

TOKEN=$(echo "$RESPONSE" | jq -r '.access_token')
echo "TOKEN=$TOKEN"
```

---

## Step 6 — Call the API

```bash
ORG_UUID=<org-uuid>    # ORG_ID from the DP_ORGANIZATION table, e.g. 65789d2d-0238-412a-995c-5ce74c82e169
BASE="https://localhost:3000/o/${ORG_UUID}/devportal/v1"

# List APIs
curl -sk "${BASE}/apis" -H "Authorization: Bearer $TOKEN" | jq .

# List applications
curl -sk "${BASE}/applications" -H "Authorization: Bearer $TOKEN" | jq .

# Create an application
curl -sk -X POST "${BASE}/applications" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"displayName": "My CLI App", "description": "Created via API"}' | jq .
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `403 Token org does not match requested organization` | Token's `org_name` claim doesn't match the org UUID's `ORGANIZATION_IDENTIFIER` | Make sure you logged in with `org=<ORGANIZATION_IDENTIFIER>` in the auth URL and are using the matching org UUID |
| `403 Forbidden` (scope error) | Token is missing required `dp:*` scopes | Complete [asgardeo-setup.md](asgardeo-setup.md) sections 3–4: register scopes and assign role to your user |
| `401 Authentication required` | Token expired or invalid | Re-run steps 1–5 to get a fresh token |
| Token has no `dp:*` scopes | Role not assigned to user in the sub-org | In Asgardeo console, go to the sub-org → Users → assign the `dp_admin` role |
| `nc` gets no output | Redirect URI not registered in IDP | Add `http://localhost:8080` to authorized redirect URIs in the Asgardeo application |

---

## Token lifetime

Asgardeo access tokens typically expire in 3600 seconds (1 hour). Re-run steps 1–5 to get a new token. The devportal also supports refresh tokens — but from the terminal, it's simpler to just re-authenticate.

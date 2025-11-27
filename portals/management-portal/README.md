# Management Portal – WSO2 API Platform

The **Management Portal** is the central control plane of the **WSO2 API Platform**, enabling unified management of gateways, APIs, policies, identity providers, governance rules, and API lifecycle processes. It serves as the administrative hub for configuring the platform, orchestrating deployments, and publishing APIs to developer portals.

---

## 🚀 Key Capabilities

* **Gateway Management & Orchestration**
* **API Lifecycle & Versioning Management**
* **Policy & Governance Rule Configuration**
* **Identity Provider & Authentication Setup**
* **API Deployment to Gateways**
* **Developer Portal Publishing Management**
* **Supports Multi‑tenant SaaS & On‑Prem Deployments**

---

## 🧩 Tech Stack

* **React + TypeScript**
* **Vite** (Frontend build tool)
* **Go** (Backend – Platform API)
* **Node.js & npm**
* **PostgreSQL** (Developer Portal database)

---

# ⚙️ Development Setup

Follow this guide to fully set up the development environment for the Management Portal.

---

# 1️⃣ Clone the API Platform Repository

```bash
git clone https://github.com/wso2/api-platform
cd api-platform
```

---

# 2️⃣ Start the Backend – Platform API

The Management Portal requires the **Platform API** to be running.

### Option 1: Run with Docker

```bash
cd platform-api
docker build -t wso2/platform-api:latest .
docker run -p 8443:8443 wso2/platform-api:latest
```

### Option 2: Run Locally

```bash
cd platform-api/src
go run ./cmd/main.go
```

---

# 3️⃣ (Optional) Run the Developer Portal

Running the Developer Portal is optional unless you want to test **API publishing workflows**.

```bash
cd portals/developer-portal
```

## Step A — Create `config.json` and `secret.json`

Use the following template files:

* `sample_config.json`
* `sample_secret.json`

Copy and modify them based on your environment.

## Step B — Set Up PostgreSQL

Create a database using the script:

```bash
portals/developer-portal/artifacts/script.sql
```

After DB creation:

* Update **dbSecret** in `secret.json` with the DB password
* Update DB configs in `config.json`

## Step C — Use the Same API Key Values

Ensure both portals share the same API key configuration:

```json
"apiKey": {
  "enabled": true,
  "keyType": "api-key",
  "keyValue": "api-key"
}
```

This must match the `apiKeySecret` value in `secret.json`.

## Step D — Organization Mapping

The organization name in Management Portal must match the Developer Portal organization.

Create an organization via:

```bash
curl --location 'http://localhost:<port or 3000>/devportal/organizations' \ 
--header 'api-key: xxx' \ 
--header 'Content-Type: application/json' \ 
--data '{
    "orgName": "<name>",
    "orgHandle": "<name>",
    "organizationIdentifier": "<id>"
}'
```

## Step E — Start the Developer Portal

```bash
cd portals/developer-portal
npm ci
npm start
```

---

# 4️⃣ Run the Management Portal

```bash
cd portals/management-portal
npm ci --legacy-peer-deps
npm ci react-markdown --legacy-peer-deps   # Only if react-markdown was skipped
npm run dev
```

Once these steps are complete, the Management Portal will be running.

---

# 5️⃣ Important Pre‑requisites

### ✅ Allow Backend HTTPS Certificate in Browser

Visit:

```
https://localhost:8443/
```

Accept the security warning — otherwise the Management Portal cannot call the backend.

---

### ✅ Create a Default Organization in Platform API

Use this curl command:

```bash
curl --location 'https://localhost:8443/api/v1/organizations' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer <shared-token>' \
--data '{
    "id": "15b2ac94-6217-4f51-90d4-b2b3814b20b4",
    "handle": "acme",
    "name": "ACME Corporation",
    "region": "US"
}'
```

You may obtain the `<shared-token>` from a team member.

Once this organization is created, the Management Portal will initialize correctly.

---

# 🟢 Development Workflow

After completing all steps above, you can begin active development on the Management Portal. The platform will be fully functional for end‑to‑end testing involving:

* API creation
* API publishing
* Deployment
* Policy configuration
* Developer portal visibility

---

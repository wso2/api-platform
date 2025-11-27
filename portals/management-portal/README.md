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
* **Supports Multi-tenant SaaS & On-Prem Deployments**

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

## Table of Contents

1. [Clone the Repository](#1️⃣-clone-the-repository)
2. [Start Backend – Platform API](#2️⃣-start-backend--platform-api)
3. [Run the Developer Portal (Optional)](#3️⃣-run-the-developer-portal-optional)
4. [Run the Management Portal](#4️⃣-run-the-management-portal)
5. [Important Prerequisites](#5️⃣-important-prerequisites)
6. [Development Workflow](#development-workflow)

---

# 1️⃣ Clone the Repository

```bash
git clone https://github.com/wso2/api-platform
cd api-platform
```

---

# 2️⃣ Start Backend – Platform API

The Management Portal requires the **Platform API** to be running.

## Option 1: Run with Docker

```bash
cd platform-api
docker build -t wso2/platform-api:latest .
docker run -p 8443:8443 wso2/platform-api:latest
```

## Option 2: Run Locally

```bash
cd platform-api/src
go run ./cmd/main.go
```

---

# 3️⃣ Run the Developer Portal (Optional)

Running the Developer Portal is optional unless you want to test **API publishing workflows**.

```bash
cd portals/developer-portal
```

## Step A — Create `config.json` and `secret.json`

Use the template files (paths relative to repo root):

* `portals/developer-portal/sample_config.json`
* `portals/developer-portal/sample_secret.json`

Copy and modify based on your environment.

## Step B — Set Up PostgreSQL

```text
portals/developer-portal/artifacts/script.sql
```

After DB creation:

* Update **dbSecret** in `secret.json`
* Update DB configs in `config.json`

## Step C — Use the Same API Key Values

```json
"apiKey": {
  "enabled": true,
  "keyType": "api-key",
  "keyValue": "api-key"
}
```

## Step E — Organization Mapping

Create an organization in Developer Portal:

## Step D — Start the Developer Portal

```bash
cd portals/developer-portal
npm ci --legacy-peer-deps
npm start
```

```bash
curl --location 'http://localhost:<port or 3000>/devportal/organizations' --header 'api-key: xxx' --header 'Content-Type: application/json' --data '{
    "orgName": "<name>",
    "orgHandle": "<name>",
    "organizationIdentifier": "<id>"
}'
```

---

# 4️⃣ Run the Management Portal

```bash
cd portals/management-portal
npm ci --legacy-peer-deps
npm run dev
```

---

# 5️⃣ Important Prerequisites

## ✅ Allow Backend HTTPS Certificate in Browser

```text
https://localhost:8443/
```

## ✅ Create a Default Organization in Platform API

```bash
curl --location 'https://localhost:8443/api/v1/organizations' --header 'Content-Type: application/json' --header 'Authorization: Bearer <shared-token>' --data '{
    "id": "15b2ac94-6217-4f51-90d4-b2b3814b20b4",
    "handle": "acme",
    "name": "ACME Corporation",
    "region": "US"
}'
```

---

# Development Workflow

Once the setup is complete, the platform is ready for end-to-end development:

* API creation
* API publishing
* Deployment to gateways
* Policy configuration
* Developer portal integration

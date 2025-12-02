# Quick Start Guide

Get the WSO2 API Platform up and running in minutes with Docker Compose.

## 1. Clone the Repository

```bash
git clone https://github.com/wso2/api-platform
cd api-platform
```

## 2. Start the Platform

```bash
cd distribution/all-in-one
docker compose up --build
```

## 3. Create a Default Organization

```bash
curl -k --location 'https://localhost:8443/api/v1/organizations' \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer <shared-token>' \
  --data '{
    "id": "15b2ac94-6217-4f51-90d4-b2b3814b20b4",
    "handle": "acme",
    "name": "ACME Corporation",
    "region": "US"
}'
```

## 4. Accept the Self-Signed Certificate

Open [https://localhost:8443](https://localhost:8443) in your browser and accept the self-signed certificate.

## 5. Open the Management Portal

Navigate to [http://localhost:5173](http://localhost:5173) to access the Management Portal.
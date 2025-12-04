# Developer Portal

A web-based developer portal for API management.

## Prerequisites

- Node.js v22.0.0 or later
- Docker

## Getting Started

### 1. Set Up PostgreSQL Database

Start a PostgreSQL container:

```bash
docker run -d --name postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:latest
```

Create the database:

```bash
docker exec postgres psql -U postgres -c "CREATE DATABASE devportal;"
```

Initialize the schema:

```bash
docker exec -i postgres psql -U postgres -d devportal < artifacts/script.sql
```

### 2. Install Dependencies

```bash
npm install
```

### 3. Start the Application

```bash
npm start
```

The developer portal will be available at http://localhost:3001


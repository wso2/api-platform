-- PostgreSQL Schema for Event-Gateway extension tables.
-- Applied after the base gateway-controller-db.postgres.sql schema.

-- Subscription plans table (organization-scoped rate/billing plans)
CREATE TABLE IF NOT EXISTS subscription_plans (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    plan_name TEXT NOT NULL,
    billing_plan TEXT,
    stop_on_quota_reach BOOLEAN DEFAULT TRUE,
    throttle_limit_count INTEGER,
    throttle_limit_unit TEXT,
    expiry_time TIMESTAMPTZ,
    status TEXT NOT NULL CHECK(status IN ('ACTIVE', 'INACTIVE')) DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE(gateway_id, plan_name)
);

-- Subscriptions table (application-level subscriptions for APIs)
-- subscription_token_hash: for xDS validation and request validation (Platform-API stores original token)
CREATE TABLE IF NOT EXISTS subscriptions (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    api_id TEXT NOT NULL,
    application_id TEXT,
    subscription_token_hash TEXT NOT NULL,
    subscription_plan_id TEXT,
    billing_customer_id TEXT,
    billing_subscription_id TEXT,
    status TEXT NOT NULL CHECK(status IN ('ACTIVE', 'INACTIVE', 'REVOKED')) DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY (gateway_id, subscription_plan_id) REFERENCES subscription_plans(gateway_id, uuid),
    UNIQUE(gateway_id, api_id, subscription_token_hash)
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_application_id ON subscriptions(application_id);

-- Per-API HMAC secrets for the websub-hmac-auth policy.
-- Ciphertext is AES-256-GCM encrypted; plaintext is never stored.
CREATE TABLE IF NOT EXISTS webhook_secrets (
    uuid          TEXT NOT NULL,
    gateway_id    TEXT NOT NULL,
    artifact_uuid TEXT NOT NULL,
    name          TEXT NOT NULL,
    display_name  TEXT NOT NULL,
    ciphertext    BYTEA NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'revoked')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE (gateway_id, artifact_uuid, name),
    FOREIGN KEY (gateway_id, artifact_uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_webhook_secrets_artifact ON webhook_secrets(gateway_id, artifact_uuid);

-- Billing identifiers on subscriptions.
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS billing_customer_id TEXT;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS billing_subscription_id TEXT;

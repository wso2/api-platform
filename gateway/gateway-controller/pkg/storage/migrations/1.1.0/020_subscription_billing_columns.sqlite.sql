-- Billing identifiers on subscriptions.
ALTER TABLE subscriptions ADD COLUMN billing_customer_id TEXT;
ALTER TABLE subscriptions ADD COLUMN billing_subscription_id TEXT;

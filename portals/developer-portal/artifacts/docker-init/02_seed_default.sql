-- Default seed data for the "explore" Docker Compose setup.
-- All statements are idempotent (ON CONFLICT DO NOTHING).
--
-- Seeds:
--   • ACME organization (fixed UUID matching org_data.sh)
--   • Default view, label, view-label link
--   • WSO2 provider entry
--   • Bronze / Silver / Gold / Unlimited / AsyncUnlimited subscription policies

-- Fixed UUIDs — keep in sync with org_data.sh if you use it separately
DO $$
DECLARE
    v_org_id       VARCHAR(256) := '1ba42a09-45c0-40f8-a1bf-e4aa7cde1575';
    v_view_id      VARCHAR(256) := 'f5d8e122-3c4a-4b67-8d12-a1b2c3d4e5f6';
    v_label_id     VARCHAR(256) := 'a2b3c4d5-e6f7-4a8b-9c0d-1e2f3a4b5c6d';
    v_vl_id        VARCHAR(256) := 'b3c4d5e6-f7a8-4b9c-0d1e-2f3a4b5c6d7e';
    v_pol_bronze   VARCHAR(256) := 'c4d5e6f7-a8b9-4c0d-1e2f-3a4b5c6d7e8f';
    v_pol_silver   VARCHAR(256) := 'd5e6f7a8-b9c0-4d1e-2f3a-4b5c6d7e8f90';
    v_pol_gold     VARCHAR(256) := 'e6f7a8b9-c0d1-4e2f-3a4b-5c6d7e8f9a0b';
    v_pol_unl      VARCHAR(256) := 'f7a8b9c0-d1e2-4f3a-4b5c-6d7e8f9a0b1c';
    v_pol_async    VARCHAR(256) := 'a8b9c0d1-e2f3-4a4b-5c6d-7e8f9a0b1c2d';
BEGIN

-- ── Organization ─────────────────────────────────────────────────────────────
INSERT INTO "DP_ORGANIZATION" (
    "ORG_ID", "ORG_NAME", "BUSINESS_OWNER", "BUSINESS_OWNER_CONTACT",
    "BUSINESS_OWNER_EMAIL", "ORG_HANDLE", "ROLE_CLAIM_NAME", "GROUPS_CLAIM_NAME",
    "ORGANIZATION_CLAIM_NAME", "ORGANIZATION_IDENTIFIER",
    "ADMIN_ROLE", "SUBSCRIBER_ROLE", "SUPER_ADMIN_ROLE", "ORG_CONFIG"
) VALUES (
    v_org_id, 'ACME', '', '', '', 'ACME',
    'roles', 'groups', 'organizationID', 'ACME',
    'admin', 'Internal/subscriber', 'superAdmin', '{}'
) ON CONFLICT DO NOTHING;

-- ── Default View ──────────────────────────────────────────────────────────────
INSERT INTO "DP_VIEWS" ("VIEW_ID", "ORG_ID", "NAME", "DISPLAY_NAME")
VALUES (v_view_id, v_org_id, 'default', 'default')
ON CONFLICT DO NOTHING;

-- ── Default Label ─────────────────────────────────────────────────────────────
INSERT INTO "DP_LABELS" ("LABEL_ID", "ORG_ID", "NAME", "DISPLAY_NAME")
VALUES (v_label_id, v_org_id, 'default', 'default')
ON CONFLICT DO NOTHING;

-- ── Label → View link ────────────────────────────────────────────────────────
INSERT INTO "DP_VIEW_LABELS" ("ID", "LABEL_ID", "VIEW_ID", "ORG_ID")
VALUES (v_vl_id, v_label_id, v_view_id, v_org_id)
ON CONFLICT DO NOTHING;

-- ── WSO2 Provider ─────────────────────────────────────────────────────────────
INSERT INTO "DP_PROVIDER" ("ORG_ID", "NAME", "PROPERTY", "VALUE")
VALUES (v_org_id, 'WSO2', 'providerURL', 'https://localhost:9443/api/am/devportal/v2')
ON CONFLICT DO NOTHING;

-- ── Subscription Policies ────────────────────────────────────────────────────
INSERT INTO "DP_SUBSCRIPTION_POLICY"
    ("ORG_ID", "POLICY_ID", "POLICY_NAME", "DISPLAY_NAME", "BILLING_PLAN", "DESCRIPTION", "REQUEST_COUNT")
VALUES
    (v_org_id, v_pol_bronze, 'Bronze',        'Bronze',        'FREE', 'Allows 1000 requests per minute',               '1000'),
    (v_org_id, v_pol_silver, 'Silver',        'Silver',        'FREE', 'Allows 2000 requests per minute',               '2000'),
    (v_org_id, v_pol_gold,   'Gold',          'Gold',          'FREE', 'Allows 5000 requests per minute',               '5000'),
    (v_org_id, v_pol_unl,    'Unlimited',     'Unlimited',     'FREE', 'Allows unlimited requests',                     'Unlimited'),
    (v_org_id, v_pol_async,  'AsyncUnlimited','AsyncUnlimited','FREE', 'Allows unlimited requests for Async APIs',      'Unlimited')
ON CONFLICT DO NOTHING;

END $$;

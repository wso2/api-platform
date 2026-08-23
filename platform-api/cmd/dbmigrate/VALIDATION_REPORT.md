# Validation Report — Platform API v1 → v2 migration

**Run:** `prod` · **Date:** 2026-08-22 · **Mode:** live + verify
**Source:** choreo dev dump (`choreo-dev-v1-backup.sql`, 447 artifacts), restored to Postgres 15 on `:5432`.
**Target:** fresh empty Postgres 15 on `:5433`; core + EventGateway plugin DDL applied by the tool.
**Built from:** v2 `a2911a091` (branch `migration`); v1 `f4556fee3` (`platform-api/v0.10.x`).

## Headline

**Verify: PASS — 121 PASS / 1 WARN / 0 FAIL** (`verify-report-prod.json`, exit 0). All 447 artifacts
(all six types incl. **84 WebSub + 8 WebBroker**) migrated with **zero quarantine** and zero config
data loss. This corrects the prior `prod-20260818` run, which **dropped all 92 event APIs** and
quarantined 102 of their orphaned children.

The single WARN is the sampled subscription-token *decrypt* round-trip, skipped because choreo dev's
token key is not on this machine. The token **bytes** were verified byte-identical to v1 (verbatim
passthrough), and the decrypt guard was proven to fail-fast on a wrong key (see below).

## Coverage (processed_per_table — all reconcile v2 + quarantine(0) == v1)

| table | rows | | table | rows |
|---|--:|---|---|--:|
| organizations | 107 | | subscriptions | 9 |
| projects | 226 | | gateways | 143 |
| applications | 20 | | gateway_endpoints | 143 |
| **artifacts** | **447** | | artifact_gateway_mappings | 108 |
| rest_apis | 193 | | gateway_custom_policies | 1 |
| llm_provider_templates | 750 | | gateway_custom_policy_usages | 3 |
| llm_providers | 93 | | gateway_tokens | 228 |
| llm_proxies | 49 | | deployments | 536 |
| mcp_proxies | 20 | | deployment_status | 194 |
| **websub_apis** | **84** | | api_keys | 139 |
| **webbroker_apis** | **8** | | application_api_key_mappings | 12 |
| subscription_plans | 60 | | application_artifact_mappings | 2 |
| subscription_plan_limits | 60 | | user_idp_references | 24 |

`gateway_tokens` (228) and the full `deployments`/`deployment_status`/`api_keys` counts are migrated
here but were absent/short in the Aug-18 run (event-artifact children were orphaned then).

## Verify layers (all PASS unless noted)

- **A Coverage:** each per-type split reconciles (`websub 84`, `webbroker 8`, …); Σ per-type == v2.artifacts (447);
  `subscription_plan_limits`==v1 throttled plans; `gateway_endpoints`==v1 non-empty vhosts;
  `websub_api_hmac_secrets`==0; `secrets`==0.
- **B Scalar (FULL):** `display_name`==v1 `name`; `created_at` instant preserved; `kind`==`type`;
  `subscriptions.api_uuid`→`artifact_uuid`; `plan_name`==`display_name` — 0 mismatch.
- **C Round-trip (FULL, decode-and-compare):** config invariants **0 mismatch** for rest_apis(193),
  websub_apis(84), webbroker_apis(8), llm_providers(93), llm_proxies(49), mcp_proxies(20);
  transport-column→blob **0 mismatch**; `subscription_plan_limits` values exact; `gateway_endpoints.url`==`vhost`;
  `subscription_token`/`_hash` byte-verbatim.
- **D Integrity (FULL):** every v2 FK resolves (0 orphans); every `(organization_uuid, handle)` unique per
  type table; `llm_provider_templates(org, group_id, version)` unique; `user_idp_references(idp_id)` unique.
- **E Generated/defaults:** carried handles == `slug(v1)`; `idp_organization_ref_uuid`==org uuid;
  template `group_id`==handle, `version`=v1.0, `managed_by`=organization, `is_latest`/`enabled`=1;
  `data_version`=1.0 & `origin`=control_plane everywhere; every `created_by` resolves in `user_idp_references`.
- **F Drop reconciliation:** intentional drops match v1 counts (devportals 107, dev_portal assoc 193,
  billing_plan 60, llm/llm-proxy/mcp status 93/49/20); every migrated table reconciles; quarantine sign-off: 0 outstanding.

## Intentional drops (auditable, `drops-prod.jsonl`)

| feature | rows | scope |
|---|--:|---|
| devportals | 107 | row (feature removed in v2) |
| dev_portal_association | 193 | row (association_mappings dev_portal rows) |
| publication_mappings | 0 | row (none in source) |
| billing_plan | 60 | field (dropped column) |
| llm_provider_status / llm_proxy_status / mcp_status | 93 / 49 / 20 | field (no v2 target column) |

Event APIs are **not** in the drop bucket.

## Flags (migrate-with-flag audit trail, `flags-prod.jsonl`)

| code | count | meaning |
|---|--:|---|
| SYNTHESIZED | 1421 | generated handles (projects/gateways/plans/api_keys), template defaults, seeded identities, **WebSub structural reshapes** |
| DEFAULTED_NULL | 2630 | rows with no v1 `created_by` → migration-actor UUID (§C.1) |
| PLAINTEXT_CREDENTIAL | 139 | LLM config blobs carrying an upstream apiKey in plaintext (security note) |
| PLACEHOLDER_IDP | 107 | `idp_organization_ref_uuid` = org uuid (backfill from IdP later) |
| TRUNCATED | 3 | value narrowed to a v2 VARCHAR limit (lossy but valid) |

## Notable finding — WebSub config is NOT a strict subset of v2

Dry-run on real data disproved the "strict subset" premise: 77/84 v1 websubs store `channels` as an
**array** (v2 wants a map) and 6 carry top-level auth `policies` (no v2 field). Both are reshaped
losslessly (`channels[]`→map keyed by `request.name`; `policies`→`allChannels`), preserving channel
identity and auth credentials. See MIGRATION_MAPPING.md → "WebSub structural reshape". Spot-checked in v2:
`channels:{"_default":{}}`, `transport:["http","https"]`, `allChannels.on_subscription.policies:[basic-auth…]`.

## Operational checks

- **Idempotent/resumable:** re-running the live migration is a clean no-op (447 artifacts, 84 websub;
  `gateway_endpoints` existence-checked, no duplicates); handles replay from the file checkpoint.
- **Decrypt guard is real:** with a wrong key the run **aborts before any write** —
  *"decrypt guard FAILED (wrong key ⇒ all tokens garbage): … cipher: message authentication failed."*

## Sign-off items for a production cutover

1. Set `APIP_MIGRATION_ENCRYPTION_KEY` to the key v1 **actually used** for subscription tokens, and run
   **without** `-skip-decrypt-check` (the guard was skipped here only because choreo's key is not local).
2. Confirm the §C.1 assumption: v1 `idp_id` == the claim v2 extracts from the token (`sub`/configured) —
   not verifiable from the DB.
3. Accept the intentional drops and the `PLAINTEXT_CREDENTIAL` security note.

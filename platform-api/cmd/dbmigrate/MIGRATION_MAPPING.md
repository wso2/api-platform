# Platform API v1 → v2 DB Migration — Mapping & Decisions

**Tool:** `platform-api/cmd/dbmigrate` (one binary; subcommands `migrate` + `verify`).
**Build/deploy from the pinned v2 revision below** — the migrator compiles v2's
`internal/model` structs and column lists; struct/DDL skew silently corrupts blobs.

| | commit | branch | schema |
|---|---|---|---|
| **v2 (target)** | `a2911a091b549ee4b4bf57f895ce0ea09932ed2c` | `migration` | `internal/database/schema.postgres.sql` + `plugins/eventgateway/schema/schema.postgres.sql` |
| **v1 (source)** | `f4556fee3df0f4f894a1699e8dbd36d6762eaff7` | `platform-api/v0.10.x` | `src/internal/database/schema.postgres.sql` |

> **Why this tool was rebuilt (2026-08-22):** the prior run (`prod-20260818`) **dropped**
> all event APIs (84 `websub_apis` + 8 `webbroker_apis`) via the removed-feature path. Under
> the settled 2026-08-19 decision, event APIs are **first-class and always migrated** into the
> plugin tables, and the plugin DDL is always applied. This build migrates all six artifact
> types and never routes event APIs to the drop path.

---

## Deliverable 0 — mechanical DDL completeness gate → **PASS**

Parsed v1 (27 tables), v2 core (32), v2 plugin (3 → 35 v2 total). Every v2 table+column is
either **MAPPED** from v1 or explicitly **EMPTY/OPTIONAL**. Reproduce by extracting the
`CREATE TABLE` table+column sets from the three schema files and diffing them against the
classification below (any v2 object not listed is a blocker). Classification:

- **MAPPED (26):** organizations, projects, applications, artifacts, rest_apis,
  subscription_plans, subscription_plan_limits, subscriptions, gateways, gateway_endpoints,
  artifact_gateway_mappings, gateway_custom_policies, gateway_custom_policy_usages,
  gateway_tokens, deployments, deployment_status, llm_provider_templates, llm_providers,
  llm_proxies, mcp_proxies, api_keys, application_api_key_mappings,
  application_artifact_mappings, user_idp_references, **websub_apis**, **webbroker_apis**.
- **EMPTY, intentional (6):** gateway_states, events (runtime HA state); secrets,
  secret_scopes, artifact_secret_refs (v2-new); websub_api_hmac_secrets (v2-new, §K.3).
- **OPTIONAL, empty by default (3):** artifact_subscription_plans (dead in v2 code; config
  blob is source of truth — `-populate-artifact-subscription-plans` to derive rows),
  user_organization_mappings (audit resolution never reads it), audit
  (`-audit-marker` to emit one "migrated" marker per org).

**v1-only tables** (dropped or renamed): `application_api_keys`→`application_api_key_mappings`,
`application_artifacts`→`application_artifact_mappings`, `association_mappings`→
`artifact_gateway_mappings` (+dev_portal rows dropped), `devportals` (drop),
`publication_mappings` (drop).

---

## Architecture (as built)

Go, inside the v2 module so `internal/*` is importable. Stream v1 → transform → **own
parameterized `INSERT`s** into v2 (never reuse repo `Create*`, never emit a `.sql` file).
Reused verbatim (all exported, un-tagged — no `-tags experimental`):

- `internal/model` — config structs marshaled to the BYTEA blobs (byte-identical to v2 writes).
- `internal/utils` — `GenerateHandle`, `GenerateDeterministicUUIDv7`,
  `Encrypt/DecryptSubscriptionToken`, `DeriveEncryptionKey`.
- `internal/constants` — kind strings (`RestApi`/`WebSubApi`/`WebBrokerApi`/`LlmProvider`/
  `LlmProxy`/`Mcp`), throttle units (`MINUTE`/`HOUR`/`DAY`/`MONTH`), `DeletedUser`.
- `internal/database` — `NewConnection` (pgx stdlib via `database/sql`), `(*DB).InitSchemaSQL`
  (applies core **and** plugin DDL), `Rebind` (`?`→`$n`), `IsDuplicateKeyError`.

All migration state is **file-based** (checkpoint, handle map, quarantine, flags, drops,
reports) — the v2 DB contains only migrated data conforming to the v2 DDL.

---

## Type conversions (applied consistently)

| conversion | columns |
|---|---|
| JSONB → BYTEA (`json.Marshal(model.X)`, plaintext) | `*_config`/`configuration`, gateway `properties`/`manifest`, event `configuration` |
| TEXT → BYTEA | `api_keys.api_key_hashes`, llm `openapi_spec`/`model_list`, `llm_provider_templates.configuration`, `deployments.metadata`, `gateway_custom_policies.policy_definition` |
| TIMESTAMP → TIMESTAMPTZ | all `created_at`/`updated_at`/`*_at` (source TZ = `-source-tz`, default UTC; never `now()`) |
| BOOLEAN → SMALLINT (`true`→1) | `gateways.is_active`/`is_critical`, `subscription_plans.stop_on_quota_reach` |
| VARCHAR widen (safe) | `subscription_token_hash` 64→255, `gateway_custom_policies.version` 15→30 |
| VARCHAR narrow (flag if truncating) | all `handle` 255→40, `created_by` 255→200, `gateway_custom_policies.description` TEXT→1023, `api_keys.issuer`/`allowed_targets` TEXT→255 |

## Config-blob structural reshapes

- **RestApi / WebSub / WebBroker:** v1 `transport VARCHAR(255)` (JSON array as TEXT) is a
  *column*; v2 moved it *into* the config as `transport []string`. → parse the v1 column,
  `json.Unmarshal` v1 `configuration` into the v2 struct, set `cfg.Transport`, re-marshal.
  In-config `name` key is **preserved** (only the outer column is renamed to `display_name`).
- **LLM Provider/Proxy, MCP:** v1 `Policies` preserved; new arrays (`GlobalPolicies`/
  `OperationPolicies`/`AdditionalProviders`) left empty. Re-marshal v1 config into v2 struct.
- **Gateway:** `properties`/`manifest` `json.Marshal(map)`; `vhost` → one `gateway_endpoints`
  row (`url=vhost`); `display_name` carried from v1 `display_name`.

### ⚠ WebSub structural reshape (discovered from real data — the "strict subset" premise was WRONG)

The dry-run against production data disproved the assumption that the v1 WebSub config is a strict
subset of v2's (only `transport` added). Two **structural** differences exist and are handled
losslessly in `preprocessWebSubRaw` (77 of 84 websubs would otherwise fail `BLOB_UNPARSEABLE`):

1. **`channels`: v1 ARRAY → v2 `map[string]WebSubChannel`.** v1 stores
   `[{"request":{"name":"/issues","method":"SUBSCRIBE"}}]`; v2 wants a map keyed by channel
   name (verified: `mapWebSubChannelsAPIToModel` keys by `name` verbatim, no slash-stripping).
   → key = `request.name`, value = empty `WebSubChannel{}`. `method` is always `SUBSCRIBE`
   (constant, carries no info) → dropped safely.
2. **top-level `policies` (legacy) → `allChannels`.** v2's struct has no top-level policy list,
   but 6 APIs store real **auth** policies there (`basic-auth`/`api-key-auth`, with credentials).
   Folded into `allChannels`: object-form `{event:[p]}` → `allChannels.event.policies`; flat
   array `[p]` (whole-API auth) → `allChannels.on_subscription.policies`. None of the 6 had a
   pre-existing `allChannels`, so no merge conflict. **Zero policy loss.**

Both remaps are recorded per row as `SYNTHESIZED` flags (`structural_reshape`). WebBroker needs
no such reshape (its `channels` are already objects). This is the prompt-vs-data reconciliation
the working method calls for: **the data wins.**

**Blob guard (three-part, §E):** (1) dry-run `Decoder.DisallowUnknownFields()` discovery lists
v1 config fields the v2 struct drops (per-field human sign-off; see §E list below); (2) reshape;
(3) FULL per-row canonical-projection round-trip in `verify` (never sampled).

---

## Per-table mapping (FK order)

| # | v1 source | v2 target(s) | key transforms |
|---|---|---|---|
| 1 | (audit identities) | **user_idp_references** | distinct `created_by` strings + `migration` actor → `uuid=GenerateDeterministicUUIDv7(idp_id, epoch)`; `ON CONFLICT(idp_id)` |
| 2 | organizations | organizations | `name`→display_name; handle carry 255→40; +`idp_organization_ref_uuid`=uuid (PLACEHOLDER_IDP); audit rewrite |
| 3 | projects | projects | **generate** handle from name; `name`→display_name |
| 4 | applications | applications | handle carry; `name`→display_name; project_uuid now nullable |
| 5 | artifacts (all 6 kinds) | artifacts | →(uuid,type,org); `kind`→`type` verbatim; carry handle/name/version/created_at/updated_at DOWN into per-type row |
| 6 | rest_apis ⋈ artifacts | rest_apis | transport col→blob; JSONB→BYTEA; lifecycle_status carried |
| 7 | llm_provider_templates | llm_provider_templates | handle 255→40; `name`→display_name; config TEXT→BYTEA; +group_id=handle, version=v1.0, managed_by=organization, is_latest=1, enabled=1, openapi_spec=NULL |
| 8 | llm_providers ⋈ artifacts | llm_providers | JSONB→BYTEA; openapi_spec/model_list TEXT→BYTEA; **status DROPPED** |
| 9 | llm_proxies ⋈ artifacts | llm_proxies | JSONB→BYTEA; openapi_spec TEXT→BYTEA; **status DROPPED** |
| 10 | mcp_proxies ⋈ artifacts | mcp_proxies | JSONB→BYTEA; **status DROPPED** |
| 11 | websub_apis ⋈ artifacts | **websub_apis (plugin)** | artifacts-split like rest_apis; transport→`WebSubAPIConfiguration.transport`; lifecycle_status carried |
| 12 | webbroker_apis ⋈ artifacts | **webbroker_apis (plugin)** | same; `WebBrokerAPIConfiguration` |
| 13 | subscription_plans | subscription_plans (+ subscription_plan_limits) | generate handle from plan_name; `plan_name`→display_name; throttle→limits row (unit CASE-CONVERT, limit_type=REQUEST_COUNT, time_amount=1, limit_count_unit=NULL); **billing_plan DROPPED** |
| 14 | subscriptions | subscriptions | `api_uuid`→`artifact_uuid`; token verbatim (+decrypt guard); hash 64→255; partial-index dup → DUP_KEY quarantine |
| 15 | gateways | gateways (+ gateway_endpoints) | generate handle from name; display_name carried; properties/manifest JSONB→BYTEA; is_active/is_critical bool→smallint; vhost→endpoints row |
| 16 | gateway_custom_policies | gateway_custom_policies | policy_definition JSONB→BYTEA; description→1023 (TRUNCATED); version 15→30; name/display_name kept |
| 17 | gateway_tokens | gateway_tokens | hashes verbatim; TS→TSTZ; +data_version/created_by/revoked_by |
| 18 | deployments | deployments | `deployment_id`→uuid; `name`→display_name; `base_deployment_id`→base_deployment_uuid; metadata TEXT→BYTEA; content verbatim |
| 19 | deployment_status | deployment_status | `deployment_id`→deployment_uuid; PK reorder; +performed_by |
| 20 | association_mappings (gateway) | artifact_gateway_mappings | dev_portal rows DROPPED; metadata=NULL |
| 21 | gateway_custom_policy_usages | gateway_custom_policy_usages | `api_uuid`→`artifact_uuid` |
| 22 | api_keys | api_keys | generate handle from name; display_name=name; api_key_hashes TEXT→BYTEA; issuer/allowed_targets→255 (TRUNCATED) |
| 23 | application_api_keys | application_api_key_mappings | rename; +created_by |
| 24 | application_artifacts | application_artifact_mappings | rename; +created_by |

Dropped (report per row): devportals, publication_mappings, dev_portal associations,
subscription_plans.billing_plan, llm_providers/llm_proxies/mcp_proxies `status`,
gateway_states, events.

---

## Decisions requiring human sign-off (defaults chosen)

1. **`idp_organization_ref_uuid` = org's own uuid** (PLACEHOLDER_IDP flag). Backfill from IdP later.
2. **`llm_provider_templates.group_id` = template's v1 handle** (matches v2 create path).
3. **Subscription token = verbatim passthrough** + **mandatory fatal decrypt guard**. Operator
   must set v2 `security.encryption_key` to the value v1 actually used
   (`database.subscription_token_encryption_key`, else `auth.jwt.secret_key`). `-skip-decrypt-check`
   only for dry-runs without the key.
4. **Dropped features → drop + report** (list above). Event APIs are **not** in this bucket.
5. **Audit identity (§C.1):** seed `user_idp_references` from distinct v1 audit strings +
   a `migration` actor; rewrite every `created_by`/`updated_by`/`revoked_by`/`performed_by`
   to the mapped UUID. **Assumption (unverifiable from DB):** v1 `idp_id` == the claim v2
   extracts from the token (`sub`/configured). If v1 stored a username but v2 auths on `sub`,
   the seeded identity displays correctly but won't unify with that user's future live-login UUID.
6. **Event APIs → plugin tables**, plugin DDL applied every run.
7. **`websub_api_hmac_secrets` = empty** (v2-new; re-issue post-migration). No `APIP_CP_ENCRYPTION_KEY` needed.
8. **`artifact_subscription_plans` = empty by default** (dead in v2 code). `-populate-artifact-subscription-plans` to derive.
9. **`audit` = empty by default.** `-audit-marker` to emit one per org.
10. **Source TZ = UTC** (`-source-tz`).

## §E dropped-field discovery (per-config-struct, filled by `migrate -dry-run`)

> Populated from the `DisallowUnknownFields` pass in the dry-run. Any field that carries data
> (not obsolete) is a STOP-and-remap. Event configs are a strict subset of v2 → expected **zero**
> unknowns. See `migration-report-<run_id>-dryrun.json → dropped_config_fields`.

## Source-data-quality lanes (fail-fast / quarantine / migrate-with-flag)

Per the prompt's table. Quarantine (`quarantine-<run_id>.jsonl`) preserves the full v1 row +
`reason_code` ∈ {ORPHAN_FK, BLOB_UNPARSEABLE, NULL_REQUIRED, HANDLE_UNRESOLVABLE, DUP_KEY};
flags (`flags-<run_id>.jsonl`) record migrate-with-flag events {DEFAULTED_NULL, TRUNCATED,
PLACEHOLDER_IDP, SYNTHESIZED, PLAINTEXT_CREDENTIAL}. `verify` gate per table:
`count(v2) + drops + quarantine(latest run) == count(v1)` AND every quarantined key resolved or
signed off (`quarantine-signoff.jsonl`).

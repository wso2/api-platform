# Bug report — subscriptions are silently dropped for gateway-originated APIs

## 1. Title

**Subscriptions are silently discarded for any API that originated on the gateway (dp→cp synced),
because the subscription snapshot matches on the gateway's own UUID and never falls back to
`cp_artifact_id`**

Shorter variant for the tracker:

> Gateway drops subscriptions for dp→cp synced APIs — snapshot filter ignores `cp_artifact_id`

---

## 2. Description

### Summary

A subscription created in the developer portal for an API that was **created directly on a
gateway** (and reached the control plane via the bottom-up dp→cp push) is accepted and stored by
the control plane, broadcast to the gateway, received by the gateway — and then silently dropped.
The API continues to answer:

```
403 {"error":"forbidden","message":"Subscription required for this API"}
```

while the portal shows the subscription as ACTIVE and the control plane's database holds it as
ACTIVE. Nothing in any log says it was discarded.

### Root cause

The two planes deliberately identify the same API by **different UUIDs**, and the broadcast event
carries the one the gateway does not index by.

- `platform-api/internal/service/artifact_import.go:51-56` states the design explicitly — the CP
  mints its own identifier and the pushed data-plane UUID is retained only as `DPID`,
  *"informational (logging/traceability) and … never used as the CP artifact UUID."* Re-push
  matching is by **handle**, not id.

- `platform-api/internal/service/subscription_service.go:232-239` broadcasts the **control
  plane's** UUID:

  ```go
  event := &model.SubscriptionCreatedEvent{
      ApiId:              sub.ArtifactUUID,   // <-- CP UUID
      SubscriptionId:     sub.UUID,
      ...
  }
  ```

- `gateway/gateway-controller/pkg/subscriptionxds/subscription_snapshot.go:115-128` builds its
  accept-set from the gateway's **own** config UUIDs and drops anything else without a word:

  ```go
  // Build set of API IDs that exist in configs (RestApi kind only).
  apiIDs := make(map[string]bool)
  for _, cfg := range configs {
      if cfg != nil && cfg.Kind == "RestApi" {
          apiIDs[cfg.UUID] = true          // gateway UUID only
      }
  }

  for apiID, list := range subsByAPI {
      if !apiIDs[apiID] {
          continue                          // <-- silent drop, no log
      }
  ```

For a **CP-originated** API the two ids agree, because the gateway's config was created from the
control plane's deployment — so this path works and masks the bug. For a **gateway-originated**
API they cannot agree: the control plane minted a fresh UUID at import, by design.

### The fix already exists in this codebase, one package over

This is the decisive point. The API-key path hits the identical dual-UUID problem and handles it
correctly — `gateway/gateway-controller/pkg/utils/api_key.go:1910-1928`:

```go
func (s *APIKeyService) getArtifactConfigByID(artifactUUID string) (*models.StoredConfig, error) {
	cfg, err := s.db.GetConfig(artifactUUID)
	if err == nil {
		return cfg, nil
	}
	...
	// Fallback: incoming UUID may be the APIM/control-plane UUID for a bottom-up
	// synced artifact. Look it up by cp_artifact_id.
	cfg, err = s.db.GetConfigByCPArtifactID(artifactUUID)
```

`GetConfigByCPArtifactID` is implemented at `pkg/storage/sql_store.go:1310`
(`WHERE gateway_id = ? AND cp_artifact_id = ?`), the column is populated after a successful
sync (`pkg/models/stored_config.go:107`), and the gateway's own IT suite already asserts it is
written (`gateway/it/features/dp-to-cp.feature:168`). The subscription snapshot is simply the one
consumer that never consults it.

### Secondary defects found alongside it

1. **The drop is silent, and inconsistently so.** In the *same loop*, a subscription referencing a
   missing *plan* logs a warning and aborts the snapshot update
   (`subscription_snapshot.go:144`), while one referencing an unknown *API* just `continue`s. A
   subscription discarded for an unknown API should not be quieter than one discarded for an
   unknown plan.

2. **`Kind == "RestApi"` excludes MCP APIs entirely.** Line 118 filters the accept-set to
   `RestApi` only, so a subscription against an `Mcp` artifact is dropped *even when the UUIDs
   match*. This is a separate bug on the same line and should be confirmed independently.

### Impact

Subscription validation and per-subscription rate limiting are **unavailable for any API created
directly on a gateway** — that is, the entire gateway-first workflow. The failure is silent on
every plane: portal shows active, control plane stores active, only the data plane's 403
disagrees, and no log explains it. An operator has no signal to diagnose this short of reading
the snapshot source.

### Environment

- Reproduced from source analysis against `api-platform` @ `VERSION` = gateway `1.2.0-SNAPSHOT`.
- Chain verified end-to-end (portal → CP accept 200 → broadcast → gateway receive) in the
  v2 integration harness; the drop is the final hop.

---

## 3. Steps to reproduce

### Preconditions

A running platform-api (control plane), a developer portal linked to it, and a gateway registered
to that control plane with dp→cp sync **enabled** and a shared webhook secret configured.

### Steps

1. **Create a RestApi directly on the gateway** — not through the control plane. This is the
   gateway-first path:

   ```bash
   curl -u admin:admin -X POST http://<gateway>:9443/api/management/v0.9/apis \
     -H 'Content-Type: application/yaml' --data-binary @- <<'YAML'
   apiVersion: gateway.api-platform.wso2.com/v1
   kind: RestApi
   metadata:
     name: sub-drop-repro
   spec:
     displayName: Subscription Drop Repro
     version: v1.0
     context: /sub-drop/$version
     upstream:
       main:
         url: http://backend:80
     securityScheme:
       - subscription        # forces subscription validation on
     operations:
       - method: GET
         path: /get
   YAML
   ```

2. **Wait for the bottom-up push to complete.** Confirm the gateway recorded a control-plane id
   for it — this is the value the snapshot needs and never reads:

   ```sql
   -- gateway controller DB
   SELECT uuid, handle, cp_sync_status, cp_artifact_id
   FROM artifacts WHERE handle = 'sub-drop-repro';
   ```

   Expect `cp_sync_status = 'synced'` and a **non-null `cp_artifact_id` that differs from
   `uuid`**. That difference is the bug's precondition.

3. **Confirm the control plane minted its own UUID:**

   ```sql
   -- platform-api DB
   SELECT uuid, handle, dpid FROM artifacts WHERE handle = 'sub-drop-repro';
   ```

   `uuid` ≠ the gateway's `uuid`; `dpid` holds the gateway's.

4. **In the developer portal**, create an application, then subscribe it to
   *Subscription Drop Repro* against any subscription plan. The portal reports success.

5. **Observe the chain succeed at every hop** — portal delivery, CP acceptance, broadcast, gateway
   receipt — and then produce an empty snapshot:

   ```
   portal    Delivered {"eventType":"subscription.created","status":200}
   gateway   Received WebSocket event type=subscription.created
   gateway   Processing replica sync event event_type=SUBSCRIPTION action=CREATE
   gateway   Subscription snapshot updated successfully subscription_count=0   <-- dropped here
   ```

6. **Invoke the API with the subscription token:**

   ```bash
   curl -i http://<gateway>:8080/sub-drop/v1.0/get \
     -H "Authorization: Bearer <subscription-token>"
   ```

### Actual result

```
HTTP/1.1 403 Forbidden
{"error":"forbidden","message":"Subscription required for this API"}
```

`subscription_count=0` in the snapshot log. No warning, no error, on either plane.

### Expected result

`HTTP/1.1 200 OK` — the subscription is ACTIVE in the control plane and was delivered to and
accepted by the gateway, so the data plane should honour it. Per-subscription rate limiting should
likewise apply.

### Control (proves this is dp-origin specific, not subscriptions generally)

Repeat the whole sequence but **create the API on the control plane and deploy it down** to the
gateway. The subscription is honoured and the invoke returns `200`. The only difference is which
plane minted the UUID.

---

## Suggested fix

Consult the mapping the gateway already stores, mirroring `getArtifactConfigByID`:

```go
apiIDs := make(map[string]bool)
for _, cfg := range configs {
    if cfg == nil || cfg.Kind != "RestApi" {   // see secondary defect 2 re: Mcp
        continue
    }
    apiIDs[cfg.UUID] = true
    if cfg.CPArtifactID != "" {
        apiIDs[cfg.CPArtifactID] = true        // accept the control plane's id too
    }
}
```

`CPArtifactID` is already on `models.StoredConfig` and already populated post-sync, so this needs
no schema change and no event-format change. Alternatives considered: carry `DPID` in the
broadcast event (changes the event contract, and the CP would need to know which gateway), or
match on handle (weaker — handles are not globally unique across gateways).

Regardless of which is chosen, **log the drop**. Make the unknown-API branch at least as loud as
the neighbouring unknown-plan branch.

## Test coverage this unblocks

`tests/framework/suites/it/features/subscription_validation.feature` and the
`gateway-subscriptions` block (currently commented out in `it-suite.yaml:556-592`). The block's
topology boots and the full chain is verified working up to this last hop; it is held back only
because this defect would make it permanently red.

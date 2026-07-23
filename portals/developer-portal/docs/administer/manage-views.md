# Manage Views

A view is a filtered, branded subset of an organization's APIs. An organization can have multiple views — for example, `public` for external developers and `internal` for internal teams — each showing only the APIs tagged with the relevant labels.

Each view has its own URL:

```
https://<host>/<orgHandle>/views/<viewName>
```

## Create a View

> **Authentication:** The examples below use a `$TOKEN` variable. Obtain a Bearer token first:
> ```bash
> TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
>   -d "username=<admin-username>&password=<admin-password>" | jq -r .token)
> ```

```json
// view.json
{
  "id": "internal",
  "displayName": "Internal Developer Portal",
  "labels": ["internal", "platform"]
}
```

```bash
curl -k -X POST https://localhost:3000/api/v0.9/views \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @view.json
```

| Field | Required | Description |
|---|---|---|
| `id` | Yes | URL-safe identifier used in the view URL (lowercase, no spaces) |
| `labels` | Yes | List of label names (at least one); only APIs with at least one matching label appear in this view |
| `displayName` | No | Human-friendly name shown in the portal header. Defaults to `id` if omitted |

## List Views

```bash
curl -k https://localhost:3000/api/v0.9/views -H "Authorization: Bearer $TOKEN"
```

## Get a View

```bash
curl -k https://localhost:3000/api/v0.9/views/{viewId} -H "Authorization: Bearer $TOKEN"
```

## Update a View

The update request takes the full desired label set via `labels` — labels present in the list are attached and any others currently attached are detached. Omit `labels` to leave the view's labels unchanged:

```json
// view-update.json
{
  "displayName": "Internal Developer Portal v2",
  "labels": ["internal", "platform", "experimental"]
}
```

```bash
curl -k -X PUT https://localhost:3000/api/v0.9/views/{viewId} \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @view-update.json
```

## Delete a View

```bash
curl -k -X DELETE https://localhost:3000/api/v0.9/views/{viewId} -H "Authorization: Bearer $TOKEN"
```

---

## Manage Labels

Labels are tags you assign to APIs to control which views they appear in. An API with the label `internal` will only appear in views that include `internal` in their label list.

### Create a Label

Labels are created one at a time as a JSON object:

```json
// label.json
{
  "id": "internal",
  "displayName": "Internal"
}
```

```bash
curl -k -X POST https://localhost:3000/api/v0.9/labels \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @label.json
```

| Field | Required | Description |
|---|---|---|
| `id` | Yes | URL-safe label identifier (lowercase, no spaces), stored as-is |
| `displayName` | Yes | Human-friendly label name shown in the portal UI |

### List Labels

```bash
curl -k https://localhost:3000/api/v0.9/labels -H "Authorization: Bearer $TOKEN"
```

### Get a Label

```bash
curl -k https://localhost:3000/api/v0.9/labels/{labelId} -H "Authorization: Bearer $TOKEN"
```

### Update a Label

```json
// label-update.json
{
  "id": "internal",
  "displayName": "Internal Teams"
}
```

```bash
curl -k -X PUT https://localhost:3000/api/v0.9/labels/{labelId} \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @label-update.json
```

### Delete a Label

```bash
curl -k -X DELETE "https://localhost:3000/api/v0.9/labels/{labelId}" \
  -H "Authorization: Bearer $TOKEN"
```

> **Note:** Deleting a label does not remove it from APIs that already have it assigned. Update each API to remove the label reference if needed.

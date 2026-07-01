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
>   -d "username=admin&password=admin" | jq -r .token)
> ```

```yaml
# view.yaml
name: internal
displayName: Internal Developer Portal
labels:
  - internal
  - platform
```

```bash
curl -X POST http://localhost:3000/devportal/v1/views \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @view.yaml
```

| Field | Required | Description |
|---|---|---|
| `name` | Yes | URL-safe identifier used in the view URL (lowercase, no spaces) |
| `labels` | Yes | List of label names; only APIs with at least one matching label appear in this view |
| `displayName` | No | Human-friendly name shown in the portal header. Defaults to `name` if omitted |

## List Views

```bash
curl http://localhost:3000/devportal/v1/views -H "Authorization: Bearer $TOKEN"
```

## Get a View

```bash
curl http://localhost:3000/devportal/v1/views/{name} -H "Authorization: Bearer $TOKEN"
```

## Update a View

The update request uses incremental label changes via `addedLabels` and `removedLabels` rather than replacing the full label list:

```yaml
# view-update.yaml
displayName: Internal Developer Portal v2
addedLabels:
  - experimental
removedLabels: []
```

```bash
curl -X PUT http://localhost:3000/devportal/v1/views/{name} \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @view-update.yaml
```

## Delete a View

```bash
curl -X DELETE http://localhost:3000/devportal/v1/views/{name} -H "Authorization: Bearer $TOKEN"
```

---

## Upload a Custom Layout

A layout is a set of Handlebars (`.hbs`) template files that define the page structure for a view. Upload a custom layout to give a view its own branding independent of the theme color settings.

```bash
curl -X POST "http://localhost:3000/devportal/v1/views/{name}/layout" \
  -H "Authorization: Bearer $TOKEN" \
  -F "zipFile=@my-layout.zip"
```

The ZIP file should contain `.hbs` template files following the portal's page structure (see [Theming](theming/org-level-theming.md) for details on the template format).

To update an existing layout:

```bash
curl -X PUT "http://localhost:3000/devportal/v1/views/{name}/layout" \
  -H "Authorization: Bearer $TOKEN" \
  -F "zipFile=@my-layout-v2.zip"
```

To remove a custom layout and revert to the default:

```bash
curl -X DELETE "http://localhost:3000/devportal/v1/views/{name}/layout/template" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Manage Labels

Labels are tags you assign to APIs to control which views they appear in. An API with the label `internal` will only appear in views that include `internal` in their label list.

### Create Labels

Labels are submitted as a JSON array (no YAML format for labels):

```json
// labels.json
[
  {"name": "internal", "displayName": "Internal"},
  {"name": "partner", "displayName": "Partner"}
]
```

```bash
curl -X POST http://localhost:3000/devportal/v1/labels \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @labels.json
```

| Field | Required | Description |
|---|---|---|
| `name` | Yes | URL-safe label identifier (lowercase, no spaces) |
| `displayName` | Yes | Human-friendly label name shown in the portal UI |

### List Labels

```bash
curl http://localhost:3000/devportal/v1/labels -H "Authorization: Bearer $TOKEN"
```

### Update Labels

```json
// labels-update.json
[
  {"name": "internal", "displayName": "Internal Teams"}
]
```

```bash
curl -X PUT http://localhost:3000/devportal/v1/labels \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @labels-update.json
```

### Delete a Label

```bash
curl -X DELETE "http://localhost:3000/devportal/v1/labels?labelName=internal" \
  -H "Authorization: Bearer $TOKEN"
```

> **Note:** Deleting a label does not remove it from APIs that already have it assigned. Update each API to remove the label reference if needed.

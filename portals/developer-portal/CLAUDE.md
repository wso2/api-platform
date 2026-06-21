# Developer Portal — Claude Code Guide

## Project overview

Express.js + Handlebars (`.hbs`) server-rendered developer portal for WSO2 API Manager.
Two distinct UI surfaces share the same server:

| Surface | Templates | CSS |
|---|---|---|
| **Developer portal** (public-facing) | `src/defaultContent/pages/` | `src/defaultContent/styles/` |
| **Portal management / admin** | `src/pages/` | `src/styles/` |

These are separate concerns — do not mix their CSS files.

## CSS architecture

### Developer portal (`src/defaultContent/styles/`)

```
main.css
 ├── @import components.css   ← canonical shared component library
 ├── @import home.css
 ├── @import footer.css
 ├── @import header.css
 ├── @import api-listing.css
 ├── @import api-content.css
 ├── @import api-landing.css
 ├── @import doc.css
 ├── @import side-bar.css
 └── @import default-api.css
```

**Standalone page CSS files** (loaded on their own route, not via `main.css`) must start with:
```css
@import "/styles/components.css";
```
Affected files: `subscriptions.css`, `login.css`, `os.css`.
This means `components.css` classes (`.page-header`, `.dp-btn`, `.dp-breadcrumb`, `.dp-empty`, etc.) are always available everywhere.

### Never touch
`src/defaultContent/styles/async-tryout.css` and `src/styles/async-tryout.css` — third-party bundles.

## Design tokens (CSS custom properties)

Defined in `main.css` `:root`:

```css
--wso2-gradient: linear-gradient(135deg, #ef4223 0%, #ff8636 100%);
--primary-gradient: linear-gradient(135deg, #3d6b8a, #1c3b52);
--primary-main-color: #1A4C6D;
--border-radius: 0.5rem;
--font-family-sans: "Montserrat", "Quicksand", "Noto Sans", "Poppins", sans-serif;
```

## CSS conventions

- **Units**: `rem` for all sizes (divide px by 16). Exception: `border-width` and `box-shadow` stay in `px`.
- **Component prefix**: shared components use `dp-*` (e.g. `dp-btn`, `dp-breadcrumb`, `dp-empty`).
- **No debug borders**: never commit `border: 1px solid red`.

## Canonical shared components (`components.css`)

### Page header
```css
.page-header   /* flex row, space-between, gap: 16px, margin-bottom: 26px */
.page-title    /* 1.5rem, font-weight: 700, color: #1a2433 */
.page-desc     /* 0.875rem, line-height: 1.5, color: #637282 */
```
Use these global classes everywhere. Do not invent page-specific header classes (e.g. `sub-page-header`, `apps-page-header` are legacy — migrate away when touching those pages).

### Breadcrumb
```hbs
<nav class="dp-breadcrumb">
  <a class="dp-breadcrumb-item" href="...">Parent</a>
  <i class="bi bi-chevron-right dp-breadcrumb-sep"></i>
  <span class="dp-breadcrumb-current">Current Page</span>
</nav>
```
Top-level pages (APIs, MCP Servers, Subscriptions, API Workflows) have no parent — omit breadcrumb there.

### Buttons
```css
.dp-btn              /* base */
.dp-btn--primary     /* wso2-gradient fill */
.dp-btn--secondary   /* outlined */
.dp-btn--icon        /* square icon-only */
```

### Empty state
```css
.dp-empty        /* flex column, centered, padding: 3.75rem 0 5rem, NO border */
.dp-empty-icon
.dp-empty-title
.dp-empty-desc
```

## Hard rules

1. **Unimplemented action buttons** must always have:
   ```html
   disabled title="Backend not wired yet — UI preview only"
   ```

2. **Do not change any file without asking first** unless it's the file directly under discussion.

3. **Breadcrumb on sub-pages**: all API/MCP detail sub-pages (subscriptions, keys, docs, flows detail) must have a breadcrumb.

## Handlebars template patterns

- `{{baseUrl}}` — portal base URL available in all templates
- `{{apiMetadata.apiHandle}}` — API/MCP handle (slug) for URL construction
- `{{or apiMetadata.apiInfo.apiTitle apiMetadata.apiInfo.apiName}}` — display name pattern (title with name fallback)
- `{{apiName}}` — passed explicitly to docs pages from `apiContentController.js`
- `{{baseDocUrl}}` — API/MCP detail page URL, passed explicitly to docs pages

## Controller locations

| Controller | File |
|---|---|
| API/MCP docs pages | `src/controllers/apiContentController.js` |
| Application management views | `src/controllers/viewConfigureController.js` |

`loadDocsPage` and `loadDocument` in `apiContentController.js` each have a design-mode and non-design-mode path — template context variables must be added in all four locations.

## Key page files

| Page | Template | CSS |
|---|---|---|
| API listing | `src/defaultContent/pages/apis/page.hbs` | `api-listing.css` |
| API detail | `src/defaultContent/pages/api-landing/page.hbs` | `api-landing.css` |
| API subscriptions | `src/defaultContent/pages/api-subscriptions/page.hbs` | `api-content.css` |
| API keys | `src/defaultContent/pages/api-keys/page.hbs` | `api-content.css` |
| MCP landing | `src/defaultContent/pages/mcp-landing/page.hbs` | `api-landing.css` |
| Docs | `src/defaultContent/pages/docs/page.hbs` | `doc.css` |
| Subscriptions | `src/defaultContent/pages/subscriptions/page.hbs` | `subscriptions.css` |
| API Flows | `src/defaultContent/pages/api-flows/page.hbs` | (via main.css) |
| Applications | `src/pages/applications/partials/applications-listing.hbs` | `src/styles/applications.css` |

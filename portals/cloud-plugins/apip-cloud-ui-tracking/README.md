# `apip-cloud-ui-tracking`

Loads WSO2 Cloud's CookiePro banner and Google Tag Manager container in the API Control Plane console. Cloud only, so it is a plugin: the open-source console carries none of it. Registered in `apip-cloud-ui/src/hosts/api-control-plane.tsx` as an `app.gate` entry (the AI Workspace host registers nothing).

WSO2 Cloud uses one CookiePro account and one GTM container for all its consoles; GA4 and Microsoft Clarity are set up inside the container. This mirrors the WSO2 Cloud console's setup (wso2cloud PR #2481).

## Configuration

Set in the console's runtime config (`window.__RUNTIME_CONFIG__`), by the deployment:

| Key | Value |
| --- | --- |
| `cookieProDomainScript` | CookiePro domain script id: one per domain and environment (`-test` suffix on dev) |
| `gtmContainerId` | `GTM-XXXX` |

Both are required and format-checked. The footer's **Cookie Policy** link is not part of this plugin: the console shows it when its own `cookiePolicyLink` runtime-config key is an `https://` URL (WSO2 Cloud sets `https://choreolabs.com/cookie-policy/`). With neither the plugin does nothing; with only one it logs a warning and stays off, since a container without CookiePro could never be granted consent.

## Behaviour

- Google consent defaults (`ad_storage`, `ad_user_data`, `ad_personalization`, `analytics_storage`) are set to denied before CookiePro or GTM load. A returning visitor starts from their saved choice in the `OptanonConsent` cookie.
- `OptanonWrapper` passes the visitor's choice to Google consent mode on load and on every change: Performance (`C0002`) grants `analytics_storage`, Targeting (`C0004`) the three ad settings. CookiePro's own Google Consent Mode integration is off in the WSO2 account.
- CookiePro's floating cookie icon is hidden (a constructable stylesheet, so the CSP's no-inline-styles rule does not matter). The console's footer shows a **Cookie Settings** link whenever both keys are set; it calls `OneTrust.ToggleInfoDisplay()` to open the same Cookie Preference Center.
- CookiePro auto-blocking (`OtAutoBlock.js`) is not loaded, as in the Cloud console.
- `<body data-clarity-mask="true">` masks all page text in Clarity recordings.
- Each route change pushes a `virtual_page_view` to the data layer with the route **pattern** (`/organizations/:orgHandle/home`), no query or hash, the origin only of an outside referrer, `product: api-platform` and a fixed `page_title`. Redirect-only routes (`/`, the organization picker, the landing redirect) are not counted. Paths no route matches are sent as `/(not-found)`. Patterns come from the console's `routes` table; a plugin-added page is reported as `/(not-found)`.

The gate mounts on in-shell (signed-in) routes, so nothing loads on the sign-in screens, which the gateway serves.

## Needed outside this package

- The deployment sets the keys (from the same `cookiepro_domain_script` and `gtm_container_id` cluster variables the other WSO2 Cloud consoles read, so one cookie choice covers consoles on a shared origin) and allows the hosts in the console's Content-Security-Policy: `script-src`, `style-src` and `connect-src` for GTM, CookiePro, `geolocation.onetrust.com`, GA4 and Clarity. The policy blocks inline scripts, so a Custom HTML tag in the container stays blocked.
- Before enabling a container: every non-Google tag in it (Clarity included) must require `analytics_storage` in its GTM consent settings, and the GA4 tag must fire on `virtual_page_view` reading `page_location`, `page_path`, `page_referrer` and `page_title` from the event, with automatic and history page views off.

## Tests

`npm test` (vitest, jsdom), or as the CI cloud-plugins step does: `vitest run --config ../vitest.shared.config.ts` against the console's `node_modules`.

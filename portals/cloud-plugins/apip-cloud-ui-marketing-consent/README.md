# `apip-cloud-ui-marketing-consent`

Asks a signed-in person **once** whether they want WSO2 marketing email, for the API Control Plane console only (registered in `apip-cloud-ui/src/hosts/api-control-plane.tsx`, not in the AI Workspace host).

## Behaviour

- `MarketingConsentGate` is an `app.gate` entry, mounted on every in-shell route. It reads what Pardot holds and opens the dialog only when the person never answered (`No Opt-in` or `Unconfirmed`) and has not opted out of all WSO2 email.
- **Subscribe** records `Confirmed`; **No thanks** records `Unsubscribed`. Closing the dialog (Esc or a click outside) records nothing and postpones it for the browser session.
- After a successful answer the prompt is also suppressed for the session. A `202` means *accepted, not delivered*, so an immediate re-read can still say `No Opt-in`.
- It never gets in the way: a failed read (403, 404, 502, timeout) shows nothing, and a failed save keeps the dialog open so the person can retry.

## Where the calls go

The browser calls `<proxy base>/marketing-consent/marketing-consent`, same-origin. The platform-api endpoint is internal-gateway-only, so a browser cannot reach it directly:

- **`external` auth mode (WSO2 Cloud):** the control-plane gateway's `gateway-bff-api-management-marketing-consent` RestApi (`/_api/api-management/marketing-consent`, in the deployment repo) swaps the session cookie for the person's **own** access token and forwards to the internal endpoint. A service token is refused with 403, and so is a token without an `email` claim.
- A 404 means marketing consent is not configured in that environment (`PARDOT_CONSENT_FORM_HANDLE` / `PARDOT_CONSENT_CHECK_URL`); the console then shows nothing.

## Layout

| File | Role |
| --- | --- |
| `MarketingConsentGate.tsx` | The dialog and its state |
| `consentApi.ts` | `fetchConsent` / `recordConsent`; never throw |
| `consentPrompt.ts` | When to ask (`shouldPrompt`) and the per-session suppress flag |
| `runtimeConfig.ts` | The proxy base and the consent URL |
| `hostPort.ts`, `types.ts` | The slice of the host Port used, and the consent shapes |

## Tests

`npm test` (vitest, jsdom). Like the other plugins this package has no `node_modules` in the monorepo checkout; `tsconfig.console.json` typechecks the tests against the console's dependencies.

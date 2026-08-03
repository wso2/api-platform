# API Control Plane

Oxygen UI console for the API Platform, replacing the legacy `choreo-console`.

## Scope

This app intentionally implements only the MVP core console:

- Auth shell and protected routes
- Organization and project navigation
- Project home
- Component list, create, and detail
- Deploy, test, manage, runtime logs, and minimal settings

Non-MVP legacy pages are hidden rather than ported.

## Development

```bash
cd portals/api-control-plane
npm install
npm run dev
```

The app is served under `/oxygen-console`.

## Validation

```bash
npm run typecheck
npm run test
npm run build
```

/*
 * Runtime config template for the deployed API Platform (Oxygen) console.
 *
 * Rendered to api-platform.env.config.js at container startup by
 * /docker-entrypoint.d/40-config-setup.sh (envsubst). Values come from the
 * environment variables set on the workload (see the OpenChoreo ReleaseBinding).
 * index.html loads the rendered file before React boots; src/config/runtime.ts
 * reads window.__RUNTIME_CONFIG__.
 */
window.__RUNTIME_CONFIG__ = {
  appBasePath: '${APP_BASE_PATH}',
  environmentName: '${ENVIRONMENT_NAME}',

  // ---- Auth: Platform IDP (Thunder) — OIDC authorization_code + PKCE ----
  authMode: 'thunder',
  authBaseUrl: '${AUTH_BASE_URL}',
  authClientId: '${AUTH_CLIENT_ID}',
  AUTH_SCOPES: '${AUTH_SCOPES}',
  THUNDER_CONFIG: {
    issuer: '${THUNDER_ISSUER}',
  },

  // ---- platform-api via apip-bml, through the OpenChoreo CP gateway ----
  // Host base only; the console appends /api/${PLATFORM_API_VERSION} (default
  // v0.9). To pin a different REST version, add a PLATFORM_API_VERSION line
  // here and set the matching env var on the workload.
  PLATFORM_API_BASE_URL: '${PLATFORM_API_BASE_URL}',

  // ---- billing user-api (first-login apip subscription activation) ----
  BILLING_SERVICE_URL: '${BILLING_SERVICE_URL}',
};

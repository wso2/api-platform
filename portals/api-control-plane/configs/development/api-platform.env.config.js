/*
 * Local dev runtime config for connecting the Oxygen console to a LOCAL
 * wso2cloud / OpenChoreo k3d cluster (Thunder IdP + api-platform).
 *
 * This file overrides the shared legacy `choreo.env.config.js` (which targets
 * the hosted Bijira dev environment) ONLY when the Vite dev server is started
 * with VITE_LOCAL_WSO2CLOUD=true — see vite.config.ts and configs/development.
 *
 * All cluster URLs are SAME-ORIGIN (https://localhost:3000/...) and forwarded
 * to the k3d gateway (127.0.0.1:19080) by the Vite dev proxy. That avoids
 * browser CORS + mixed-content (https page -> http gateway) entirely.
 */
window.__RUNTIME_CONFIG_PARAM__ = {
  DOMAIN: 'localhost:3000',
};

window.__RUNTIME_CONFIG__ = {
  // App is served under /oxygen-console (keep in sync with VITE_APP_BASE_PATH).
  appBasePath: '/oxygen-console',
  environmentName: 'local',

  // ---- Auth: Thunder (OIDC authorization_code + PKCE) ----
  authMode: 'local-file',
  // Same-origin base; Vite proxies /oauth2 -> Thunder. The adapter derives
  // /oauth2/{authorize,token,userinfo,jwks} from this base.
  authBaseUrl: 'https://localhost:3000/oauth2',
  authClientId: 'oxygen-console-local',
  AUTH_SCOPES: 'openid,profile,email',
  THUNDER_CONFIG: {
    // Thunder issues a bare-name issuer (NOT a URL); must match exactly.
    issuer: 'platform-idp',
  },

  // ---- platform-api (api-platform Go backend) ----
  // Same-origin host base; Vite proxies it to the gateway. The client appends
  // /api/${PLATFORM_API_VERSION} (default v0.9), so this is the prefix BEFORE
  // /api/v0.9. Set PLATFORM_API_VERSION here to target a different REST version.
  PLATFORM_API_BASE_URL: '/platform-api-service-platform-api-endpoint',

  // Shown in self-hosted gateway setup instructions only.
  GATEWAY_CONTROL_PLANE_HOST:
    'development-wso2cloud.openchoreoapis.localhost:19080',

  // Legacy GraphQL / hosted services are intentionally left unset so all read
  // and write flows go through platform-api (usePlatformApi() === true).
};

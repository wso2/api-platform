/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * What the console needs from whatever handles sign-in in front of it, and
 * where to find it.
 *
 * The console never holds a token. Something in front of it does: this
 * portal's own BFF, or a gateway or auth proxy that acts as the BFF (an API
 * Platform gateway with the `oauth2-session` policy, oauth2-proxy, …). The
 * console only needs three things from it, all same-origin and cookie-based:
 *
 *   - a way to sign in: a credentials form it posts to, or a URL it sends the
 *     browser to with a "come back to" parameter;
 *   - an endpoint that says who is signed in;
 *   - a way to sign out.
 *
 * `authMode` picks a preset for all of them, and each one can be overridden
 * from the runtime config. `basic` and `oidc` are this portal's BFF and are
 * what every existing deployment already sets, so their defaults are exactly
 * the BFF's routes: nothing changes for them. `external` is for a gateway or
 * auth proxy in front of the console, with no BFF.
 */

export type AuthMode = 'basic' | 'oidc' | 'external';

/** `form`: the console posts a username and password. `redirect`: it sends the browser away. */
export type LoginStyle = 'form' | 'redirect';

/**
 * `bff`: the session endpoint answers `{authenticated, user}` with the user
 * already resolved, as this portal's BFF does. `claims`: it answers with the
 * signed-in user's claims. `sdk`: it answers the ThunderID SDK's backend-mode
 * session, `{isSignedIn, user, expiresAt, scope}`, with the user's claims in
 * `user` (as the gateway's `oauth2-session` policy does) and the granted scopes,
 * space-separated, in `scope` (`@thunderid/browser` 1.9.3 and later read it
 * too). For `claims` and `sdk` the console resolves the user itself through
 * `claimMappings`. Any non-2xx answer means "not signed in" in every format.
 */
export type SessionFormat = 'bff' | 'claims' | 'sdk';

/**
 * `post-json`: POST to the logout endpoint, then follow the `logoutUrl` it
 * returns, if any (the BFF's contract). `navigate`: send the browser to the
 * logout URL, which ends the session and redirects.
 */
export type LogoutStyle = 'post-json' | 'navigate';

/**
 * Which claim carries each user field. A value may be a dotted path
 * (`realm_access.roles`). The keys and defaults are the BFF's own
 * `[auth.claim_mappings]` (bff/internal/session/claims.go), so one set of
 * settings means the same thing on both sides.
 */
export type ClaimMappings = {
  username: string;
  email: string;
  roles: string;
  scope: string;
  organization: string;
  org_name: string;
  org_handle: string;
  picture: string;
};

export type AuthContract = {
  loginStyle: LoginStyle;
  /** Where `redirect` sign-in sends the browser. */
  loginUrl: string;
  /** Where `form` sign-in posts the credentials. */
  credentialsUrl: string;
  /** Query parameter that carries the path to come back to after sign-in. */
  returnParam: string;
  sessionUrl: string;
  sessionFormat: SessionFormat;
  logoutUrl: string;
  logoutStyle: LogoutStyle;
  claimMappings: ClaimMappings;
};

/** Raw values as they arrive from the runtime config or the build: strings, or absent. */
export type AuthContractOverrides = Partial<{
  loginStyle: string;
  loginUrl: string;
  credentialsUrl: string;
  returnParam: string;
  sessionUrl: string;
  sessionFormat: string;
  logoutUrl: string;
  logoutStyle: string;
  /** A JSON object, either as a string or already parsed. */
  claimMappings: string | Record<string, unknown>;
}>;

export const DEFAULT_CLAIM_MAPPINGS: ClaimMappings = {
  username: 'username',
  email: 'email',
  roles: 'roles',
  scope: 'scope',
  organization: 'organization',
  org_name: 'org_name',
  org_handle: 'org_handle',
  picture: 'picture',
};

/** The BFF's own routes (bff/internal/server/server.go). */
const BFF_ROUTES = {
  loginUrl: '/api/auth/login',
  credentialsUrl: '/api/login',
  returnParam: 'return',
  sessionUrl: '/api/session',
  logoutUrl: '/api/logout',
} as const;

const PRESETS: Record<
  AuthMode,
  Pick<AuthContract, 'loginStyle' | 'sessionFormat' | 'logoutStyle'>
> = {
  basic: { loginStyle: 'form', sessionFormat: 'bff', logoutStyle: 'post-json' },
  oidc: { loginStyle: 'redirect', sessionFormat: 'bff', logoutStyle: 'post-json' },
  external: { loginStyle: 'redirect', sessionFormat: 'claims', logoutStyle: 'navigate' },
};

export const readAuthMode = (value: string | undefined): AuthMode =>
  value === 'oidc' || value === 'external' ? value : 'basic';

const oneOf = <T extends string>(
  value: string | undefined,
  allowed: readonly T[],
  fallback: T,
): T => (allowed.includes(value as T) ? (value as T) : fallback);

const nonEmpty = (value: string | undefined, fallback: string) => {
  const trimmed = value?.trim();
  return trimmed ? trimmed : fallback;
};

/**
 * Overlays the configured claim names on the defaults. Unknown keys, and
 * values that are not non-empty strings, are ignored; broken JSON leaves the
 * defaults in place rather than failing the whole console.
 */
export const parseClaimMappings = (
  value: AuthContractOverrides['claimMappings'],
): ClaimMappings => {
  let parsed: unknown = value;
  if (typeof value === 'string') {
    if (!value.trim()) return { ...DEFAULT_CLAIM_MAPPINGS };
    try {
      parsed = JSON.parse(value);
    } catch {
      return { ...DEFAULT_CLAIM_MAPPINGS };
    }
  }
  const out: ClaimMappings = { ...DEFAULT_CLAIM_MAPPINGS };
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return out;
  for (const key of Object.keys(DEFAULT_CLAIM_MAPPINGS) as (keyof ClaimMappings)[]) {
    const v = (parsed as Record<string, unknown>)[key];
    if (typeof v === 'string' && v.trim()) out[key] = v.trim();
  }
  return out;
};

export const resolveAuthContract = (
  mode: AuthMode,
  overrides: AuthContractOverrides = {},
): AuthContract => {
  const preset = PRESETS[mode];
  return {
    loginStyle: oneOf(overrides.loginStyle, ['form', 'redirect'], preset.loginStyle),
    loginUrl: nonEmpty(overrides.loginUrl, BFF_ROUTES.loginUrl),
    credentialsUrl: nonEmpty(overrides.credentialsUrl, BFF_ROUTES.credentialsUrl),
    returnParam: nonEmpty(overrides.returnParam, BFF_ROUTES.returnParam),
    sessionUrl: nonEmpty(overrides.sessionUrl, BFF_ROUTES.sessionUrl),
    sessionFormat: oneOf(overrides.sessionFormat, ['bff', 'claims', 'sdk'], preset.sessionFormat),
    logoutUrl: nonEmpty(overrides.logoutUrl, BFF_ROUTES.logoutUrl),
    logoutStyle: oneOf(overrides.logoutStyle, ['post-json', 'navigate'], preset.logoutStyle),
    claimMappings: parseClaimMappings(overrides.claimMappings),
  };
};

/**
 * Turns a path inside the app (what the router knows, without the base path)
 * into a path from the host root, which is what every sign-in endpoint
 * redirects back to. `appBasePath` is '' or '/sub-path' (see runtime.ts).
 */
export const withAppBasePath = (path: string, appBasePath: string) => {
  const inApp = path.startsWith('/') ? path : `/${path}`;
  if (!appBasePath) return inApp;
  return inApp === '/' ? `${appBasePath}/` : `${appBasePath}${inApp}`;
};

/** Appends the "come back to" parameter, whether or not the URL already has a query. */
export const buildLoginUrl = (contract: AuthContract, returnTo: string) => {
  const separator = contract.loginUrl.includes('?') ? '&' : '?';
  return `${contract.loginUrl}${separator}${encodeURIComponent(contract.returnParam)}=${encodeURIComponent(returnTo)}`;
};

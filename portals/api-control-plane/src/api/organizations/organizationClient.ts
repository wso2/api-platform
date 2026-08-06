import { runtimeConfig } from '../../config/runtime';
import type { AuthUser } from '../../features/auth/authTypes';
import type { Organization } from '../../types/domain';
import { toOrganization } from '../adapters';
import { getJson } from '../client';
import { organizations } from '../mocks/data';
import { usePlatformApi } from '../platform/platformClient';
import { delay, useMockApi } from '../shared/apiClientUtils';

/**
 * Direct (no-BML) org resolution: the BFF already resolves the session
 * token's org claims server-side (see `bff/internal/session/claims.go`'s
 * `UserFromClaims`) and returns them as `user.org` on `/api/session` — the
 * browser never holds the token itself to decode. This stands in for the
 * membership lookup that BML's `/validate/user` provides, and is single-org
 * (Platform API's file-based/OIDC claims carry exactly one org per token),
 * matching how the AI Workspace BFF/frontend already do this.
 */
const listOrganizationsFromSession = async (): Promise<Organization[]> => {
  let user: AuthUser | undefined;
  try {
    const response = await fetch('/api/session', { credentials: 'same-origin' });
    if (!response.ok) return [];
    const body = (await response.json()) as { user?: AuthUser };
    user = body.user;
  } catch {
    return [];
  }
  const org = user?.org;
  if (!org || (!org.id && !org.handle)) return [];
  return [
    toOrganization({
      id: org.id,
      uuid: org.id,
      handle: org.handle || org.id,
      name: org.name || org.handle || org.id,
    }),
  ];
};

const readOrganizationList = (value: unknown): unknown[] => {
  if (Array.isArray(value)) return value;
  if (!value || typeof value !== 'object') return [];
  const source = value as Record<string, unknown>;
  if (Array.isArray(source.organizations)) return source.organizations;
  if (Array.isArray(source.list)) return source.list;
  if (Array.isArray(source.items)) return source.items;
  if (Array.isArray(source.data)) return source.data;
  if (source.selectedOrg) return [source.selectedOrg];
  if (source.organization) return [source.organization];
  if (source.data && typeof source.data === 'object') {
    return readOrganizationList(source.data);
  }
  return [];
};

const listOrganizationsFromOrganizationApi = async () => {
  if (!runtimeConfig.organizationApiUrl) return [];
  const data = await getJson<unknown>(
    `${runtimeConfig.organizationApiUrl}/orgs`
  );
  return readOrganizationList(data).map(toOrganization);
};

export async function listOrganizations(): Promise<Organization[]> {
  if (useMockApi()) {
    await delay();
    return organizations.map(toOrganization);
  }

  // Try each source independently. A failure of one (e.g. the org API returns
  // 401 before an org-scoped token exists) must NOT prevent the others from
  // running — `/validate/user` is the source that works with the base user
  // token and is what the legacy console relies on at this stage. Only surface
  // an error if every configured source fails.
  let lastError: unknown;

  // In platform/BML mode, skip the legacy org API entirely — the user's orgs
  // come from the user-context /validate/user below (no org-scoped token), the
  // same source BML uses for membership.
  if (!usePlatformApi() && runtimeConfig.organizationApiUrl) {
    try {
      const fromOrgApi = await listOrganizationsFromOrganizationApi();
      if (fromOrgApi.length > 0) return fromOrgApi;
    } catch (error) {
      lastError = error;
      // eslint-disable-next-line no-console
      console.warn('Organization API org list failed; falling back.', error);
    }
  }

  if (runtimeConfig.usersManagementApiUrl) {
    try {
      const data = await getJson<unknown>(
        `${runtimeConfig.usersManagementApiUrl}/validate/user?origin_cloud=${runtimeConfig.toSServiceName}`
      );
      const validatedOrganizations =
        readOrganizationList(data).map(toOrganization);
      if (validatedOrganizations.length > 0) return validatedOrganizations;
    } catch (error) {
      lastError = error;
      // eslint-disable-next-line no-console
      console.warn('validate/user org list failed.', error);
    }
  }

  // Direct (no-BML) fallback: with no `/validate/user` membership source, derive
  // the single org the BFF already resolved into the session, so the console
  // can proceed.
  if (usePlatformApi()) {
    const fromSession = await listOrganizationsFromSession();
    if (fromSession.length > 0) return fromSession;
  }

  // All sources returned empty without error → genuinely no organizations.
  if (lastError) throw lastError;
  return [];
}

/**
 * Loads a single organization's details. Mirrors legacy
 * `getOrganizationInformation` (GET ${organizationApiUrl}/orgs/{handle}). Falls
 * back to the entry in the org list when no per-org endpoint is configured, so
 * callers always get the best available details.
 */
export async function getOrganization(
  orgHandle: string
): Promise<Organization | undefined> {
  if (!orgHandle) return undefined;

  if (!useMockApi() && !usePlatformApi() && runtimeConfig.organizationApiUrl) {
    try {
      const data = await getJson<unknown>(
        `${runtimeConfig.organizationApiUrl}/orgs/${orgHandle}`
      );
      const [organization] = readOrganizationList(data).map(toOrganization);
      if (organization) return organization;
    } catch {
      // Fall through to the list lookup below.
    }
  }

  const all = await listOrganizations();
  return all.find((item) => item.handle === orgHandle || item.id === orgHandle);
}

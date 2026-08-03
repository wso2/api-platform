import { runtimeConfig } from '../../config/runtime';
import type { Organization } from '../../types/domain';
import { toOrganization } from '../adapters';
import { getJson, getPlatformToken } from '../client';
import { organizations } from '../mocks/data';
import { usePlatformApi } from '../platform/platformClient';
import { delay, useMockApi } from '../shared/apiClientUtils';

/** Decodes a JWT payload (no signature check) into a claims object. */
const decodeJwtClaims = (
  token: string
): Record<string, unknown> | undefined => {
  const payload = token.split('.')[1];
  if (!payload) return undefined;
  try {
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/');
    const padded = base64.padEnd(Math.ceil(base64.length / 4) * 4, '=');
    return JSON.parse(decodeURIComponent(escape(atob(padded)))) as Record<
      string,
      unknown
    >;
  } catch {
    return undefined;
  }
};

/**
 * Direct (no-BML) org resolution: derive the single organization from the IdP
 * access token's OU claims. In the wso2cloud model each user belongs to one
 * Organization Unit (= one organization), carried as ouId/ouName/ouHandle.
 * This stands in for the membership lookup that BML's `/validate/user` provides.
 */
const listOrganizationsFromToken = async (): Promise<Organization[]> => {
  const token = await getPlatformToken();
  if (!token) return [];
  const claims = decodeJwtClaims(token);
  if (!claims) return [];
  const id =
    (claims.ouId as string) ??
    (claims.organization as string) ??
    (claims.organizationID as string);
  const handle = (claims.ouHandle as string) ?? (claims.org_handle as string);
  const name = (claims.ouName as string) ?? (claims.org_name as string);
  if (!id && !handle) return [];
  return [
    toOrganization({
      id,
      uuid: id,
      handle: handle ?? id,
      name: name ?? handle ?? id,
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
  // the single org from the IdP token's OU claims so the console can proceed.
  if (usePlatformApi()) {
    const fromToken = await listOrganizationsFromToken();
    if (fromToken.length > 0) return fromToken;
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

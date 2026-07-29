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

type ProjectLike = {
  id?: string;
  displayName?: string;
};
type OrgLike = {
  handle?: string;
  name?: string;
};

export function slugifyProjectName(name?: string): string {
  if (!name) return '';
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export function getProjectSlug(project?: ProjectLike | null): string {
  if (!project) return '';
  // ProjectBase no longer carries a dedicated `handler` slug field, so use
  // the project's `id` (its handle) as the routing slug.
  return project.id || slugifyProjectName(project.displayName);
}

export function slugifyOrgName(name?: string): string {
  if (!name) return '';
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export function getOrgSlug(org?: OrgLike | null): string {
  if (!org) return '';
  return org.handle || slugifyOrgName(org.name);
}

export function getProjectBasePath(project?: ProjectLike | null): string {
  const slug = getProjectSlug(project);
  return slug ? `/${slug}` : '';
}

export function getOrgBasePath(org?: OrgLike | null): string {
  const slug = getOrgSlug(org);
  return slug ? `/organizations/${slug}` : '';
}

export function buildOrgPath(org: OrgLike | null | undefined, path: string): string {
  const base = getOrgBasePath(org);
  if (!base) return path.startsWith('/') ? path : `/${path}`;
  if (!path || path === '/') return base;
  return `${base}/${path.replace(/^\/+/, '')}`;
}

/**
 * Resolves the route for a proxy that may live in any project of the current
 * organization.
 *
 * `proxyProjectId` is the project handle (the same value `ProjectBase.id`
 * carries), so the proxy's own project is matched by handle. Returns `null`
 * when the proxy names a project outside `projects` — a stale listing, or a
 * project the user can't see. Callers must surface that as an error rather
 * than navigating somewhere the proxy doesn't live: an org-level fallback
 * lands on a page whose project-scoped breadcrumbs and back links are wrong,
 * and whose fetch fails anyway when the project is genuinely out of reach.
 */
export function buildProxyPath(
  org: OrgLike | null | undefined,
  projects: ProjectLike[],
  proxyId: string,
  proxyProjectId?: string
): string | null {
  const proxyProject = proxyProjectId
    ? projects.find((project) => project.id === proxyProjectId)
    : undefined;
  if (!proxyProject) return null;

  return buildProjectPath(
    org,
    proxyProject,
    `/proxies/${encodeURIComponent(proxyId)}`
  );
}

/**
 * Resolves the route for a gateway detail view.
 *
 * Gateways are organization-scoped, not project-scoped, so this is always an
 * org-level path. `gatewayId` is the gateway handle — the same value
 * `Gateway.id` carries and the value `ViewGateway` matches its `:gatewayName`
 * param against.
 */
export function buildGatewayPath(
  org: OrgLike | null | undefined,
  gatewayId: string
): string {
  return buildOrgPath(org, `/gateways/view/${encodeURIComponent(gatewayId)}`);
}

export function buildProjectPath(
  org: OrgLike | null | undefined,
  project: ProjectLike | null | undefined,
  path: string
): string {
  const orgBase = getOrgBasePath(org);
  const projectBase = getProjectBasePath(project);
  const base =
    orgBase && projectBase ? `${orgBase}/projects${projectBase}` : '';
  if (!base) return path.startsWith('/') ? path : `/${path}`;
  if (!path || path === '/') return base;
  return `${base}/${path.replace(/^\/+/, '')}`;
}

import type { CreateProjectInput, Project } from '../../types/domain';
import { toProject } from '../adapters';
import { postGraphql } from '../client';
import { components, organizations, projects } from '../mocks/data';
import { listOrganizations } from '../organizations/organizationClient';
import {
  PLATFORM_API_BASE,
  platformDelete,
  platformGet,
  platformPost,
  usePlatformApi,
} from '../platform/platformClient';
import {
  delay,
  type GraphqlResponse,
  useMockApi,
} from '../shared/apiClientUtils';
import { ApiError } from '../types/errors';

const findOrganization = async (orgHandle: string) => {
  const normalizedOrgHandle = orgHandle.toLowerCase();
  const organizationsList = await listOrganizations();
  return organizationsList.find((item) =>
    [item.handle, item.id, item.uuid]
      .filter(Boolean)
      .some((value) => value.toLowerCase() === normalizedOrgHandle)
  );
};

export async function listProjects(orgHandle: string): Promise<Project[]> {
  if (useMockApi()) {
    await delay();
    const org = organizations.find((item) => item.handle === orgHandle);
    return projects
      .filter((project) => project.orgId === org?.id)
      .map(toProject);
  }

  // platform-api REST via BML: org comes from the token; X-Org-Id carries the
  // handle. Response is { list, count, pagination }.
  if (usePlatformApi()) {
    const data = await platformGet<{ list?: unknown[] }>(
      `${PLATFORM_API_BASE}/projects`,
      orgHandle
    );
    return (data.list || []).map(toProject);
  }

  const organization = await findOrganization(orgHandle);
  if (!organization) {
    throw new Error(`Organization context was not found for "${orgHandle}".`);
  }
  const orgId = organization.numericId ?? Number(organization.id);
  if (!Number.isFinite(orgId)) {
    throw new Error(
      `Organization "${orgHandle}" does not include the numeric id required by the projects API.`
    );
  }
  const data = await postGraphql<GraphqlResponse<{ projects: unknown[] }>>(
    `query {
      projects(orgId: ${orgId}) {
        id
        orgId
        name
        version
        createdDate
        handler
        region
        description
        defaultDeploymentPipelineId
        deploymentPipelineIds
        type
        gitProvider
        gitOrganization
        repository
        branch
        secretRef
        updatedAt
      }
    }`,
    undefined,
    { orgHandle }
  );
  return data.projects.map(toProject);
}

export async function getProject(
  orgHandle: string,
  projectHandler: string
): Promise<Project | undefined> {
  // platform-api: projects are UUID-keyed and the URL handler is that UUID.
  if (usePlatformApi()) {
    try {
      const project = await platformGet<unknown>(
        `${PLATFORM_API_BASE}/projects/${encodeURIComponent(projectHandler)}`,
        orgHandle
      );
      return toProject(project);
    } catch (error) {
      if (error instanceof ApiError && error.code === 'NOT_FOUND')
        return undefined;
      throw error;
    }
  }

  const allProjects = await listProjects(orgHandle);
  return allProjects.find((project) => project.handler === projectHandler);
}

const slugify = (value: string) =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

export async function createProject(
  orgHandle: string,
  input: CreateProjectInput
): Promise<Project> {
  const name = input.name.trim();
  if (!name) throw new Error('Project name is required');

  if (useMockApi()) {
    await delay();
    const org = organizations.find((item) => item.handle === orgHandle);
    if (projects.some((p) => p.orgId === org?.id && p.name === name)) {
      throw new ApiError(
        'Project already exists in organization',
        'CONFLICT',
        409
      );
    }
    const handler = slugify(name) || `project-${Date.now()}`;
    const now = new Date().toISOString();
    const project: Project = {
      id: `project-${Date.now()}`,
      orgId: org?.id || '',
      name,
      handler,
      description: input.description?.trim() || undefined,
      createdDate: now,
      updatedAt: now,
    };
    projects.push(project);
    return toProject(project);
  }

  // platform-api REST via BML: POST {PLATFORM_API_BASE}/projects. The
  // organization is taken from the token (X-Org-Id carries the handle); the
  // body is just displayName + optional description. Response is the created
  // Project.
  if (usePlatformApi()) {
    const created = await platformPost<unknown>(
      `${PLATFORM_API_BASE}/projects`,
      orgHandle,
      {
        displayName: name,
        ...(input.description?.trim()
          ? { description: input.description.trim() }
          : {}),
      }
    );
    return toProject(created);
  }

  throw new ApiError(
    'Project creation requires the platform-api backend.',
    'UNKNOWN'
  );
}

export async function deleteProject(
  orgHandle: string,
  project: Project
): Promise<void> {
  if (useMockApi()) {
    await delay();
    const org = organizations.find((item) => item.handle === orgHandle);
    const orgProjects = projects.filter((p) => p.orgId === org?.id);
    // Mirror the platform-api delete guards (last project / has APIs) so the
    // mock surfaces the same blocking errors the real backend returns.
    if (orgProjects.length <= 1) {
      throw new ApiError(
        'Organization must have at least one project',
        'UNKNOWN',
        400
      );
    }
    if (components.some((c) => c.projectId === project.id)) {
      throw new ApiError('Project has associated APIs', 'UNKNOWN', 400);
    }
    const index = projects.findIndex((p) => p.id === project.id);
    if (index < 0) throw new ApiError('Project not found', 'NOT_FOUND', 404);
    projects.splice(index, 1);
    return;
  }

  // platform-api: DELETE {PLATFORM_API_BASE}/projects/{projectId} (UUID).
  // Returns 204; the backend enforces last-project / associated-API|MCP|WebSub
  // guards and returns 400 with a descriptive message, surfaced via ApiError.
  if (usePlatformApi()) {
    await platformDelete<void>(
      `${PLATFORM_API_BASE}/projects/${encodeURIComponent(project.id)}`,
      orgHandle
    );
    return;
  }

  throw new ApiError(
    'Project deletion requires the platform-api backend.',
    'UNKNOWN'
  );
}

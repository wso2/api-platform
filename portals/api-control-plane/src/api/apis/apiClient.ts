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

import type { Api, ApiDetail, CreateApiInput } from '../../types/domain';
import { toApi } from '../adapters';
import { postGraphql } from '../client';
import { components } from '../mocks/data';
import {
  createRestApiBody,
  detailToRestApiBody,
  restApiToApi,
  restApiToDetail,
} from '../platform/platformAdapters';
import {
  PLATFORM_API_BASE,
  platformDelete,
  platformGet,
  platformPost,
  platformPut,
  usePlatformApi,
} from '../platform/platformClient';
import { getProject } from '../projects/projectClient';
import {
  delay,
  gqlString,
  type GraphqlResponse,
  useMockApi,
} from '../shared/apiClientUtils';
import { ApiError } from '../types/errors';
import { recordApiDerivedData } from './apiDerivedDataStore';

const componentListSelection = `
  projectId
  id
  description
  status
  initStatus
  name
  handler
  displayName
  displayType
  componentSubType
  ownerName
  version
  createdAt
  lastBuildDate
  orgHandler
  isSystemComponent
  httpBased
  apiVersions {
    apiVersion
    proxyName
    proxyUrl
    proxyId
    id
    state
    latest
    branch
    accessibility
    autoDeployEnabled
  }
  deploymentTracks {
    id
    createdAt
    updatedAt
    apiVersion
    branch
    description
    componentId
    latest
    versionStrategy
    autoDeployEnabled
  }
`;

const componentDetailSelection = `
  id
  name
  handler
  description
  displayType
  componentSubType
  displayName
  ownerName
  orgId
  orgHandler
  version
  labels
  createdAt
  projectId
  apiId
  httpBased
  isMigrationCompleted
  skipDeploy
  endpointShortUrlEnabled
  isUnifiedConfigMapping
  serviceAccessMode
  apiVersions {
    apiVersion
    proxyName
    proxyUrl
    proxyId
    id
    state
    latest
    branch
    accessibility
    versionId
    appEnvVersions {
      environmentId
      releaseId
      release {
        id
        metadata {
          choreoEnv
        }
        environmentId
        environment
        gitHash
        gitOpsHash
      }
    }
    autoDeployEnabled
  }
  deploymentTracks {
    id
    createdAt
    updatedAt
    apiVersion
    branch
    description
    componentId
    latest
    versionStrategy
    autoDeployEnabled
  }
`;

export async function listApis(
  orgHandle: string,
  projectHandler: string
): Promise<Api[]> {
  // platform-api: components are REST APIs under a project. The URL
  // projectHandler is the project UUID (platform projects are UUID-keyed).
  if (usePlatformApi()) {
    const projectId = encodeURIComponent(projectHandler);
    const apis = await platformGet<{ list?: unknown[] }>(
      `${PLATFORM_API_BASE}/rest-apis?projectId=${projectId}`,
      orgHandle
    );
    return (apis.list || []).map(restApiToApi);
  }

  const project = await getProject(orgHandle, projectHandler);
  if (!project) return [];

  if (useMockApi()) {
    await delay();
    return components
      .filter((component) => component.projectId === project.id)
      .map(toApi);
  }

  const data = await postGraphql<GraphqlResponse<{ components: unknown[] }>>(
    `query {
      components(orgHandler: ${gqlString(orgHandle)}, projectId: ${gqlString(
        project.id
      )}) {
        ${componentListSelection}
      }
    }`,
    undefined,
    { orgHandle, projectHandler }
  );
  data.components.forEach(recordApiDerivedData);
  return data.components.map(toApi);
}

export async function getApi(
  orgHandle: string,
  projectHandler: string,
  apiHandler: string
): Promise<Api | undefined> {
  // platform-api: apiHandler is the REST API id (handle).
  if (usePlatformApi()) {
    try {
      const api = await platformGet<unknown>(
        `${PLATFORM_API_BASE}/rest-apis/${encodeURIComponent(apiHandler)}`,
        orgHandle
      );
      return restApiToApi(api);
    } catch (error) {
      if (error instanceof ApiError && error.code === 'NOT_FOUND')
        return undefined;
      throw error;
    }
  }

  const project = await getProject(orgHandle, projectHandler);
  if (!project) return undefined;

  if (useMockApi()) {
    const allApis = await listApis(orgHandle, projectHandler);
    return allApis.find((component) => component.handler === apiHandler);
  }

  const data = await postGraphql<GraphqlResponse<{ component?: unknown }>>(
    `query {
      component(
        projectId: ${gqlString(project.id)}
        componentHandler: ${gqlString(apiHandler)}
      ) {
        ${componentDetailSelection}
      }
    }`,
    undefined,
    { orgHandle, projectHandler }
  );
  if (!data.component) return undefined;
  recordApiDerivedData(data.component);
  return toApi(data.component);
}

export async function createApi(
  orgHandle: string,
  projectHandler: string,
  input: CreateApiInput
): Promise<Api> {
  const project = await getProject(orgHandle, projectHandler);
  if (!project) throw new Error('Project not found');

  if (useMockApi()) {
    await delay();
    const component: Api = {
      id: `component-${Date.now()}`,
      projectId: project.id,
      name: input.name,
      displayName: input.displayName,
      handler: input.name.toLowerCase().replace(/\s+/g, '-'),
      kind: input.kind,
      status: 'PENDING',
      description: input.description,
      version: input.version,
      httpBased: true,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    components.push(component);
    return toApi(component);
  }

  if (usePlatformApi()) {
    // Only API proxies are creatable from the console. platform-api has no
    // server-side OpenAPI import — CreateRESTAPI takes a structured body — so
    // an imported definition is parsed in the browser (ApiCreatePage) and its
    // operations/upstream are carried on the input, then sent as one create.
    const created = await platformPost<unknown>(
      `${PLATFORM_API_BASE}/rest-apis`,
      orgHandle,
      createRestApiBody(input, project.id)
    );
    return restApiToApi(created);
  }

  const data = await postGraphql<{ createComponent: Api }>(
    `mutation OxygenCreateComponent(
      $orgHandle: String!
      $projectId: String!
      $component: OxygenCreateComponentInput!
    ) {
      createComponent(
        orgHandler: $orgHandle
        projectId: $projectId
        component: $component
      ) {
        id projectId name displayName handler description version status
      }
    }`,
    { orgHandle, projectId: project.id, component: input },
    { orgHandle, projectHandler }
  );
  return toApi(data.createComponent);
}

const detailFromApi = (component: Api): ApiDetail => ({
  ...component,
  operations: [],
  policies: [],
  endpoints: {},
});

/**
 * Loads the full Develop detail for an API. Platform mode tries the REST
 * API then the MCP proxy (apiHandler is the slug id); mock mode derives a
 * minimal detail from the listed API.
 */
export async function getApiDetail(
  orgHandle: string,
  projectHandler: string,
  apiHandler: string
): Promise<ApiDetail | undefined> {
  if (usePlatformApi()) {
    try {
      const api = await platformGet<unknown>(
        `${PLATFORM_API_BASE}/rest-apis/${encodeURIComponent(apiHandler)}`,
        orgHandle
      );
      return restApiToDetail(api);
    } catch (error) {
      if (error instanceof ApiError && error.code === 'NOT_FOUND')
        return undefined;
      throw error;
    }
  }

  const component = await getApi(orgHandle, projectHandler, apiHandler);
  return component ? detailFromApi(component) : undefined;
}

/**
 * Persists Develop edits via a full-object PUT (GET-merge-PUT: `detail.raw`
 * holds the untouched platform object). Returns the refreshed detail.
 */
export async function updateApi(
  orgHandle: string,
  projectHandler: string,
  detail: ApiDetail
): Promise<ApiDetail> {
  if (usePlatformApi()) {
    const updated = await platformPut<unknown>(
      `${PLATFORM_API_BASE}/rest-apis/${encodeURIComponent(detail.handler)}`,
      orgHandle,
      detailToRestApiBody(detail)
    );
    return restApiToDetail(updated);
  }

  // Mock mode: no persistence layer for develop fields; echo the edited detail.
  await delay();
  return detail;
}

/**
 * Deletes an API proxy. `api.handler` is the platform slug id. Irreversible.
 */
export async function deleteApi(
  orgHandle: string,
  projectHandler: string,
  api: Api
): Promise<void> {
  if (useMockApi()) {
    await delay();
    const index = components.findIndex(
      (component) =>
        component.id === api.id || component.handler === api.handler
    );
    if (index >= 0) components.splice(index, 1);
    return;
  }

  if (usePlatformApi()) {
    await platformDelete<void>(
      `${PLATFORM_API_BASE}/rest-apis/${encodeURIComponent(api.handler)}`,
      orgHandle
    );
    return;
  }

  const project = await getProject(orgHandle, projectHandler);
  if (!project) throw new Error('Project not found');
  await postGraphql<{ deleteComponent: unknown }>(
    `mutation OxygenDeleteComponent(
      $orgHandle: String!
      $projectId: String!
      $componentId: String!
    ) {
      deleteComponent(
        orgHandler: $orgHandle
        projectId: $projectId
        id: $componentId
      )
    }`,
    { orgHandle, projectId: project.id, componentId: api.id },
    { orgHandle, projectHandler }
  );
}

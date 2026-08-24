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

import { graphql, HttpResponse } from 'msw';

import type { Api } from '../../types/domain';
import {
  apiProxies,
  components,
  deployments,
  environments,
  organizations,
  projects,
} from './data';

type ProjectVariables = {
  orgHandle: string;
};

type ApiVariables = {
  projectId: string;
};

type DeploymentVariables = {
  componentId: string;
};

type CreateApiVariables = {
  projectId: string;
  component: Partial<Api>;
};

export const handlers = [
  graphql.query('OxygenOrganizations', () =>
    HttpResponse.json({ data: { organizations } })
  ),
  graphql.query('OxygenProjects', ({ variables }) => {
    const projectVariables = variables as ProjectVariables;
    const organization = organizations.find(
      (item) => item.handle === projectVariables.orgHandle
    );
    return HttpResponse.json({
      data: {
        projects: projects.filter(
          (project) => project.orgId === organization?.id
        ),
      },
    });
  }),
  graphql.query('OxygenComponents', ({ variables }) => {
    const componentVariables = variables as ApiVariables;
    return HttpResponse.json({
      data: {
        components: components.filter(
          (component) => component.projectId === componentVariables.projectId
        ),
      },
    });
  }),
  graphql.query('OxygenEnvironments', () =>
    HttpResponse.json({ data: { environments } })
  ),
  graphql.query('OxygenDeployments', ({ variables }) => {
    const deploymentVariables = variables as DeploymentVariables;
    return HttpResponse.json({
      data: {
        deployments: deployments.filter(
          (deployment) =>
            deployment.componentId === deploymentVariables.componentId
        ),
      },
    });
  }),
  graphql.query('OxygenApiProxy', ({ variables }) => {
    const deploymentVariables = variables as DeploymentVariables;
    return HttpResponse.json({
      data: {
        apiProxy: apiProxies.find(
          (apiProxy) => apiProxy.componentId === deploymentVariables.componentId
        ),
      },
    });
  }),
  graphql.mutation('OxygenCreateComponent', ({ variables }) => {
    const createVariables = variables as CreateApiVariables;
    const displayName = createVariables.component.displayName || 'Create API';
    const component: Api = {
      id: `component-${Date.now()}`,
      projectId: createVariables.projectId,
      name: createVariables.component.name || String(displayName),
      displayName: String(displayName),
      handler: String(createVariables.component.name || displayName)
        .toLowerCase()
        .replace(/\s+/g, '-'),
      kind: createVariables.component.kind || 'API_PROXY',
      status: 'PENDING',
      description: createVariables.component.description,
      version: createVariables.component.version,
      httpBased: true,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    components.push(component);
    return HttpResponse.json({ data: { createApi: component } });
  }),
];

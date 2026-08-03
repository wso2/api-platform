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

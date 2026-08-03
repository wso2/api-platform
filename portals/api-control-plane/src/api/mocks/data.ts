import type {
  ApiProxy,
  Api,
  Deployment,
  Environment,
  Gateway,
  Organization,
  Project,
} from '../../types/domain';

export const organizations: Organization[] = [
  {
    id: 'org-1',
    uuid: '783c6c4d-8b9b-4190-b70a-e717ab1ee739',
    name: 'API Platform Demo',
    handle: 'api-platform-demo',
    description: 'Demo organization for API Platform development',
    status: 'ACTIVE',
  },
];

export const projects: Project[] = [
  {
    id: 'project-1',
    orgId: 'org-1',
    name: 'Retail APIs',
    handler: 'retail-apis',
    description: 'Core APIs for retail services',
    region: 'us-east-1',
    version: '1.0.0',
    createdDate: '2026-05-20T08:00:00.000Z',
    updatedAt: '2026-06-01T08:00:00.000Z',
    type: 'MULTI_REPO',
    gitProvider: 'github',
    repository: 'api-platform-demo/retail-apis',
  },
  {
    id: 'project-2',
    orgId: 'org-1',
    name: 'Internal Tools',
    handler: 'internal-tools',
    description: 'Operations and internal integration services',
    region: 'us-east-1',
    version: '1.0.0',
    createdDate: '2026-05-28T08:00:00.000Z',
    updatedAt: '2026-06-10T08:00:00.000Z',
    type: 'MONO_REPO',
    gitProvider: 'github',
    repository: 'api-platform-demo/internal-tools',
  },
];

export const components: Api[] = [
  {
    id: 'component-1',
    projectId: 'project-1',
    name: 'orders-api',
    displayName: 'Orders API',
    handler: 'orders-api',
    kind: 'API_PROXY',
    status: 'ACTIVE',
    description: 'Proxy for order management APIs',
    version: '1.0.0',
    httpBased: true,
    updatedAt: '2026-06-11T08:00:00.000Z',
    owner: 'AnuGayan',
  },
];

export const environments: Environment[] = [
  { id: 'env-dev', name: 'Development', type: 'DEVELOPMENT' },
  { id: 'env-prod', name: 'Production', type: 'PRODUCTION' },
];

export const deployments: Deployment[] = [
  {
    id: 'deployment-1',
    componentId: 'component-1',
    environmentId: 'env-dev',
    status: 'READY',
    version: '1.0.0',
    updatedAt: '2026-06-13T08:00:00.000Z',
  },
  {
    id: 'deployment-2',
    componentId: 'component-1',
    environmentId: 'env-prod',
    status: 'NOT_DEPLOYED',
    version: '1.0.0',
  },
];

export const gateways: Gateway[] = [
  {
    id: 'gw-prod-1',
    name: 'prod-gateway-01',
    displayName: 'Production Gateway 01',
    vhost: 'mg.api-platform-demo.dev',
    functionalityType: 'regular',
    mode: 'self-hosted',
    description: 'Primary production gateway',
    version: '1.0',
    isActive: true,
    isCritical: true,
    organizationId: 'org-1',
    updatedAt: '2026-06-12T08:00:00.000Z',
  },
];

export const apiProxies: ApiProxy[] = [
  {
    id: 'api-proxy-1',
    componentId: 'component-1',
    context: '/orders',
    version: '1.0.0',
    visibility: 'PUBLIC',
  },
];

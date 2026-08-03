export const routes = {
  login: '/login',
  authCallback: '/login/callback',
  signInCallback: '/signin',
  unauthorized: '/unauthorized',
  sessionExpired: '/session-expired',
  serverError: '/server-error',
  organizations: '/organizations',
  organizationHome: (orgHandle = ':orgHandle') =>
    `/organizations/${orgHandle}/home`,
  projects: (orgHandle = ':orgHandle') =>
    `/organizations/${orgHandle}/projects`,
  gateways: (orgHandle = ':orgHandle') =>
    `/organizations/${orgHandle}/gateways`,
  newGateway: (orgHandle = ':orgHandle') =>
    `/organizations/${orgHandle}/gateways/new`,
  gateway: (orgHandle = ':orgHandle', gatewayId = ':gatewayId') =>
    `/organizations/${orgHandle}/gateways/${gatewayId}`,
  projectHome: (orgHandle = ':orgHandle', projectHandler = ':projectHandler') =>
    `/organizations/${orgHandle}/projects/${projectHandler}/home`,
  apis: (orgHandle = ':orgHandle', projectHandler = ':projectHandler') =>
    `/organizations/${orgHandle}/projects/${projectHandler}/apis`,
  newApi: (
    orgHandle = ':orgHandle',
    projectHandler = ':projectHandler'
  ) => `/organizations/${orgHandle}/projects/${projectHandler}/apis/new`,
  api: (
    orgHandle = ':orgHandle',
    projectHandler = ':projectHandler',
    apiHandler = ':apiHandler'
  ) =>
    `/organizations/${orgHandle}/projects/${projectHandler}/apis/${apiHandler}`,
  apiDeploy: (
    orgHandle = ':orgHandle',
    projectHandler = ':projectHandler',
    apiHandler = ':apiHandler'
  ) =>
    `/organizations/${orgHandle}/projects/${projectHandler}/apis/${apiHandler}/deploy`,
  apiTest: (
    orgHandle = ':orgHandle',
    projectHandler = ':projectHandler',
    apiHandler = ':apiHandler'
  ) =>
    `/organizations/${orgHandle}/projects/${projectHandler}/apis/${apiHandler}/test`,
  apiManage: (
    orgHandle = ':orgHandle',
    projectHandler = ':projectHandler',
    apiHandler = ':apiHandler'
  ) =>
    `/organizations/${orgHandle}/projects/${projectHandler}/apis/${apiHandler}/manage`,
  runtimeLogs: (orgHandle = ':orgHandle', projectHandler = ':projectHandler') =>
    `/organizations/${orgHandle}/projects/${projectHandler}/observe/runtimelogs`,
  settings: (orgHandle = ':orgHandle', projectHandler = ':projectHandler') =>
    `/organizations/${orgHandle}/projects/${projectHandler}/settings`,
};

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
  devportal: (orgHandle = ':orgHandle') =>
    `/organizations/${orgHandle}/devportal`,
  newDevportal: (orgHandle = ':orgHandle') =>
    `/organizations/${orgHandle}/devportal/new`,
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

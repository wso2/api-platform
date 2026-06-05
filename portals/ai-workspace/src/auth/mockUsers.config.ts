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

import type { PlatformRole } from './permissions';

export interface MockUser {
  username: string;
  password: string;
  role: PlatformRole;
  name: string;
  email: string;
}

/**
 * Mock users for local development (VITE_DISABLE_AUTH=true).
 * Edit this list to add, remove, or change users and their roles.
 * Default: admin / admin with admin role.
 */
export const MOCK_USERS: MockUser[] = [
  {
    username: 'admin',
    password: 'admin',
    role: 'admin',
    name: 'Admin User',
    email: 'admin@localhost',
  },
  {
    username: 'developer',
    password: 'developer',
    role: 'developer',
    name: 'Developer User',
    email: 'developer@localhost',
  },
  {
    username: 'viewer',
    password: 'viewer',
    role: 'viewer',
    name: 'Viewer User',
    email: 'viewer@localhost',
  },
];

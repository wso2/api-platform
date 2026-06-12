/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

const llmProxyOverviewMock = {
  provider: {
    name: 'OpenAI',
    type: 'Managed',
    baseUrl: 'https://api.openai.com',
    modelFamily: 'GPT-4',
    region: 'us-east-1',
    notes: 'Managed by platform team',
  },
  resources: [
    { method: 'POST', path: '/chat/completions' },
    { method: 'POST', path: '/completions' },
    { method: 'POST', path: '/embeddings' },
    { method: 'POST', path: '/models' },
  ],
  security: {
    authentication: 'API Key',
    header: 'x-api-key',
    blockedConsumers: [
      { criteria: 'Application', identifier: 'Docs Assistant' },
      { criteria: 'User', identifier: 'Nimsara' },
    ],
    accessControl: [
      {
        method: 'POST',
        path: '/chat/completions',
        scopes: ['admin', 'platform_engineer'],
      },
      {
        method: 'POST',
        path: '/completions',
        scopes: ['admin', 'platform_engineer'],
      },
      { method: 'POST', path: '/embeddings', scopes: ['admin'] },
      { method: 'POST', path: '/models', scopes: ['admin'] },
    ],
  },
  policies: {
    global: ['Policy - PII Masking', 'Policy - Prompt Injection'],
    resourceWise: [
      { method: 'POST', path: '/models', policies: ['Policy - PII Masking'] },
      { method: 'POST', path: '/completions', policies: ['Policy - Rate Guard'] },
      { method: 'POST', path: '/embeddings', policies: ['Policy - PII Masking'] },
      {
        method: 'POST',
        path: '/chat/completions',
        policies: ['Policy - Prompt Injection', 'Policy - Rate Guard'],
      },
    ],
  },
};

export default llmProxyOverviewMock;

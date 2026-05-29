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

const serviceProviderOverviewMock = {
  connection: {
    providerEndpoint: 'https://example.com/openapi',
    authentication: 'API Key',
    credential: '****************',
    specificationUrl: 'github.com/openai/openapi.yml',
    llmConfigurations: [
      { label: 'Request Model', source: 'Payload', identifier: '$.model' },
      { label: 'Response Model', source: 'Payload', identifier: '$.model' },
      {
        label: 'Prompt Token Count',
        source: 'Header',
        identifier: '$.usage.input_tokens',
      },
      {
        label: 'Completion Token Count',
        source: 'Header',
        identifier: '$.usage.output_tokens',
      },
      {
        label: 'Total Token Count',
        source: 'Payload',
        identifier: '$.usage.total_tokens',
      },
      { label: 'Cost', source: 'Payload', identifier: '$.usage.cost' },
    ],
  },
  resources: [
    { method: 'POST', path: '/chat/completions' },
    { method: 'POST', path: '/completions' },
    { method: 'POST', path: '/embeddings' },
    { method: 'POST', path: '/models' },
  ],
  guardrails: {
    global: ['GR1', 'GR2'],
    resourceWise: [
      { method: 'POST', path: '/models', guardrails: ['GR1', 'GR2'] },
      { method: 'POST', path: '/completions', guardrails: ['GR1'] },
      { method: 'POST', path: '/embeddings', guardrails: ['GR2'] },
      { method: 'POST', path: '/chat/completions', guardrails: ['GR1', 'GR2'] },
    ],
  },
  rateLimiting: {
    backend: {
      criteria: [
        {
          label: 'Request Count',
          quota: '1500',
          resetValue: '2',
          resetUnit: 'weeks',
        },
        {
          label: 'Token Count',
          quota: '1000000',
          resetValue: '1',
          resetUnit: 'month',
        },
        { label: 'Cost', quota: '$ 1000', resetValue: '1', resetUnit: 'month' },
      ],
    },
    perConsumer: {
      criteria: [
        {
          label: 'Request Count',
          quota: '50',
          resetValue: '2',
          resetUnit: 'weeks',
        },
        {
          label: 'Token Count',
          quota: '100000',
          resetValue: '1',
          resetUnit: 'month',
        },
        { label: 'Cost', quota: '$ 100', resetValue: '1', resetUnit: 'month' },
      ],
    },
  },
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
  models: {
    providers: [
      {
        name: 'OpenAI',
        models: ['gpt-4o', 'gpt-4.1', 'gpt-4.1-mini', 'gpt-3.5-turbo'],
      },
    ],
  },
};

export default serviceProviderOverviewMock;

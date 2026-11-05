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

import yaml from 'js-yaml';

import type { PolicyDefinition } from '../engine';

/**
 * Real policy definitions copied from wso2/gateway-controllers (policies/<name>/policy-definition.yaml).
 */
export type FixtureName =
  | 'oauth2-generator'
  | 'cors'
  | 'set-headers'
  | 'request-rewrite'
  | 'token-based-ratelimit'
  | 'log-message'
  | 'advanced-ratelimit'
  | 'aws-bedrock-guardrail'
  | 'json-xml-mediator'
  | 'api-key-auth'
  | 'content-length-guardrail';

const files = import.meta.glob('./fixtures/*.yaml', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>;

const cache = new Map<FixtureName, PolicyDefinition>();

export function fixture(name: FixtureName): PolicyDefinition {
  if (!cache.has(name)) {
    const text = files[`./fixtures/${name}.yaml`];
    if (!text) throw new Error(`Missing fixture ${name}.yaml`);
    cache.set(name, yaml.load(text) as PolicyDefinition);
  }
  return cache.get(name)!;
}

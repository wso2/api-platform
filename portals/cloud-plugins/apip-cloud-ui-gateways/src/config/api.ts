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

import type { ApiFetch } from '../hostPort';
import type { ConfigValues, GatewayConfiguration } from '../types';

const configurationPath = (gatewayId: string) =>
  `/managed-gateways/${encodeURIComponent(gatewayId)}/configuration`;

/**
 * The Port resolves `undefined` for an empty successful body (a 204, say). Both
 * endpoints below always answer with the whole configuration, so an empty body
 * is a broken response rather than an "unset" configuration — fail loudly here
 * instead of leaking `undefined` into a form that has no way to render it.
 */
async function required(
  call: Promise<GatewayConfiguration | undefined>
): Promise<GatewayConfiguration> {
  const configuration = await call;
  if (!configuration) {
    throw new Error('The gateway configuration service returned an empty response.');
  }
  return configuration;
}

/** Scopes: `ap:gateway:read` or `ap:gateway:manage`. 404 for a gateway with no managed binding. */
export const readConfiguration = (apiFetch: ApiFetch, gatewayId: string) =>
  required(apiFetch<GatewayConfiguration>('GET', configurationPath(gatewayId)));

/**
 * A SPARSE PATCH: send only the settings that changed. `values` must name at
 * least one — `{}` and `{"values":{}}` are both a 400 — and no other top-level
 * key may appear, which is also a 400.
 *
 * The response is the WHOLE configuration after the write, in the GET's shape.
 * Use it as the confirmation and as the new baseline; do not re-GET.
 *
 * Scopes: `ap:gateway:update` or `ap:gateway:manage`.
 */
export const writeConfiguration = (
  apiFetch: ApiFetch,
  gatewayId: string,
  values: ConfigValues
) =>
  required(
    apiFetch<GatewayConfiguration>('PUT', configurationPath(gatewayId), {
      values,
    })
  );

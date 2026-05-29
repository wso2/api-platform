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

import { get } from '../clients/choreoApiClient';
import { API_BASE_URLS } from '../config.env';

export interface EnvironmentTemplate {
  id: string;
  env_name: string;
  [key: string]: any;
}

export interface EnvironmentTemplatesResponse {
  data: EnvironmentTemplate[];
}

/**
 * Fetches environment templates for an organization
 */
export async function getEnvTemplates(
  orgId: number | string
): Promise<EnvironmentTemplate[]> {
  const response = await get<EnvironmentTemplatesResponse>(
    `/api/v1/organizations/${orgId}/environment-templates`,
    undefined,
    API_BASE_URLS.devOps
  );
  return response.data;
}

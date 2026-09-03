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

/**
 * The gateway's functionality type. Mirrors platform-api's `functionalityType`
 * (`regular | ai | event`); this UI only offers `ai`/`event` on create, but a
 * gateway created elsewhere can come back as `regular`, so the type tolerates it.
 */
export type GatewayType = 'regular' | 'ai' | 'event';

export type GatewayStatus = 'active' | 'inactive';

export type Environment = {
  id: string;
  name: string;
};

export type Gateway = {
  id: string;
  name: string;
  description?: string;
  type: GatewayType;
  environmentId: string;
  /** The external host the gateway is exposed on — server-assigned, not a create input. */
  url: string;
  status: GatewayStatus;
  updatedAt: string;
};

/**
 * Fields the create/edit form collects. `id`/`url` (host) are server-assigned;
 * on edit only `name`/`description` are mutable (`type`/`environmentId` are
 * fixed at creation).
 */
export type GatewayInput = {
  name: string;
  description?: string;
  type: GatewayType;
  environmentId: string;
};

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

import type { GatewayVersionEntry } from '@/config/runtime';
import type { GatewayFunctionality } from './gatewayDisplay';

/**
 * Lookups over the gateway release catalog — the BFF's `[[gateway_versions]]`
 * config, surfaced as `runtimeConfig.gatewayVersions`. The catalog is an
 * argument rather than read here, so callers and tests choose what it holds.
 *
 * The config lists releases newest first, so the first entry of a type is the
 * one the console offers as "Latest".
 */

/** The releases offered for one gateway type, newest first. */
export const versionsForType = (
  catalog: readonly GatewayVersionEntry[],
  type: GatewayFunctionality,
): GatewayVersionEntry[] => catalog.filter((entry) => entry.type === type);

/**
 * The release a gateway was created against.
 *
 * A gateway that reports a version is matched by that version only: falling
 * back to another release would hand out a download for a build the gateway is
 * not running. Only a gateway with no version takes its type's latest release.
 */
export const findVersionEntry = (
  catalog: readonly GatewayVersionEntry[],
  type: GatewayFunctionality,
  version?: string,
): GatewayVersionEntry | undefined => {
  const reported = version?.trim();
  return reported
    ? catalog.find((entry) => entry.type === type && entry.version === reported)
    : catalog.find((entry) => entry.type === type);
};

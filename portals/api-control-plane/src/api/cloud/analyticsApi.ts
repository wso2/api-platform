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
 * Client for WSO2 Cloud Moesif analytics endpoints (wso2cloud platform-api).
 * Requests go through the portal BFF same-origin proxy (/proxy/cloud/...).
 */

import { runtimeConfig } from '../../config/runtime';

type CollectorKeyResponse = {
  moesifKey?: string;
};

type ApiErrorPayload = {
  error?: string;
  message?: string;
};

export class MoesifCollectorKeyUnavailableError extends Error {
  constructor(
    message = 'Moesif collector key is not available for this organization.'
  ) {
    super(message);
    this.name = 'MoesifCollectorKeyUnavailableError';
  }
}

const cloudApiBase = () => runtimeConfig.platformApiBaseUrl.replace(/\/$/, '');

const readJson = async <T>(response: Response): Promise<T> => {
  const payload = (await response.json().catch(() => ({}))) as T &
    ApiErrorPayload;
  if (!response.ok) {
    if (response.status === 404 || response.status === 503) {
      throw new MoesifCollectorKeyUnavailableError();
    }
    const detail =
      payload.error ||
      payload.message ||
      `Cloud analytics request failed (${response.status})`;
    throw new Error(detail);
  }
  return payload;
};

/**
 * Collector JWTs / application ids are pasted into shell heredocs and Helm
 * `--set` values. Reject anything outside a conservative, single-line grammar
 * so newlines or shell metacharacters cannot alter the copied command.
 */
const SHELL_SAFE_COLLECTOR_KEY = /^[A-Za-z0-9._+/-]+$/;

export function isShellSafeCollectorKey(key: string): boolean {
  return SHELL_SAFE_COLLECTOR_KEY.test(key);
}

/**
 * GET /cloud/analytics/internal/collector-key?env={envId}
 * Returns the full Moesif collector JWT for gateway event publishing.
 */
export async function fetchCollectorKey(
  envId = runtimeConfig.moesifEnvId
): Promise<string> {
  const trimmedEnv = envId.trim();
  if (!trimmedEnv) {
    throw new Error('Moesif environment id is required.');
  }

  const response = await fetch(
    `${cloudApiBase()}/cloud/analytics/internal/collector-key?env=${encodeURIComponent(trimmedEnv)}`,
    {
      credentials: 'include',
      headers: { accept: 'application/json' },
    }
  );
  const payload = await readJson<CollectorKeyResponse>(response);
  const key = payload.moesifKey?.trim();
  if (!key) {
    throw new MoesifCollectorKeyUnavailableError(
      'Collector key missing from cloud analytics response.'
    );
  }
  if (!isShellSafeCollectorKey(key)) {
    throw new MoesifCollectorKeyUnavailableError(
      'Collector key from cloud analytics has an unexpected format.'
    );
  }
  return key;
}

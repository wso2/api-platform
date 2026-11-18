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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { http, type RequestOptions } from '../../core/http';
import type { BodyOf, ResponseOf } from '../../core/spec';

/**
 * Transport layer for `/webhook-secrets`, the org's secrets for signed webhook
 * events from its API Portals. A portal signs every event it delivers to the
 * control plane with one of these; the control plane finds the secret from the
 * hash the event carries.
 *
 * The plaintext comes back once, on create and on rotate, and never again: a
 * read returns the secret's metadata and masked versions only.
 */

export type WebhookSecret = ResponseOf<'getWebhookSecret'>;
export type WebhookSecretWithValue = ResponseOf<'createWebhookSecret'>;
export type CreateWebhookSecretBody = BodyOf<'createWebhookSecret'>;

const WEBHOOK_SECRETS_BASE = '/webhook-secrets';

/** URL-encoded path for one secret. Ids are user-supplied - always encode. */
const resourcePath = (secretId: string): string =>
  `${WEBHOOK_SECRETS_BASE}/${encodeURIComponent(secretId)}`;

export const getWebhookSecret = async (
  secretId: string,
  options?: RequestOptions,
): Promise<WebhookSecret> =>
  http.get<WebhookSecret>(resourcePath(secretId), {
    ...options,
    operationName: 'getWebhookSecret',
  });

export const createWebhookSecret = async (
  body: CreateWebhookSecretBody,
  options?: RequestOptions,
): Promise<WebhookSecretWithValue> =>
  http.post<WebhookSecretWithValue>(WEBHOOK_SECRETS_BASE, body, {
    ...options,
    operationName: 'createWebhookSecret',
  });

/**
 * Issues a new version of the secret. Every retained version stays valid, so a
 * portal still configured with an older one keeps delivering until it is
 * updated.
 */
export const rotateWebhookSecret = async (
  secretId: string,
  options?: RequestOptions,
): Promise<WebhookSecretWithValue> =>
  http.post<WebhookSecretWithValue>(`${resourcePath(secretId)}/rotate`, undefined, {
    ...options,
    operationName: 'rotateWebhookSecret',
  });

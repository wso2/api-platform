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

import { http, type RequestOptions } from '../../core/http';
import type { BodyOf, QueryOf, ResponseOf, Schema } from '../../core/spec';

/**
 * Transport layer for `/meters`: what paid subscription plans charge for, either
 * the requests the gateway allows or the tokens of AI API requests, added up from
 * the meter's event key. Each filter field a meter lists carries the event key it
 * reads, and each plan price that uses a meter gives its own filters. Meters are
 * kept in the Platform API and checked with the API Portal's rules and messages.
 * A meter reaches an API Portal only when a plan that charges for it is published
 * there.
 */

export type Meter = Schema<'Meter'>;
export type MeterListResponse = ResponseOf<'ListMeters'>;
export type ListMetersQuery = QueryOf<'ListMeters'>;
export type MeterBody = BodyOf<'CreateMeter'>;

const BASE = '/meters';

const resourcePath = (meterId: string): string => `${BASE}/${encodeURIComponent(meterId)}`;

export const listMeters = async (options?: RequestOptions): Promise<MeterListResponse> => {
  return http.get<MeterListResponse>(BASE, { ...options, operationName: 'ListMeters' });
};

export const getMeter = async (meterId: string, options?: RequestOptions): Promise<Meter> => {
  return http.get<Meter>(resourcePath(meterId), { ...options, operationName: 'GetMeter' });
};

export const createMeter = async (body: MeterBody, options?: RequestOptions): Promise<Meter> => {
  return http.post<Meter>(BASE, body, { ...options, operationName: 'CreateMeter' });
};

/**
 * Changes a meter. Once any plan prices it, every change is refused (409
 * `METER_IN_USE`, with a message that counts the plans), its name and
 * description included.
 */
export const updateMeter = async (
  meterId: string,
  body: MeterBody,
  options?: RequestOptions
): Promise<Meter> => {
  return http.put<Meter>(resourcePath(meterId), body, { ...options, operationName: 'UpdateMeter' });
};

/**
 * Deletes a meter. It is refused while a plan prices it (409 `METER_IN_USE`).
 * The delete is not sent to API Portals, which keep their copies.
 */
export const deleteMeter = async (meterId: string, options?: RequestOptions): Promise<void> => {
  await http.delete<void>(resourcePath(meterId), { ...options, operationName: 'DeleteMeter' });
};

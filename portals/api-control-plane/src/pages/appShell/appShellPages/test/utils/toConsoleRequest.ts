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

import type { ConsoleAutoParam, ConsoleSendRequest } from '@/components/ApiTestConsole';
import {
  rawFormatFor,
  type ConsoleRequest,
  type KeyValueRow,
} from './types';

/**
 * Mirrors the test console's request into the cURL builder's model.
 *
 * Credentials the page injects itself (the test key) are left out here on
 * purpose: `withTarget` re-applies them to whichever list the API's policy
 * names, and a copy that arrived here as an ordinary row would be duplicated.
 * `Content-Type` is left out too, because the model derives it from the body.
 */

let rowSequence = 0;
const row = (prefix: string, name: string, value: string): KeyValueRow => {
  rowSequence += 1;
  return { enabled: true, id: `${prefix}-${rowSequence}`, name, value };
};

export const toConsoleRequest = (
  request: ConsoleSendRequest,
  autoHeaders: readonly ConsoleAutoParam[],
  autoQuery: readonly ConsoleAutoParam[],
): ConsoleRequest => {
  const autoHeaderNames = new Set(autoHeaders.map((header) => header.name.toLowerCase()));
  const autoQueryNames = new Set(autoQuery.map((param) => param.name));

  const base: ConsoleRequest = {
    baseUrl: request.baseUrl,
    body: '',
    bodyMode: 'none',
    formFields: [],
    headers: request.headers
      .filter(([name]) => name.toLowerCase() !== 'content-type' && !autoHeaderNames.has(name.toLowerCase()))
      .map(([name, value]) => row('h', name, value)),
    method: request.method,
    path: request.path,
    queryParams: request.query
      .filter(([name]) => !autoQueryNames.has(name))
      .map(([name, value]) => row('q', name, value)),
    rawFormat: 'json',
  };

  const { body } = request;
  if (!body) return base;

  switch (body.kind) {
    case 'text':
      return { ...base, body: body.text, bodyMode: 'raw', rawFormat: rawFormatFor(body.mediaType) };
    case 'form':
      return {
        ...base,
        bodyMode: 'url-encoded',
        formFields: body.fields.map(([name, value]) => row('f', name, value)),
      };
    case 'multipart':
      // Files cannot be expressed in the model; their text fields can.
      return {
        ...base,
        bodyMode: 'form-data',
        formFields: body.parts.flatMap((part) => ('value' in part ? [row('f', part.name, part.value)] : [])),
      };
    case 'binary':
      return base;
  }
};

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

import type { IntlShape } from 'react-intl';

import {
  ConsoleTransportError,
  type ConsoleBody,
  type ConsoleSendRequest,
  type ConsoleSendResult,
  type ConsoleTransport,
} from '../types';
import { headerProblem, resolveTarget } from '../request/safety';
import { transportMessages } from './messages';
import {
  decodeBody,
  MAX_DISPLAY_BYTES,
  MAX_REQUEST_BODY_BYTES,
  readLimited,
  requestBodySize,
} from './response';

/**
 * Sends the request from the browser straight to the gateway.
 *
 * Hardened on purpose, because the browser is the one making the call:
 *
 * - the target is proven to be the gateway (`resolveTarget`) — same origin,
 *   under the gateway's path — before anything is sent;
 * - no cookies or ambient credentials go with it (`credentials: 'omit'`) and no
 *   Referer leaks the portal's address;
 * - redirects are never followed, so a response cannot steer the console to a
 *   different host;
 * - forbidden or malformed headers are refused up front;
 * - a deadline bounds how long a stuck gateway can hold the console;
 * - the response is read up to a ceiling and no further.
 */

export const DIRECT_TIMEOUT_MS = 30_000;

const toFetchBody = (body: ConsoleBody | undefined, intl: IntlShape): BodyInit | undefined => {
  if (!body) return undefined;
  if (requestBodySize(body) > MAX_REQUEST_BODY_BYTES) {
    throw new ConsoleTransportError(intl.formatMessage(transportMessages.bodyTooLarge));
  }
  switch (body.kind) {
    case 'text':
      return body.text;
    case 'form':
      return new URLSearchParams(body.fields);
    case 'multipart': {
      const form = new FormData();
      body.parts.forEach((part) => {
        if ('file' in part) form.append(part.name, part.file, part.file.name);
        else form.append(part.name, part.value);
      });
      return form;
    }
    case 'binary':
      return body.file;
  }
};

export const createDirectTransport =
  (intl: IntlShape): ConsoleTransport =>
  async (request: ConsoleSendRequest, signal: AbortSignal): Promise<ConsoleSendResult> => {
    const target = resolveTarget(request.baseUrl, request.path);
    if (!target) {
      throw new ConsoleTransportError(intl.formatMessage(transportMessages.invalidTarget));
    }
    request.query.forEach(([name, value]) => target.searchParams.append(name, value));

    const headers = new Headers();
    request.headers.forEach(([name, value]) => {
      if (headerProblem(name, value)) {
        throw new ConsoleTransportError(intl.formatMessage(transportMessages.invalidHeader));
      }
      headers.append(name, value);
    });

    const body = toFetchBody(request.body, intl);

    const controller = new AbortController();
    let timedOut = false;
    const onAbort = () => controller.abort();
    if (signal.aborted) controller.abort();
    signal.addEventListener('abort', onAbort, { once: true });
    const timer = setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, DIRECT_TIMEOUT_MS);

    const started = performance.now();
    try {
      const response = await fetch(target, {
        body,
        cache: 'no-store',
        credentials: 'omit',
        headers,
        method: request.method,
        mode: 'cors',
        redirect: 'manual',
        referrerPolicy: 'no-referrer',
        signal: controller.signal,
      });

      if (response.type === 'opaqueredirect' || (response.status >= 300 && response.status < 400)) {
        throw new ConsoleTransportError(intl.formatMessage(transportMessages.redirect));
      }

      const read = await readLimited(response, MAX_DISPLAY_BYTES);
      const decoded = decodeBody(read.bytes, response.headers.get('content-type'), read.truncated);

      return {
        binary: decoded.binary,
        bodyText: decoded.text,
        durationMs: Math.round(performance.now() - started),
        headers: Array.from(response.headers.entries()),
        sizeBytes: read.totalSeen,
        status: response.status,
        statusText: response.statusText,
        truncated: read.truncated,
        url: target.toString(),
      };
    } catch (error) {
      if (error instanceof ConsoleTransportError) throw error;
      if (timedOut) {
        throw new ConsoleTransportError(intl.formatMessage(transportMessages.timeout));
      }
      if (signal.aborted) throw new ConsoleTransportError('', { aborted: true });
      throw new ConsoleTransportError(intl.formatMessage(transportMessages.directUnreachable));
    } finally {
      clearTimeout(timer);
      signal.removeEventListener('abort', onAbort);
    }
  };

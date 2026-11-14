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

import { defineMessages, type IntlShape } from 'react-intl';

import { isApiError, type ApiError } from '@/api/core/errors';
import type { InvokeRestApiOnGatewayResponse } from '@/api/resources/restApis/apiGateways/apiGateways.endpoints';
import type { InvokeRestApiOnGatewayVariables } from '@/api/resources/restApis/apiGateways/apiGateways.hooks';
import {
  ConsoleTransportError,
  decodeBody,
  headerProblem,
  MAX_DISPLAY_BYTES,
  resolveTarget,
  type ConsoleBody,
  type ConsoleSendRequest,
  type ConsoleSendResult,
  type ConsoleTransport,
} from '@/components/ApiTestConsole';

/**
 * The console's "Through proxy" transport: relays a test request through the
 * Platform API's `POST /gateway-relay/invoke` instead of calling the gateway
 * from the browser.
 *
 * ## Why a relay at all
 *
 * The gateway has a different origin from the portal in every deployment, so a
 * browser call needs the API to define a `cors` policy. An HTTP gateway is
 * blocked as mixed content when the portal uses HTTPS, and a cluster-internal
 * gateway is not reachable from the user's machine. The relay avoids all three,
 * and returns every response header rather than only the CORS-safelisted ones.
 *
 * ## What it is careful about
 *
 * - The relay is told *which API and gateway* to call and the path within the
 *   API; the Platform API resolves the real target. The path is checked here
 *   first (`resolveTarget`), so a request is never relayed unless it provably
 *   stays on the selected gateway.
 * - A gateway that answered at all comes back as a normal response with its own
 *   status, so "your backend returned 502" and "the control plane could not
 *   reach your gateway" never look alike.
 * - The reply is an untrusted envelope: it is shape-checked before use, and the
 *   body is capped before it reaches the screen.
 * - Failures are mapped to our own wording by error code. The server's message
 *   text is never shown.
 *
 * The relay reports total time as the user experienced it (browser to Platform
 * API to gateway and back), not the gateway-only duration.
 */

const messages = defineMessages({
  busy: {
    id: 'apiControlPlane.pages.test.console.relayTransport.busy',
    defaultMessage:
      'The portal is handling too many test requests right now. Try again in a moment.',
    description: 'Shown when the portal-side relay is at capacity. Not a gateway failure.',
  },
  apiNotFound: {
    id: 'apiControlPlane.pages.test.console.relayTransport.apiNotFound',
    defaultMessage: 'This API could not be found. Try reloading the page.',
    description: 'Shown when the control plane does not return the API being tested.',
  },
  gatewayNoEndpoint: {
    id: 'apiControlPlane.pages.test.console.relayTransport.gatewayNoEndpoint',
    defaultMessage: 'The selected gateway has no endpoint configured.',
  },
  gatewayNotDeployed: {
    id: 'apiControlPlane.pages.test.console.relayTransport.gatewayNotDeployed',
    defaultMessage:
      'This API is not deployed to the selected gateway. Deploy it first, then try again.',
    description: 'Actionable: the gateway exists but the API has not been deployed to it.',
  },
  gatewayUnsupportedScheme: {
    id: 'apiControlPlane.pages.test.console.relayTransport.gatewayUnsupportedScheme',
    defaultMessage:
      'The selected gateway’s endpoint is not an HTTP address, so it cannot be called from here.',
  },
  invalidRequest: {
    id: 'apiControlPlane.pages.test.console.relayTransport.invalidRequest',
    defaultMessage:
      'This request cannot be sent as written. Check the path, headers and parameters.',
    description: 'Shown when the portal refused to send the request the form describes.',
  },
  noTarget: {
    id: 'apiControlPlane.pages.test.console.relayTransport.noTarget',
    defaultMessage: 'Select a deployed gateway before sending a request.',
  },
  sessionExpired: {
    id: 'apiControlPlane.pages.test.console.relayTransport.sessionExpired',
    defaultMessage: 'Your session has expired. Sign in again to continue testing.',
  },
  targetNotAllowed: {
    id: 'apiControlPlane.pages.test.console.relayTransport.targetNotAllowed',
    defaultMessage:
      'The portal is not configured to reach this gateway. Ask an administrator to allow it.',
    description:
      'Shown only when the egress policy refuses the target — an operator setting, not something the user can fix. Deliberately vague about the policy itself.',
  },
  tooLarge: {
    id: 'apiControlPlane.pages.test.console.relayTransport.tooLarge',
    defaultMessage:
      'This request is too large to send from the console. Use the cURL command instead.',
  },
  unreachable: {
    id: 'apiControlPlane.pages.test.console.relayTransport.unreachable',
    defaultMessage: 'The portal could not reach this gateway.',
    description:
      'Shown when the relay itself failed. Deliberately distinct from a gateway that answered with an error status.',
  },
  unexpected: {
    id: 'apiControlPlane.pages.test.console.relayTransport.unexpected',
    defaultMessage: 'The request could not be sent. Try again.',
  },
  upstreamTimeout: {
    id: 'apiControlPlane.pages.test.console.relayTransport.upstreamTimeout',
    defaultMessage: 'The gateway did not respond in time.',
  },
});

/** The invoke operation's error codes, and the console's own, mapped to their wording. */
const ERROR_MESSAGES: Record<string, (typeof messages)[keyof typeof messages]> = {
  REST_API_NOT_FOUND: messages.apiNotFound,
  REST_API_INVOKE_GATEWAY_NOT_DEPLOYED: messages.gatewayNotDeployed,
  REST_API_INVOKE_GATEWAY_NO_ENDPOINT: messages.gatewayNoEndpoint,
  REST_API_INVOKE_GATEWAY_UNSUPPORTED_SCHEME: messages.gatewayUnsupportedScheme,
  REST_API_INVOKE_REQUEST_INVALID: messages.invalidRequest,
  VALIDATION_FAILED: messages.invalidRequest,
  REST_API_INVOKE_BUSY: messages.busy,
  PAYLOAD_TOO_LARGE: messages.tooLarge,
  REQUEST_TOO_LARGE: messages.tooLarge,
  UNAUTHORIZED: messages.sessionExpired,
  REST_API_INVOKE_TARGET_NOT_ALLOWED: messages.targetNotAllowed,
  REST_API_INVOKE_UPSTREAM_TIMEOUT: messages.upstreamTimeout,
  REST_API_INVOKE_UPSTREAM_UNREACHABLE: messages.unreachable,
};

/** What the Platform API returns when the relay reached the gateway. */
type RelayResult = InvokeRestApiOnGatewayResponse;

/** Everything the relay needs to send one request. */
export type RelayContext = {
  /** Sends the invoke request: `useInvokeRestApiOnGateway`'s `mutateAsync`. */
  invoke: (variables: InvokeRestApiOnGatewayVariables) => Promise<InvokeRestApiOnGatewayResponse>;
  restApiId: string;
  gatewayId: string;
  intl: IntlShape;
};

const localized = (intl: IntlShape, code: string | undefined): string =>
  intl.formatMessage(ERROR_MESSAGES[code ?? ''] ?? messages.unexpected);

/**
 * The message for a request that was not relayed. One that never reached the
 * Platform API, or timed out on the way, has no code of its own.
 */
const relayErrorMessage = (intl: IntlShape, error: ApiError): string => {
  if (error.kind === 'network') return intl.formatMessage(messages.unreachable);
  if (error.kind === 'timeout') return intl.formatMessage(messages.upstreamTimeout);
  return localized(intl, error.code);
};

/**
 * Decodes a base64 body into bytes without assuming a Node Buffer exists.
 *
 */
const base64ToBytes = (value: string) => {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
  return bytes;
};

/**
 * Browser-side limit for binary bodies, checked before reading any bytes.
 *
 * The Platform API's `gateway_relay.max_request_bytes` remains authoritative.
 * This higher limit prevents large attachments from being loaded and
 * base64-encoded in memory.
 */
const MAX_BINARY_BODY_BYTES = 16 * 1024 * 1024;

/** Rough per-part cost of a multipart boundary and its headers. */
const MULTIPART_PART_OVERHEAD_BYTES = 256;

/** Raised before reading, and localized like an oversized request the Platform API refuses. */
const bodyTooLargeError = () =>
  Object.assign(new Error('request body exceeds the console limit'), {
    code: 'REQUEST_TOO_LARGE',
  });

/** Rejects text whose UTF-8 size is over the console limit; character count is not bytes. */
const assertTextWithinLimit = (text: string): void => {
  if (new TextEncoder().encode(text).length > MAX_BINARY_BODY_BYTES) throw bodyTooLargeError();
};

/**
 * Normalizes the request body for transport in the invoke request.
 *
 * Multipart bodies require an explicit boundary because the Platform API,
 * rather than the browser, sends the request. Attachment sizes are checked before reading.
 */
export const encodeRequestBody = async (
  body: unknown,
): Promise<{ body: string; bodyEncoding: 'utf8' | 'base64'; contentType?: string }> => {
  if (body === undefined || body === null || body === '') {
    return { body: '', bodyEncoding: 'utf8' };
  }
  if (typeof body === 'string') {
    assertTextWithinLimit(body);
    return { body, bodyEncoding: 'utf8' };
  }
  if (body instanceof URLSearchParams) {
    const text = body.toString();
    assertTextWithinLimit(text);
    return { body: text, bodyEncoding: 'utf8' };
  }
  if (typeof FormData !== 'undefined' && body instanceof FormData) {
    const { bytes, contentType } = await encodeMultipart(body);
    return { ...bytesToPayload(bytes), contentType };
  }
  if (body instanceof Blob) {
    // `size` is metadata; reading it does not pull the file into memory.
    if (body.size > MAX_BINARY_BODY_BYTES) throw bodyTooLargeError();
    return bytesToPayload(new Uint8Array(await body.arrayBuffer()));
  }
  if (body instanceof ArrayBuffer) {
    if (body.byteLength > MAX_BINARY_BODY_BYTES) throw bodyTooLargeError();
    return bytesToPayload(new Uint8Array(body));
  }
  if (ArrayBuffer.isView(body)) {
    if (body.byteLength > MAX_BINARY_BODY_BYTES) throw bodyTooLargeError();
    return bytesToPayload(new Uint8Array(body.buffer, body.byteOffset, body.byteLength));
  }
  // Any other structure: sending its JSON is closer to what the user entered than dropping it.
  return { body: JSON.stringify(body), bodyEncoding: 'utf8' };
};

/**
 * Serialises a `FormData` into multipart bytes plus the matching Content-Type.
 *
 * Done by hand rather than by round-tripping through `Request`, because that
 * relies on the runtime serialising a `FormData` body — which browsers do and
 * jsdom does not, so the shortcut would have produced `[object FormData]` in
 * tests while appearing to work in the app. Explicit assembly behaves the same
 * everywhere and is the thing worth testing anyway.
 *
 * Field and file names are escaped per the WHATWG form-data algorithm; a quote
 * or newline in a name would otherwise let the part header be rewritten.
 */
const encodeMultipart = async (
  form: FormData,
): Promise<{ bytes: Uint8Array; contentType: string }> => {
  // Sized before any file is read: File.size is metadata, and text is counted
  // in UTF-8 bytes (not characters) together with its name.
  const sizer = new TextEncoder();
  let declared = 0;
  for (const [name, value] of form.entries()) {
    declared += MULTIPART_PART_OVERHEAD_BYTES + sizer.encode(name).length;
    declared +=
      typeof value === 'string'
        ? sizer.encode(value).length
        : value.size + sizer.encode(value.name).length;
    if (declared > MAX_BINARY_BODY_BYTES) throw bodyTooLargeError();
  }

  const boundary = `----apicpTestConsole${randomBoundaryToken()}`;
  const encoder = new TextEncoder();
  const chunks: Uint8Array[] = [];

  for (const [name, value] of form.entries()) {
    let header = `--${boundary}\r\nContent-Disposition: form-data; name="${escapeFormName(name)}"`;
    if (typeof value === 'string') {
      chunks.push(encoder.encode(`${header}\r\n\r\n${value}\r\n`));
      continue;
    }
    header += `; filename="${escapeFormName(value.name)}"\r\nContent-Type: ${
      value.type || 'application/octet-stream'
    }`;
    chunks.push(encoder.encode(`${header}\r\n\r\n`));
    chunks.push(new Uint8Array(await value.arrayBuffer()));
    chunks.push(encoder.encode('\r\n'));
  }
  chunks.push(encoder.encode(`--${boundary}--\r\n`));

  const total = chunks.reduce((sum, chunk) => sum + chunk.length, 0);
  // The framing is real now, so check the assembled payload as well.
  if (total > MAX_BINARY_BODY_BYTES) throw bodyTooLargeError();
  const bytes = new Uint8Array(total);
  let offset = 0;
  chunks.forEach((chunk) => {
    bytes.set(chunk, offset);
    offset += chunk.length;
  });

  return { bytes, contentType: `multipart/form-data; boundary=${boundary}` };
};

/** A boundary token unlikely to appear inside any part's content. */
const randomBoundaryToken = (): string => {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
};

/** WHATWG form-data name escaping — the three characters that break a part header. */
const escapeFormName = (name: string): string =>
  name.replace(/\r/g, '%0D').replace(/\n/g, '%0A').replace(/"/g, '%22');

/** Bytes as UTF-8 when they decode cleanly, base64 otherwise. */
const bytesToPayload = (bytes: Uint8Array): { body: string; bodyEncoding: 'utf8' | 'base64' } => {
  try {
    // `ignoreBOM` keeps a leading U+FEFF; by default the decoder drops it, which would alter an uploaded file.
    const text = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(bytes);
    return { body: text, bodyEncoding: 'utf8' };
  } catch {
    let binary = '';
    bytes.forEach((byte) => {
      binary += String.fromCharCode(byte);
    });
    return { body: btoa(binary), bodyEncoding: 'base64' };
  }
};

/**
 * Narrows the relay's reply to the envelope this module knows how to convert.
 *
 * The body is cast, not trusted: a 200 carrying something else entirely is a
 * realistic outcome when an intermediary — a corporate proxy, a service-mesh
 * error page, a misconfigured ingress; answers in the Platform API's place. Without
 * this, a non-array `headers` reaches `forEach` and surfaces in the console as
 * a raw TypeError rather than as a message anyone can act on.
 */
const isRelayResult = (value: unknown): value is RelayResult => {
  if (typeof value !== 'object' || value === null) return false;
  const { outcome, response } = value as { outcome?: unknown; response?: unknown };
  if (outcome !== 'response') return false;
  if (typeof response !== 'object' || response === null) return false;
  const { status, statusText, headers, body, bodyEncoding, truncated } = response as {
    status?: unknown;
    statusText?: unknown;
    headers?: unknown;
    body?: unknown;
    bodyEncoding?: unknown;
    truncated?: unknown;
  };

  return (
    typeof status === 'number' &&
    typeof statusText === 'string' &&
    typeof body === 'string' &&
    (bodyEncoding === 'utf8' || (bodyEncoding === 'base64' && isBase64(body))) &&
    (truncated === undefined || typeof truncated === 'boolean') &&
    Array.isArray(headers) &&
    headers.every(
      (header) =>
        typeof header === 'object' &&
        header !== null &&
        typeof header.name === 'string' &&
        typeof header.value === 'string',
    )
  );
};

/** Whether text is canonical base64, so `atob` cannot throw on it. */
const isBase64 = (value: string): boolean =>
  value.length % 4 === 0 && /^[A-Za-z0-9+/]*={0,2}$/.test(value);

/** Converts the console's body into what `encodeRequestBody` accepts. */
const bodyInput = (body: ConsoleBody | undefined): unknown => {
  if (!body) return undefined;
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

/**
 * The relay's reply as the console's result.
 *
 * The body is decoded and capped like any other response; `url` is the call
 * the user made (no query string, which can carry a credential), not the relay
 * endpoint.
 */
export const toSendResult = (
  result: RelayResult,
  url: string,
  durationMs: number,
): ConsoleSendResult => {
  const { response } = result;
  const contentType =
    response.headers.find(({ name }) => name.toLowerCase() === 'content-type')?.value ?? null;

  const bytes =
    response.bodyEncoding === 'base64'
      ? base64ToBytes(response.body)
      : new TextEncoder().encode(response.body);
  // Cut here, or already cut by the Platform API before it answered.
  const truncated = bytes.length > MAX_DISPLAY_BYTES || response.truncated === true;
  const decoded = decodeBody(
    bytes.length > MAX_DISPLAY_BYTES ? bytes.slice(0, MAX_DISPLAY_BYTES) : bytes,
    contentType,
    truncated,
  );

  return {
    binary: decoded.binary,
    bodyText: decoded.text,
    durationMs,
    headers: response.headers.map(({ name, value }) => [name, value] as [string, string]),
    sizeBytes: bytes.length,
    status: response.status,
    statusText: response.statusText,
    truncated,
    url,
  };
};

/** Builds the transport the console uses in "Through proxy" mode. */
export const createRelayTransport =
  (context: RelayContext): ConsoleTransport =>
  async (request: ConsoleSendRequest, signal: AbortSignal): Promise<ConsoleSendResult> => {
    const { intl } = context;

    if (!context.restApiId || !context.gatewayId) {
      throw new ConsoleTransportError(intl.formatMessage(messages.noTarget));
    }
    // Refuses anything that would leave the gateway before the Platform API sees it.
    if (!resolveTarget(request.baseUrl, request.path)) {
      throw new ConsoleTransportError(intl.formatMessage(messages.invalidRequest));
    }
    if (request.headers.some(([name, value]) => headerProblem(name, value))) {
      throw new ConsoleTransportError(intl.formatMessage(messages.invalidRequest));
    }

    let encoded: Awaited<ReturnType<typeof encodeRequestBody>>;
    try {
      encoded = await encodeRequestBody(bodyInput(request.body));
    } catch (error) {
      throw new ConsoleTransportError(localized(intl, (error as { code?: string }).code));
    }

    let headers = request.headers.map(([name, value]) => ({ name, value }));
    if (encoded.contentType) {
      // Multipart only: the boundary must match the body, so it replaces any
      // Content-Type rather than sitting beside it as a second header.
      headers = headers.filter((header) => header.name.toLowerCase() !== 'content-type');
      headers.push({ name: 'Content-Type', value: encoded.contentType });
    }

    const started = performance.now();
    let parsed: unknown;
    try {
      parsed = await context.invoke({
        body: {
          restApiId: context.restApiId,
          gatewayId: context.gatewayId,
          method: request.method,
          path: request.path,
          query: request.query.map(([name, value]) => ({ name, value })),
          headers,
          body: encoded.body,
          bodyEncoding: encoded.bodyEncoding,
        },
        signal,
      });
    } catch (error) {
      // An aborted request is the user cancelling, not a failure to relabel.
      if (signal.aborted || (isApiError(error) && error.kind === 'aborted')) {
        throw new ConsoleTransportError('', { aborted: true });
      }
      throw new ConsoleTransportError(
        isApiError(error)
          ? relayErrorMessage(intl, error)
          : intl.formatMessage(messages.unexpected),
      );
    }
    if (!isRelayResult(parsed)) {
      throw new ConsoleTransportError(intl.formatMessage(messages.unexpected));
    }
    return toSendResult(
      parsed,
      `${request.baseUrl}${request.path}`,
      Math.round(performance.now() - started),
    );
  };

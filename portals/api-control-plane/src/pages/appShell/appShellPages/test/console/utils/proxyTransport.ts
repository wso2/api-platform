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

import { CSRF_HEADER, CSRF_HEADER_VALUE } from '@/contexts/auth/authConstants';
import { splitAgainstBase } from './swaggerRequest';

/**
 * Sends the console's try-out requests through the BFF instead of straight at
 * the gateway, without changing the request the console displays.
 *
 * ## Why a relay at all
 *
 * The gateway is a different origin from the portal in every deployment, so the
 * test key header triggers a CORS preflight that only succeeds if the API
 * happens to carry a `cors` policy. A plain-http gateway is additionally
 * blocked as mixed content from an https portal, and a cluster-internal gateway
 * is not routable from the user's machine at all. The relay removes all three,
 * and returns more than a direct call could: a browser can only read *simple*
 * response headers cross-origin unless the gateway opts in with
 * `Access-Control-Expose-Headers`, whereas the relay hands back every one.
 *
 * ## Why the displayed request is still the real one
 *
 * This swaps the transport, not the request. Swagger builds the real gateway
 * request, stores it (`setRequest`), runs `requestInterceptor` over it and
 * stores that too (`setMutatedRequest`) — and only then calls `userFetch` to
 * put it on the wire. Everything the UI reads for the Request URL, the curl
 * snippet and the header list comes from those two stored copies, so replacing
 * the last step alone leaves all of it untouched and truthful.
 *
 * Rewriting the request in `requestInterceptor` instead would have meant
 * un-rewriting it in each of those display paths, and would additionally have
 * broken `fromSwaggerRequest` — which refuses a URL that does not point at the
 * gateway, and would have silently frozen the cURL view.
 *
 * `userFetch` is a per-request hook swagger-client honours in place of the
 * global `fetch` (`http()` calls `request.userFetch || fetch`). Swagger UI does
 * not plumb it through its own config, so it is injected by wrapping the
 * `spec.executeRequest` action — a state-plugin wrapper, which renders nothing
 * and so is unaffected by the React-reconciliation problem that made
 * `wrapComponents` unusable here (see TestConsoleSpecViewer's own notes).
 *
 * One thing the relay cannot make faithful: Swagger measures the duration
 * itself, around the whole call, so the figure shown includes the browser's hop
 * to the BFF as well as the gateway round trip. The BFF reports the gateway
 * time separately in `durationMs`, but Swagger overwrites `duration` after the
 * transport returns, so what is displayed is the wait the user actually had.
 */

/** BFF-owned endpoint, mounted beside /api/login rather than under the proxy prefix. */
const INVOKE_URL = '/api/test-console/invoke';

const messages = defineMessages({
  busy: {
    id: 'apiControlPlane.pages.test.console.proxyTransport.busy',
    defaultMessage:
      'The portal is handling too many test requests right now. Try again in a moment.',
    description: 'Shown when the portal-side relay is at capacity. Not a gateway failure.',
  },
  invalidRequest: {
    id: 'apiControlPlane.pages.test.console.proxyTransport.invalidRequest',
    defaultMessage:
      'This request cannot be sent as written. Check the path, headers and parameters.',
    description: 'Shown when the portal refused to send the request the form describes.',
  },
  noTarget: {
    id: 'apiControlPlane.pages.test.console.proxyTransport.noTarget',
    defaultMessage: 'Select a deployed gateway before sending a request.',
  },
  sessionExpired: {
    id: 'apiControlPlane.pages.test.console.proxyTransport.sessionExpired',
    defaultMessage: 'Your session has expired. Sign in again to continue testing.',
  },
  targetNotAllowed: {
    id: 'apiControlPlane.pages.test.console.proxyTransport.targetNotAllowed',
    defaultMessage:
      'This API cannot be tested on the selected gateway. Check that it is still deployed there.',
  },
  tooLarge: {
    id: 'apiControlPlane.pages.test.console.proxyTransport.tooLarge',
    defaultMessage:
      'This request is too large to send from the console. Use the cURL command instead.',
  },
  unreachable: {
    id: 'apiControlPlane.pages.test.console.proxyTransport.unreachable',
    defaultMessage: 'The portal could not reach this gateway.',
    description:
      'Shown when the relay itself failed. Deliberately distinct from a gateway that answered with an error status.',
  },
  unexpected: {
    id: 'apiControlPlane.pages.test.console.proxyTransport.unexpected',
    defaultMessage: 'The request could not be sent. Try again.',
  },
  upstreamTimeout: {
    id: 'apiControlPlane.pages.test.console.proxyTransport.upstreamTimeout',
    defaultMessage: 'The gateway did not respond in time.',
  },
});

/** Maps each BFF failure code to its own wording. */
const ERROR_MESSAGES: Record<string, (typeof messages)[keyof typeof messages]> = {
  INVALID_REQUEST_BODY: messages.invalidRequest,
  INVALID_TEST_REQUEST: messages.invalidRequest,
  RELAY_BUSY: messages.busy,
  REQUEST_TOO_LARGE: messages.tooLarge,
  SESSION_EXPIRED: messages.sessionExpired,
  TARGET_NOT_ALLOWED: messages.targetNotAllowed,
  UPSTREAM_TIMEOUT: messages.upstreamTimeout,
  UPSTREAM_UNREACHABLE: messages.unreachable,
};

/** One header or query parameter on the wire. A list, because names repeat. */
type WirePair = { name: string; value: string };

/** What the BFF returns when the relay reached the gateway. */
type RelayResult = {
  outcome: 'response';
  response: {
    status: number;
    statusText: string;
    headers: WirePair[];
    body: string;
    bodyEncoding: 'utf8' | 'base64';
    truncated: boolean;
    durationMs: number;
  };
};

/**
 * Everything the relay needs that changes after mount.
 *
 * Read through a ref rather than captured, because swagger-ui-react builds its
 * system once and keeps the first `plugins` value it was given — a captured
 * gateway id would stay pinned to whichever gateway was selected at mount.
 */
export type RelayContext = {
  /** Invoke URL of the selected gateway — the base the displayed request uses. */
  baseUrl: string;
  /** Organization handle, forwarded so Platform API can scope the lookup. */
  orgHandle: string;
  restApiId: string;
  gatewayId: string;
  intl: IntlShape;
};

/** A ref so the caller can keep the context current without remounting. */
export type RelayContextRef = { readonly current: RelayContext };

/**
 * The subset of a `Response` that swagger-client's `serializeResponse` reads.
 *
 * A shim rather than a real `Response`, for two reasons: a constructed
 * `Response` reports an empty `url` (and the property is read-only), which
 * would drop the query string from the Request URL the console displays; and
 * its constructor rejects a body on 204/304, which a gateway may legitimately
 * return.
 */
type ResponseLike = {
  ok: boolean;
  url: string;
  status: number;
  statusText: string;
  headers: Headers;
  text: () => Promise<string>;
  blob: () => Promise<Blob>;
};

/** The swagger request object handed to `userFetch`, as far as we rely on it. */
type SwaggerRequest = {
  url?: string;
  method?: string;
  headers?: Record<string, unknown>;
  body?: unknown;
  signal?: AbortSignal;
};

const localized = (intl: IntlShape, code: string | undefined): string =>
  intl.formatMessage(ERROR_MESSAGES[code ?? ''] ?? messages.unexpected);

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
 * Normalizes whatever swagger put in `body` into something the envelope can
 * carry.
 *
 * `FormData` is the interesting case. Swagger deliberately strips its own
 * `Content-Type` for multipart so the browser can add the boundary — but the
 * browser is no longer the one sending, so the boundary has to be produced
 * here. Round-tripping through `Request` does exactly that and hands back the
 * matching header.
 */
export const encodeRequestBody = async (
  body: unknown,
): Promise<{ body: string; bodyEncoding: 'utf8' | 'base64'; contentType?: string }> => {
  if (body === undefined || body === null || body === '') {
    return { body: '', bodyEncoding: 'utf8' };
  }
  if (typeof body === 'string') {
    return { body, bodyEncoding: 'utf8' };
  }
  if (body instanceof URLSearchParams) {
    return { body: body.toString(), bodyEncoding: 'utf8' };
  }
  if (typeof FormData !== 'undefined' && body instanceof FormData) {
    const { bytes, contentType } = await encodeMultipart(body);
    return { ...bytesToPayload(bytes), contentType };
  }
  if (body instanceof Blob) {
    return bytesToPayload(new Uint8Array(await body.arrayBuffer()));
  }
  if (body instanceof ArrayBuffer) {
    return bytesToPayload(new Uint8Array(body));
  }
  if (ArrayBuffer.isView(body)) {
    return bytesToPayload(new Uint8Array(body.buffer, body.byteOffset, body.byteLength));
  }
  // Swagger keeps some request-body forms as plain objects; sending the JSON is
  // closer to what the user filled in than dropping the body entirely.
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
    const text = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
    return { body: text, bodyEncoding: 'utf8' };
  } catch {
    let binary = '';
    bytes.forEach((byte) => {
      binary += String.fromCharCode(byte);
    });
    return { body: btoa(binary), bodyEncoding: 'base64' };
  }
};

/** Swagger stores absent headers as null rather than removing the key. */
const toWirePairs = (headers: Record<string, unknown> | undefined): WirePair[] =>
  Object.entries(headers ?? {})
    .filter(([name, value]) => name.trim() !== '' && typeof value === 'string')
    .map(([name, value]) => ({ name, value: value as string }));

/**
 * Rebuilds the relayed response into the shape swagger-client expects.
 *
 * `url` is the real gateway URL, so the Request URL the console shows is the
 * one the user thinks they called — not the relay endpoint.
 */
export const toResponseLike = (result: RelayResult, gatewayUrl: string): ResponseLike => {
  const { response } = result;
  const headers = new Headers();
  response.headers.forEach(({ name, value }) => {
    try {
      headers.append(name, value);
    } catch {
      // A gateway can emit a header name the Headers API refuses. Dropping that
      // one header beats failing the whole response the user asked to see.
    }
  });

  const isBase64 = response.bodyEncoding === 'base64';
  const bytes = isBase64 ? base64ToBytes(response.body) : undefined;

  return {
    ok: response.status >= 200 && response.status < 300,
    url: gatewayUrl,
    status: response.status,
    statusText: response.statusText,
    headers,
    text: async () => (bytes ? new TextDecoder().decode(bytes) : response.body),
    blob: async () => new Blob([bytes ?? response.body]),
  };
};

/**
 * Builds the `userFetch` swagger-client calls in place of the global `fetch`.
 *
 * It receives the final, fully-built gateway request — the same object that has
 * already been stored for display — and relays it rather than sending it.
 */
export const createRelayFetch =
  (contextRef: RelayContextRef) =>
  async (_url: string, request: SwaggerRequest): Promise<ResponseLike> => {
    const context = contextRef.current;
    const gatewayUrl = typeof request.url === 'string' ? request.url : '';

    if (!context.restApiId || !context.gatewayId || !context.baseUrl) {
      throw new Error(context.intl.formatMessage(messages.noTarget));
    }

    // Reused from the cURL sync so both agree on what "within this API" means.
    // It also refuses a URL aimed at another origin, which is not this API's
    // request and must not be relayed anywhere.
    const split = splitAgainstBase(gatewayUrl, context.baseUrl);
    if (!split) {
      throw new Error(context.intl.formatMessage(messages.invalidRequest));
    }

    const encoded = await encodeRequestBody(request.body);
    let headers = toWirePairs(request.headers);
    if (encoded.contentType) {
      // Multipart only. Swagger strips its own Content-Type just before calling
      // userFetch so the browser can add the boundary, so there is normally
      // nothing to replace — but a duplicate would reach the gateway as two
      // Content-Type headers, so the invariant is enforced here rather than
      // assumed from swagger's internals.
      headers = headers.filter((header) => header.name.toLowerCase() !== 'content-type');
      headers.push({ name: 'Content-Type', value: encoded.contentType });
    }

    let res: Response;
    try {
      res = await fetch(INVOKE_URL, {
        method: 'POST',
        // The session cookie authenticates the envelope; it is never forwarded
        // on to the gateway.
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', [CSRF_HEADER]: CSRF_HEADER_VALUE },
        signal: request.signal,
        body: JSON.stringify({
          orgHandle: context.orgHandle,
          restApiId: context.restApiId,
          gatewayId: context.gatewayId,
          method: request.method ?? 'GET',
          path: split.path,
          query: split.query.map(([name, value]) => ({ name, value })),
          headers,
          body: encoded.body,
          bodyEncoding: encoded.bodyEncoding,
        }),
      });
    } catch (error) {
      // An aborted request is the user navigating away, not a failure worth
      // relabelling — swagger has its own handling for it. The signal is
      // consulted as well as the error's name because what `fetch` rejects
      // with on abort is not consistent across runtimes.
      if (request.signal?.aborted || (error as Error | undefined)?.name === 'AbortError') {
        throw error;
      }
      throw new Error(context.intl.formatMessage(messages.unreachable));
    }

    if (!res.ok) {
      // A failure of the relay, not of the gateway. The two are deliberately
      // distinguishable: a gateway that answered at all comes back as a 200
      // with its own status inside, so "your backend returned 502" and "the
      // portal could not reach your gateway" never look alike.
      const body = (await res.json().catch(() => ({}))) as { code?: string };
      throw new Error(localized(context.intl, body.code));
    }

    const result = (await res.json()) as RelayResult;
    return toResponseLike(result, gatewayUrl);
  };

/**
 * The swagger-ui plugin that installs the relay.
 *
 * `executeRequest`'s payload flows into swagger-client's `buildRequest`, which
 * copies `userFetch` onto the request it builds. Wrapping the action is the
 * only way in, since swagger-ui exposes no config key for it — and wrapping an
 * *action* touches nothing that renders.
 */
export const testConsoleRelayPlugin = (contextRef: RelayContextRef) => {
  const userFetch = createRelayFetch(contextRef);
  return () => ({
    statePlugins: {
      spec: {
        wrapActions: {
          executeRequest:
            (oriAction: (payload: Record<string, unknown>) => unknown) =>
            (payload: Record<string, unknown>) =>
              oriAction({ ...payload, userFetch }),
        },
      },
    },
  });
};

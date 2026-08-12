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

import axios, {
    AxiosError,
    AxiosResponse,
  type AxiosInstance,
  type AxiosRequestConfig,
  type InternalAxiosRequestConfig,
} from 'axios';

import { runtimeConfig } from '../../config/runtime';
import {
  CSRF_HEADER,
  CSRF_HEADER_VALUE,
} from '../../features/auth/authConstants';
import { ApiErrorKind, platformErrorFromBody, platformErrorFromTransport } from './errors';

/**
 * Per-request context carried on the axios config. Declared via module
 * augmentation so it is type-checked at every call site and readable from the
 * interceptors. org scope reaches the wire as an explicit, per-call value
 * rather than mutable instance state, which is what makes concurrent requests
 * across two organizations safe.
 */
declare module 'axios' {
  export interface AxiosRequestConfig {
    /**
     * Organization the request is scoped to. Sent as `X-Org-Id` for the BFF to
     * resolve and validate. Required for every organization-scoped endpoint;
     * omit only for genuinely global ones (`/organizations`).
     */
    orgId?: string;
    /** Client correlation id, generated per request and echoed onto errors. */
    requestId?: string;
    /** Human label for logs/telemetry, e.g. `ListRESTAPIs`. */
    operationName?: string;
  }
}

/* -------------------------------------------------------------------------- */
/* Configuration                                                               */
/* -------------------------------------------------------------------------- */

/** Header the platform API uses to resolve and validate the active org. */
export const ORG_HEADER = 'X-Org-Id';

/** Default per-request deadline. Long operations override it per call. */
export const DEFAULT_TIMEOUT_MS = 30_000;

/**
 * Base for every platform-api call: the BFF's same-origin proxy prefix plus the
 * spec's version segment. Both come from runtime config, so pointing at a new
 * API version is a deployment change, not a rebuild.
 */
export const platformApiBaseUrl = (): string =>
  `${runtimeConfig.platformApiBaseUrl}/api/${runtimeConfig.platformApiVersion}`;

const MUTATING_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

const newRequestId = (): string =>
  typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `${Date.now().toString(16)}-${Math.random().toString(16).slice(2, 10)}`;

/**
 * Listeners notified exactly once when the BFF reports the session is gone.
 * The transport must not import the auth provider (that would be a cycle and
 * would make the client un-testable), so it publishes an event instead and
 * `AuthProvider` subscribes.
 */
type SessionExpiredListener = () => void;
const sessionExpiredListeners = new Set<SessionExpiredListener>();

export const onSessionExpired = (listener: SessionExpiredListener): (() => void) => {
  sessionExpiredListeners.add(listener);
  return () => sessionExpiredListeners.delete(listener);
};

/**
 * Debounced so a dashboard firing eight parallel queries against a dead session
 * triggers one redirect to login, not eight.
 */
let sessionExpiredNotifiedAt = 0;

/**
 * Clears the debounce window so the next 401 notifies again.
 *
 * Test seam, like `resetHttpClient`: the window is module state, so without
 * this a spec that triggers a 401 would silently suppress the notification in
 * every spec that runs within the next few seconds — making the tests
 * order-dependent.
 */
export const resetSessionExpiryNotice = (): void => {
  sessionExpiredNotifiedAt = 0;
};

const notifySessionExpired = () => {
  const now = Date.now();
  if (now - sessionExpiredNotifiedAt < 3_000) return;
  sessionExpiredNotifiedAt = now;
  for (const listener of sessionExpiredListeners) listener();
};

const attachRequestContext = (
  config: InternalAxiosRequestConfig
): InternalAxiosRequestConfig => {
  const method = (config.method ?? 'get').toUpperCase();

  config.requestId ??= newRequestId();
  config.headers.set('Accept', 'application/json');
  config.headers.set('X-Request-Id', config.requestId);

  if (config.orgId) config.headers.set(ORG_HEADER, config.orgId);

  // The BFF requires this on every state-mutating request as its CSRF defense.
  // Safe because there is no CORS layer in front of it, so only same-origin
  // script can set a custom header at all.
  if (MUTATING_METHODS.has(method)) {
    config.headers.set(CSRF_HEADER, CSRF_HEADER_VALUE);
  }

  // Let the browser set the multipart boundary itself for uploads; axios only
  // needs to be told to stay out of the way.
  if (config.data !== undefined) {
    // FormData must NOT carry an explicit Content-Type: the browser has to set
    // it so it can append the multipart boundary. Setting it by hand produces
    // a body the server cannot split.
    if (config.data instanceof FormData) {
        config.headers.delete('Content-Type');
    } else {
        config.headers.set('Content-Type', 'application/json');
    }
  }

  return config;
};
//   if (error instanceof ApiError) return error;

//   if (axios.isCancel(error)) {
//     return new ApiError('Request was canceled', { kind: 'aborted', cause: error });
//   }

//   if (!axios.isAxiosError(error)) {
//     return new ApiError(
//       error instanceof Error ? error.message : 'Unexpected error',
//       { kind: 'http', cause: error }
//     );
//   }

//   const config = error.config as AxiosRequestConfig | undefined;
//   const operation =
//     config?.operationName ??
//     (config ? `${(config.method ?? 'get').toUpperCase()} ${config.url ?? ''}` : undefined);
//   const shared = { requestId: config?.requestId, operation, cause: error };

//   if (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT') {
//     return new ApiError('The request timed out. Please try again.', {
//       kind: 'timeout',
//       ...shared,
//     });
//   }

//   const response = error.response;
//   if (!response) {
//     return new ApiError(
//       'Unable to reach the server. Check your connection and try again.',
//       { kind: 'network', ...shared }
//     );
//   }

//   if (response.status === 401) notifySessionExpired();

//   const body: unknown = response.data;
//   if (isErrorEnvelope(body)) {
//     return new ApiError(body.message, {
//       kind: 'http',
//       status: response.status,
//       code: body.code,
//       fieldErrors: body.errors ?? [],
//       details: body.details as Record<string, unknown> | undefined,
//       trackingId: body.trackingId,
//       ...shared,
//     });
//   }

//   // Non-conforming body (proxy HTML page, empty 502). Never surface the raw
//   // body — it is markup at best and infrastructure detail at worst.
//   return new ApiError(`Request failed with status ${response.status}`, {
//     kind: 'http',
//     status: response.status,
//     ...shared,
//   });
// };

/* -------------------------------------------------------------------------- */
/* Deadline composed with the caller's signal                                  */
/* -------------------------------------------------------------------------- */

interface Deadline {
  signal: AbortSignal;
  /** Which of the two sources fired, once one has. */
  reason: () => ApiErrorKind | undefined;
  release: () => void;
}

/**
 * Produces a single signal that aborts when EITHER the caller's signal aborts
 * or the deadline elapses, and remembers which happened.
 *
 * Composed by hand rather than with `AbortSignal.any` + `AbortSignal.timeout`
 * because those need a browser baseline this app does not yet require, and
 * because we need to distinguish the two causes afterwards — `AbortSignal.any`
 * reports only that *something* aborted, which would make every timeout look
 * like a user-initiated cancellation and be silently swallowed.
 *
 * `release()` must run on every path: it clears the timer and detaches the
 * listener from the caller's signal, which typically outlives the request
 * (React Query reuses one per query observer), so a missed detach is a real
 * leak rather than a theoretical one.
 */
const withDeadline = (
  callerSignal: AbortSignal | undefined,
  timeoutMs: number,
): Deadline => {
  const controller = new AbortController();
  let reason: ApiErrorKind | undefined;

  const abortWith = (kind: ApiErrorKind) => {
    if (controller.signal.aborted) return;
    reason = kind;
    controller.abort();
  };

  const onCallerAbort = () => abortWith('aborted');

  // A signal already aborted before we were called must not start a request.
  if (callerSignal?.aborted) {
    abortWith('aborted');
  } else {
    callerSignal?.addEventListener('abort', onCallerAbort, { once: true });
  }

  const timer =
    timeoutMs > 0 && !controller.signal.aborted
      ? setTimeout(() => abortWith('timeout'), timeoutMs)
      : undefined;

  return {
    signal: controller.signal,
    reason: () => reason,
    release: () => {
      if (timer !== undefined) clearTimeout(timer);
      callerSignal?.removeEventListener('abort', onCallerAbort);
    },
  };
};

/* -------------------------------------------------------------------------- */
/* Query strings                                                               */
/* -------------------------------------------------------------------------- */

export type QueryValue =
  | string
  | number
  | boolean
  | null
  | undefined
  | ReadonlyArray<string | number | boolean>;

export type Query = Record<string, QueryValue>;

/**
 * Serializes query parameters, dropping `undefined`/`null` and repeating array
 * values as `key=a&key=b`.
 *
 * Dropping empties matters for cache correctness as much as for tidiness: a
 * stray `?limit=undefined` is a distinct URL from `?`, so it would be a
 * distinct React Query cache entry for identical data.
 */
export const buildQueryString = (query?: Query): string => {
  if (!query) return '';
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null) continue;
    if (Array.isArray(value)) {
      for (const item of value) params.append(key, String(item));
    } else {
      params.append(key, String(value as string | number | boolean));
    }
  }
  const serialized = params.toString();
  return serialized ? `?${serialized}` : '';
};


/* -------------------------------------------------------------------------- */
/* The axios instance                                                          */
/* -------------------------------------------------------------------------- */

export const createHttpClient = (
  config: AxiosRequestConfig = {}
): AxiosInstance => {
  const instance = axios.create({
    baseURL: platformApiBaseUrl(),
    timeout: DEFAULT_TIMEOUT_MS,
    // Same-origin: the BFF session cookie rides along automatically and the BFF
    // injects the upstream bearer token server-side. The browser never holds a
    // token, so there is nothing here to attach, refresh, or leak.
    withCredentials: false,
    validateStatus: () => true, // every status is handled in `request()`
    ...config,
  });

  instance.interceptors.request.use(attachRequestContext);

  return instance;
};

/**
 * The application-wide client, created on first use rather than at module load.
 *
 * `createHttpClient()` reads `runtimeConfig` to build its `baseURL`, and that
 * config is populated by a script the BFF serves at runtime. Instantiating at
 * import time would freeze whatever happened to be present when the module was
 * first pulled in — which in tests is "nothing", forcing every spec to
 * `vi.resetModules()` and re-import the module under test just to change a URL.
 * Deferring to first call means the instance sees the config that is actually
 * in effect when a request is made.
 */
let clientInstance: AxiosInstance | undefined;

export const getHttpClient = (): AxiosInstance =>
  (clientInstance ??= createHttpClient());

/**
 * Drops the memoized instance so the next request rebuilds it from current
 * config. Intended for tests (`afterEach`); harmless in production, where
 * nothing calls it.
 */
export const resetHttpClient = (): void => {
  clientInstance = undefined;
};

/**
 * Options every generated endpoint function accepts. `signal` is the important
 * one: TanStack Query passes its own signal into every `queryFn`, so wiring it
 * through means superseded and unmounted queries actually abort in flight
 * instead of resolving into a discarded cache entry.
 */
export type RequestOptions = {
  orgId?: string;
  signal?: AbortSignal;
  timeout?: number;
  operationName?: string;
  query?: Query;
  headers?: Record<string, string>;
  body?: unknown;
};

const isFormData = (value: unknown): value is FormData =>
  typeof FormData !== 'undefined' && value instanceof FormData;


/**
 * Turns the raw response body into `T`.
 *
 * `204 No Content` (and an empty body generally) resolves to `undefined` — the
 * spec uses 204 for deletes and undeploys, and `JSON.parse('')` would throw.
 * A success body that is not valid JSON is a contract violation, so it is
 * surfaced as an error rather than passed to a caller expecting an object.
 */
const parseBody = <T>(response: AxiosResponse<unknown>): T => {
  if (response.status === 204 || response.status === 205) {
    return undefined as T;
  }
  const raw = response.data;
  if (raw === undefined || raw === null || raw === '') return undefined as T;
  if (typeof raw !== 'string') return raw as T;
  try {
    return JSON.parse(raw) as T;
  } catch {
    throw platformErrorFromBody(response.status, undefined);
  }
};

/**
 * Classifies an axios rejection. Reaching here means no response arrived —
 * `validateStatus` accepts every status, so a 4xx/5xx never lands in this path.
 */
const classifyTransportFailure = (
  error: unknown,
  deadline: Deadline,
): ApiErrorKind => {
  // Our own composed signal fired: we know precisely which source did it.
  const reason = deadline.reason();
  if (reason) return reason;
  if (axios.isCancel(error)) return 'aborted';
  if (error instanceof AxiosError && error.code === AxiosError.ETIMEDOUT) {
    return 'timeout';
  }
  return 'network';
};


/**
 * Issues one request and resolves it against the spec's contract.
 *
 * Always rejects with a `ApiError`, never an `AxiosError`, never a
 * raw body, so a caller has exactly one error type to handle and the
 * transport stays swappable behind this module.
 */
export async function request<T>(
  method: string,
  path: string,
  options: RequestOptions = {}
) : Promise<T> {
  const verb = method.toUpperCase();
  const deadline = withDeadline(
    options.signal,
    options.timeout ?? DEFAULT_TIMEOUT_MS
  );

  const config: AxiosRequestConfig = {
    method: verb,
    url: `${path}${buildQueryString(options.query)}`,
    headers: options.headers,
    signal: deadline.signal,
    orgId: options.orgId,
    operationName: options.operationName,
    // FormData is passed through untouched; anything else is serialized here
    // so the Content-Type set above is always accurate.
    ...(options.body !== undefined
      ? {
          data: isFormData(options.body)
            ? options.body
            : JSON.stringify(options.body),
        }
      : {}),
  };

  let response: AxiosResponse<unknown>;
  try {
    response = await getHttpClient().request<unknown>(config);
  } catch (error) {
      throw platformErrorFromTransport(
        error,
        classifyTransportFailure(error, deadline),
        config.requestId,
        options.operationName
      )
  } finally {
    deadline.release();
  }

  if (response.status >= 400) {
    let body: unknown = response.data;
    if (typeof body == 'string') {
      try {
        body = JSON.parse(body);
      } catch {
        body = undefined;
      }
    }

    if (response.status === 401) notifySessionExpired();

    throw platformErrorFromBody(response.status, body, config.requestId, options.operationName);
  }

  return parseBody<T>(response);
}

/* -------------------------------------------------------------------------- */
/* Public surface                                                              */
/* -------------------------------------------------------------------------- */

type BodylessOptions = Omit<RequestOptions, 'body'>;

/**
 * The verb-shaped API resource modules use:
 *
 * ```ts
 * http.get<ListResult>('/rest-apis', { org, query: { limit: 20 }, signal });
 * http.post<RestApi>('/rest-apis', body, { org, signal });
 * ```
 */
export const http = {
  get: <T>(path: string, options?: BodylessOptions) =>
    request<T>('GET', path, options),

  post: <T>(path: string, body?: unknown, options?: BodylessOptions) =>
    request<T>('POST', path, { ...options, body }),

  put: <T>(path: string, body?: unknown, options?: BodylessOptions) =>
    request<T>('PUT', path, { ...options, body }),

  patch: <T>(path: string, body?: unknown, options?: BodylessOptions) =>
    request<T>('PATCH', path, { ...options, body }),

  /** DELETE typically answers 204, so `T` defaults to `void`. */
  delete: <T = void>(path: string, options?: BodylessOptions) =>
    request<T>('DELETE', path, options),
} as const;
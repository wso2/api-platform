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
 * The platform-api error envelope, straight from the spec — every failed
 * request across all 62 paths returns this exact shape, so the client only
 * ever needs to parse one thing.
 */

/* -------------------------------------------------------------------------- */
/* Wire shapes — a hand-mirror of the spec's schemas                           */
/* -------------------------------------------------------------------------- */

/** Mirror of `components.schemas.FieldError`. */
export type FieldError = {
  field: string,
  message: string,
}

/**
 * Mirror of `components.schemas.Error`.
 *
 * Declared here rather than imported from `src/api/generated/schema.d.ts`
 * because this module is the layer that decides whether an *unvalidated* body
 * matches the contract — it must be able to describe the expected shape while
 * treating the input as `unknown`
 */
export type ErrorEnvelope = {
  status: "error";
  code: string;
  message: string;
  errors?: FieldError[];
  details?: Record<string, unknown>;
  trackingId?: string;
}

/* -------------------------------------------------------------------------- */
/* Error code catalog                                                          */
/* -------------------------------------------------------------------------- */

/**
 * Every error code the spec names, with where it comes from.
 *
 * This is **not** the whole catalog. The spec defines `code` only as a pattern
 * (`^[A-Z][A-Z0-9_]*$`) "from the error catalog", and the catalog itself is not
 * published — so these are the codes that happen to appear in examples, not an
 * enumeration the backend is bound to. Getting the catalog into the spec as an
 * enum is workstream W1.2 in the plan; until then `PlatformApiErrorCode` stays
 * an open union and an unrecognized code must never be treated as impossible.
 */
export const ErrorCode = {
  // components.responses.* — shared across most operations
  UNAUTHORIZED: 'UNAUTHORIZED',
  FORBIDDEN: 'FORBIDDEN',
  NOT_FOUND: 'NOT_FOUND',
  CONFLICT: 'CONFLICT',
  VALIDATION_FAILED: 'VALIDATION_FAILED',
  INTERNAL_ERROR: 'INTERNAL_ERROR',
  SERVICE_UNAVAILABLE: 'SERVICE_UNAVAILABLE',
  DEPLOYMENT_ACTIVE: 'DEPLOYMENT_ACTIVE',
  GATEWAY_CONNECTION_UNAVAILABLE: 'GATEWAY_CONNECTION_UNAVAILABLE',
  // components.examples.* — referenced by individual operations
  GATEWAY_NOT_FOUND: 'GATEWAY_NOT_FOUND',
  DEPLOYMENT_BASE_NOT_FOUND: 'DEPLOYMENT_BASE_NOT_FOUND',
  // declared inline on a single operation's response
  SECRET_IN_USE: 'SECRET_IN_USE',
  POLICY_INVALID_STATE: 'POLICY_INVALID_STATE',
  // named only in the `code` field's own example
  REST_API_NOT_FOUND: 'REST_API_NOT_FOUND',
} as const;

/**
 * Codes this client synthesizes when the server did not supply one. They are
 * prefixed so they can never collide with a catalog code, and their presence
 * means "the contract was not met", not "the backend reported this".
 */
export const ClientErrorCode = {
  /** The request never produced a response (offline, DNS, TLS, CORS). */
  CLIENT_NETWORK_ERROR: 'CLIENT_NETWORK_ERROR',
  /** The client's own per-request deadline elapsed before a response arrived. */
  CLIENT_TIMEOUT: 'CLIENT_TIMEOUT',
  /** The caller aborted the request (navigation, React Query cancellation). */
  CLIENT_REQUEST_ABORTED: 'CLIENT_REQUEST_ABORTED',
  /** A failure response whose body is absent, non-JSON, or not the Error shape. */
  CLIENT_MALFORMED_ERROR: 'CLIENT_MALFORMED_ERROR',
} as const;

/**
 * Any code from the spec catalog or this client's sentinels, but assignable
 * from any string, because the catalog is open. The union exists for editor
 * completion and typo-catching in `isPlatformApiError(e, ErrorCode.X)` calls,
 * not to constrain what the server may send.
 */
export type PlatformApiErrorCode =
  | (typeof ErrorCode)[keyof typeof ErrorCode]
  | (typeof ClientErrorCode)[keyof typeof ClientErrorCode]
  | (string & {});



/**
 * How the request failed at the *transport* level. This is deliberately
 * separate from `code` (the server's business reason): retry policy, offline
 * banners and cancellation all key off `kind`, while the UI keys off `code`.
 */
export type ApiErrorKind =
  /** Server answered with a non-2xx status. `status` and usually `code` are set. */
  | 'http'
  /** Request never reached the server (DNS, offline, TLS, CORS, BFF down). */
  | 'network'
  /** Client-side deadline elapsed before a response arrived. */
  | 'timeout'
  /** Aborted deliberately — unmounted component, superseded query, user navigation. */
  | 'aborted';

/**
 * Generic fallback text. Never interpolates the response body — a failure
 * body can contain backend detail that should not reach the UI verbatim
 * (see .claude/rules/error-handling.md, directive 1).
 */
const GENERIC_MESSAGE = 'The request could not be completed.';

// /**
//  * Stable, machine-readable codes from the platform-api error catalog
//  * (`<DOMAIN>_<REASON>`, e.g. `REST_API_NOT_FOUND`). The spec types this as a
//  * plain string with a pattern, so we keep it open — narrowing to a union here
//  * would break the client every time the backend adds a code. Branch on these
//  * via `isErrorCode()`, never on HTTP status, per the spec's own guidance.
//  */
// export type ApiErrorCode = string;

/**
 * The single error type the whole app sees. Every transport failure, every
 * non-2xx response, and every parse failure is normalized into one of these by
 * the axios response interceptor, so no component or hook ever handles a raw
 * `AxiosError`.
 */
export class ApiError extends Error {
  /** Transport-level classification — drives retry, not display. */
  readonly kind: ApiErrorKind;

  /** HTTP status, when the server actually answered. */
  readonly status?: number;

  /**
   * Server's stable error code (e.g. `REST_API_NOT_FOUND`). Absent for
   * network/timeout/cancel, and for non-conforming responses (gateway HTML
   * error pages, BFF 502s).
   */
  readonly code?: PlatformApiErrorCode;

  /** Per-field validation failures, when the server returned any. */
  readonly fieldErrors: FieldError[];

  /** Structured, code-specific metadata (e.g. resources blocking a delete). */
  readonly details?: Record<string, unknown>;

  /** Server correlation id — present on 5xx. Surface this in support flows. */
  readonly trackingId?: string;

  /** Client-generated correlation id, echoed in the request headers. */
  readonly requestId?: string;

  /** Method + path, for logging. Never contains bodies or credentials. */
  readonly operation?: string;

  /**
   * The underlying transport failure, kept for debugging. Declared explicitly
   * rather than passed to `super(message, { cause })` so this compiles below an
   * ES2022 lib target.
   */
  readonly cause?: unknown;

  constructor(
    message: string,
    init: {
      kind: ApiErrorKind;
      status?: number;
      code?: PlatformApiErrorCode;
      fieldErrors?: FieldError[];
      details?: Record<string, unknown>;
      trackingId?: string;
      requestId?: string;
      operation?: string;
      cause?: unknown;
    }
  ) {
    super(message);
    this.name = 'ApiError';
    // Assigned rather than passed to `super` so this compiles below an ES2022
    // lib target; the property is what matters for debugging either way.
    this.cause = init.cause;
    this.kind = init.kind;
    this.status = init.status;
    this.code = init.code;
    this.fieldErrors = init.fieldErrors ?? [];
    this.details = init.details;
    this.trackingId = init.trackingId;
    this.requestId = init.requestId;
    this.operation = init.operation;
  }

  /** The session is gone — the BFF could not authenticate the request. */
  get isUnauthenticated(): boolean {
    return this.status === 401;
  }

  /** Authenticated, but the token lacks the `ap:*` scope for this operation. */
  get isForbidden(): boolean {
    return this.status === 403;
  }

  get isNotFound(): boolean {
    return this.status === 404;
  }

  get isConflict(): boolean {
    return this.status === 409;
  }

  /** A 400 the user can fix by editing the form. */
  get isValidation(): boolean {
    return this.status === 400 || this.status === 422;
  }

  /**
   * Whether retrying the identical request could plausibly succeed. This is the
   * single source of truth for the query/mutation retry policy — retrying a 403
   * or a 409 only burns quota and delays the error the user needs to see.
   */
  get isRetryable(): boolean {
    if (this.kind === 'aborted') return false;
    if (this.kind === 'network' || this.kind === 'timeout') return true;
    if (this.status === undefined) return false;
    // 408 Request Timeout and 429 Too Many Requests are explicitly retryable;
    // every other 4xx is a client mistake that will fail identically on retry.
    if (this.status === 408 || this.status === 429) return true;
    return this.status >= 500;
  }

  /**
   * Field errors keyed by field name, for binding onto form inputs.
   * Multiple errors on one field are joined so a single input can show them.
   */
  fieldErrorMap(): Record<string, string> {
    const map: Record<string, string> = {};
    for (const fieldError of this.fieldErrors) {
      const field = fieldError.field;
      if (!field) continue;
      const message = fieldError.message ?? 'Invalid value';
      map[field] = map[field] ? `${map[field]}; ${message}` : message;
    }
    return map;
  }

  /**
   * Structured payload for logging/telemetry. Deliberately excludes the request
   * body and any header — an error log must never become a credential sink.
   */
  toLogContext(): Record<string, unknown> {
    return {
      kind: this.kind,
      status: this.status,
      code: this.code,
      operation: this.operation,
      requestId: this.requestId,
      trackingId: this.trackingId,
      fieldErrorCount: this.fieldErrors.length,
    };
  }
}

/** The spec's own constraint on `code`, applied as the shape test. */
const CODE_PATTERN = /^[A-Z][A-Z0-9_]*$/;

export const isApiError = (error: unknown): error is ApiError =>
  error instanceof ApiError;

/** Branch on the server's stable code — the spec's recommended discriminator. */
export const isErrorCode = (error: unknown, ...codes: PlatformApiErrorCode[]): boolean =>
  isApiError(error) && error.code !== undefined && codes.includes(error.code);

const isRecord = (v: unknown): v is Record<string, unknown> =>
  typeof v === 'object' && v !== null && !Array.isArray(v);

const isNonEmptyString = (v: unknown): v is string =>
  typeof v === 'string' && v.length > 0;

/** Keeps only well-formed `{field, message}` entries from `errors[]`. */
const readFieldErrors = (value: unknown): FieldError[] => {
  if (!Array.isArray(value)) return [];
  return value.flatMap((entry) =>
    isRecord(entry) &&
    isNonEmptyString(entry.field) &&
    typeof entry.message === 'string'
      ? [{ field: entry.field, message: entry.message }]
      : [],
  );
};

/**
 * Narrows an unknown response body to the spec's `Error` envelope. Anything
 * that fails this check (an HTML 502 from a load balancer, a proxy timeout
 * page) falls back to a status-derived message rather than leaking markup into
 * the UI.
 */
export const isErrorEnvelope = (body: unknown): body is ErrorEnvelope => {
  if (!body || typeof body !== 'object') return false;
  const candidate = body as Record<string, unknown>;
  return (
    typeof candidate.code === 'string' && CODE_PATTERN.test(candidate.code) && typeof candidate.message === 'string'
  );
};

/**
 * Builds a `PlatformApiError` from a failure status and its already-parsed
 * body. This is the transport-agnostic entry point: `fetch` callers reach it
 * through `platformErrorFromResponse`, and transports that parse the body
 * themselves (axios hands back `error.response.data`) call it directly.
 *
 * `body` is deliberately `unknown` — this function is the boundary where an
 * unvalidated payload becomes a typed error, so validating it is the job, not
 * a precondition.
 */
export function platformErrorFromBody(
  httpStatus: number,
  body: unknown,
  requestId?: string,
  operationName?: string,
): ApiError {
  if (!isErrorEnvelope(body)) {
    // A failure the contract does not cover: an HTML error page from an
    // intermediary, an empty body, a proxy timeout. `httpStatus` is still
    // meaningful, so keep it; the code says the body was unusable.
    return new ApiError(GENERIC_MESSAGE, {
      kind: 'http',
      status: httpStatus,
      code: ClientErrorCode.CLIENT_MALFORMED_ERROR,
      requestId,
      operation: operationName,
    });
  }

  return new ApiError(isNonEmptyString(body.message) ? body.message : GENERIC_MESSAGE, {
    kind: 'http',
    status: httpStatus,
    code: body.code,
    fieldErrors: readFieldErrors(body.errors),
    details: isRecord(body.details) ? body.details : undefined,
    trackingId: isNonEmptyString(body.trackingId) ? body.trackingId : undefined,
    requestId,
    operation: operationName,
  });
}


const TRANSPORT_FAILURES: Record<
  ApiErrorKind,
  { code: PlatformApiErrorCode; message: string }
> = {
  network: {
    code: ClientErrorCode.CLIENT_NETWORK_ERROR,
    message: GENERIC_MESSAGE,
  },
  timeout: {
    code: ClientErrorCode.CLIENT_TIMEOUT,
    message: 'The request timed out.',
  },
  aborted: {
    code: ClientErrorCode.CLIENT_REQUEST_ABORTED,
    message: 'The request was cancelled.',
  },
  http: {
    code: ClientErrorCode.CLIENT_MALFORMED_ERROR,
    message: 'The server returned an unexpected response.',
  }
};

/**
 * Wraps a thrown transport failure (no response was received) so callers only
 * ever catch one error type.
 *
 * `kind` is passed explicitly by transports that can classify precisely —
 * axios distinguishes a cancellation from a timeout via its own error codes,
 * and that knowledge belongs in the transport, not here. When omitted, the
 * only distinction inferable from a bare thrown value is `AbortError`.
 */
export function platformErrorFromTransport(
  cause: unknown,
  kind?: ApiErrorKind,
  requestId?: string,
  operationName?: string,
): ApiError {
  // Checked by `name` rather than by prototype: an abort surfaces as a
  // `DOMException` in browsers and under jsdom, and `DOMException` does not
  // extend `Error` everywhere, so `instanceof Error` misses it.
  const abortLike =
    isRecord(cause) && (cause as { name?: unknown }).name === 'AbortError';
  const inferred: ApiErrorKind = kind ?? (abortLike ? 'aborted' : 'network');

  const { code, message } = TRANSPORT_FAILURES[inferred];

  return new ApiError(message, {
    kind: inferred,
    status: 0,
    code,
    cause,
    requestId,
    operation: operationName  ,
  }
  )
}
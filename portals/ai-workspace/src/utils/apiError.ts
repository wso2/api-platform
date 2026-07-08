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

// ============================================================================
// Platform API Error Handling
// ----------------------------------------------------------------------------
// The Platform API returns a single standard error shape on every failed
// request (see platform-api/resources/openapi.yaml "Error" schema):
//
//   { status: "error", code: "REST_API_NOT_FOUND", message: "...",
//     errors?: FieldError[], details?: object, trackingId?: uuid }
//
// This module parses that shape into an `ApiError`, and provides helpers so
// UI code never has to guess at field names or dump a raw backend message.
// ============================================================================

/** A single field-level validation failure. */
export interface FieldError {
  field: string;
  message: string;
}

/** Raw error body shape returned by the Platform API on failed requests. */
export interface PlatformErrorBody {
  status: 'error';
  code: string;
  message: string;
  errors?: FieldError[];
  details?: Record<string, unknown>;
  trackingId?: string;
}

/** Error thrown by API clients on non-2xx responses. */
export interface ApiError extends Error {
  status?: number;
  /** Machine-readable code, e.g. "REST_API_NOT_FOUND". Prefer this over `status` for branching. */
  code?: string;
  /** Per-field validation failures, present on validation failures. */
  errors?: FieldError[];
  /** Structured metadata specific to this error condition; shape varies by `code`. */
  details?: Record<string, unknown>;
  /** Correlation ID for server-side (5xx) failures. */
  trackingId?: string;
  /** Raw parsed response body, kept for callers that need the full payload. */
  data?: unknown;
}

/** UI messages are capped so a huge backend error string never floods a banner or toast. */
const MAX_UI_MESSAGE_LENGTH = 160;

/** Truncate a message to a UI-safe length, preferring a sentence boundary when there is one. */
export function shortenMessage(message: string, maxLength = MAX_UI_MESSAGE_LENGTH): string {
  const trimmed = (message ?? '').trim();
  if (trimmed.length <= maxLength) return trimmed;

  const truncated = trimmed.slice(0, maxLength);
  const lastStop = Math.max(truncated.lastIndexOf('. '), truncated.lastIndexOf('.\n'));
  const cut = lastStop > maxLength * 0.4 ? truncated.slice(0, lastStop + 1) : truncated;
  return `${cut.replace(/[.,;:\s]+$/, '')}…`;
}

function asPlatformErrorBody(body: unknown): Partial<PlatformErrorBody> {
  return body && typeof body === 'object' ? (body as Partial<PlatformErrorBody>) : {};
}

/**
 * Build an `ApiError` from an HTTP status and the response's parsed JSON body
 * (or `undefined` if the body wasn't valid JSON / wasn't the standard shape).
 * The resulting `error.message` is already shortened and safe to render as-is.
 */
export function buildApiError(
  status: number,
  body: unknown,
  fallbackMessage = `Request failed (HTTP ${status})`,
): ApiError {
  const parsed = asPlatformErrorBody(body);
  const rawMessage = typeof parsed.message === 'string' && parsed.message.trim().length > 0
    ? parsed.message
    : fallbackMessage;

  const err = new Error(shortenMessage(rawMessage)) as ApiError;
  err.status = status;
  err.code = typeof parsed.code === 'string' ? parsed.code : undefined;
  err.errors = Array.isArray(parsed.errors) ? parsed.errors : undefined;
  err.details = parsed.details && typeof parsed.details === 'object' ? parsed.details : undefined;
  err.trackingId = typeof parsed.trackingId === 'string' ? parsed.trackingId : undefined;
  err.data = body;
  return err;
}

/** Short, UI-safe message for any error (Platform API error or otherwise). Never returns a huge string. */
export function getErrorMessage(error: unknown, fallback = 'Something went wrong.'): string {
  const message = (error as Error | null | undefined)?.message;
  return message ? shortenMessage(message) : fallback;
}

/** Machine-readable error code (e.g. "REST_API_NOT_FOUND"). Prefer this over HTTP status for branching. */
export function getErrorCode(error: unknown): string | undefined {
  return (error as ApiError | null | undefined)?.code;
}

/** Per-field validation errors, if this was a validation failure. */
export function getFieldErrors(error: unknown): FieldError[] | undefined {
  return (error as ApiError | null | undefined)?.errors;
}

/** Correlation ID for 5xx failures, for the user to quote when reporting the error. */
export function getTrackingId(error: unknown): string | undefined {
  return (error as ApiError | null | undefined)?.trackingId;
}

/** HTTP status code, if this error originated from an HTTP response. */
export function getHttpStatus(error: unknown): number | undefined {
  return (error as ApiError | null | undefined)?.status;
}

/** Map field-level validation errors onto a `{ [fieldName]: message }` map for form state. */
export function fieldErrorsToMap(errors?: FieldError[] | null): Record<string, string> {
  if (!errors) return {};
  return errors.reduce<Record<string, string>>((acc, e) => {
    if (e?.field) acc[e.field] = e.message;
    return acc;
  }, {});
}

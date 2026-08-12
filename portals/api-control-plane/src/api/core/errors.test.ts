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

import { describe, expect, it } from 'vitest';

import {
  ApiError,
  ClientErrorCode,
  ErrorCode,
  isApiError,
  isErrorCode,
  isErrorEnvelope,
  platformErrorFromBody,
  platformErrorFromTransport,
  type ApiErrorKind,
} from './errors';

/**
 * `ApiError` is the only error type the rest of the app ever sees, so this file
 * is the contract test for that promise. Three things matter:
 *
 *   1. A well-formed error envelope survives intact — code, field errors and
 *      tracking id are what the UI branches on.
 *   2. A malformed body degrades safely and never leaks its contents.
 *   3. `isRetryable` is exactly right, because the whole retry policy reads it.
 */

/** A complete, spec-shaped error body as platform-api would return it. */
const validationEnvelope = {
  status: 'error',
  code: 'VALIDATION_FAILED',
  message: 'The request could not be validated.',
  errors: [
    { field: 'displayName', message: 'must not be blank' },
    { field: 'version', message: 'must match semver' },
  ],
  details: { rejectedFields: 2 },
  trackingId: '4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11',
};

describe('isErrorEnvelope', () => {
  it('accepts a body carrying the spec-required code and message', () => {
    expect(isErrorEnvelope({ code: 'REST_API_NOT_FOUND', message: 'Gone.' })).toBe(
      true
    );
  });

  it.each([
    ['an HTML error page from a proxy', '<html><body>502</body></html>'],
    ['an empty body', undefined],
    ['null', null],
    ['a body with no code', { message: 'something failed' }],
    ['a body with no message', { code: 'REST_API_NOT_FOUND' }],
  ])('rejects %s', (_label, body) => {
    expect(isErrorEnvelope(body)).toBe(false);
  });

  it('rejects a code that does not match the spec pattern, so a stray string is not mistaken for a catalog code', () => {
    // The spec constrains `code` to /^[A-Z][A-Z0-9_]*$/.
    expect(isErrorEnvelope({ code: 'not_a_code', message: 'x' })).toBe(false);
    expect(isErrorEnvelope({ code: '404', message: 'x' })).toBe(false);
  });
});

describe('platformErrorFromBody — a well-formed envelope', () => {
  const error = platformErrorFromBody(400, validationEnvelope, 'req-1', 'CreateRESTAPI');

  it('keeps the server message, because it is written for the user', () => {
    expect(error.message).toBe('The request could not be validated.');
  });

  it('keeps the stable code the UI is meant to branch on', () => {
    expect(error.code).toBe(ErrorCode.VALIDATION_FAILED);
  });

  it('keeps the HTTP status', () => {
    expect(error.status).toBe(400);
    expect(error.kind).toBe<ApiErrorKind>('http');
  });

  it('keeps per-field errors so a form can bind them to inputs', () => {
    expect(error.fieldErrors).toEqual(validationEnvelope.errors);
  });

  it('keeps structured details and the server tracking id', () => {
    expect(error.details).toEqual({ rejectedFields: 2 });
    expect(error.trackingId).toBe(validationEnvelope.trackingId);
  });

  it('records the correlation id and operation passed by the transport', () => {
    expect(error.requestId).toBe('req-1');
    expect(error.operation).toBe('CreateRESTAPI');
  });
});

describe('platformErrorFromBody — a body that breaks the contract', () => {
  it('falls back to a generic message rather than surfacing the raw body', () => {
    // A load balancer returning an HTML page must not put markup on screen.
    const error = platformErrorFromBody(502, '<html>Bad Gateway</html>');

    expect(error.message).not.toContain('<html>');
    expect(error.message).toBe('The request could not be completed.');
  });

  it('marks the failure as malformed so it is distinguishable from a catalog code', () => {
    const error = platformErrorFromBody(502, '<html>Bad Gateway</html>');

    expect(error.code).toBe(ClientErrorCode.CLIENT_MALFORMED_ERROR);
  });

  it('still keeps the status, which remains meaningful', () => {
    expect(platformErrorFromBody(502, undefined).status).toBe(502);
  });

  it('drops field entries that are not shaped {field, message}', () => {
    const error = platformErrorFromBody(400, {
      code: 'VALIDATION_FAILED',
      message: 'Invalid.',
      errors: [
        { field: 'name', message: 'required' },
        { field: '', message: 'no field name' },
        'not an object',
        { message: 'missing field key' },
      ],
    });

    expect(error.fieldErrors).toEqual([{ field: 'name', message: 'required' }]);
  });
});

describe('platformErrorFromTransport', () => {
  it('classifies an AbortError as a deliberate cancellation when no kind is given', () => {
    // jsdom and browsers both surface aborts as a DOMException named AbortError.
    const abort = new DOMException('The operation was aborted.', 'AbortError');

    expect(platformErrorFromTransport(abort).kind).toBe<ApiErrorKind>('aborted');
  });

  it('assumes a network failure for any other bare thrown value', () => {
    expect(platformErrorFromTransport(new Error('socket hang up')).kind).toBe<ApiErrorKind>(
      'network'
    );
  });

  it('prefers the kind the transport supplies, which can tell timeout from cancel', () => {
    const abort = new DOMException('aborted', 'AbortError');

    expect(platformErrorFromTransport(abort, 'timeout').kind).toBe<ApiErrorKind>('timeout');
  });

  it.each([
    ['network', ClientErrorCode.CLIENT_NETWORK_ERROR],
    ['timeout', ClientErrorCode.CLIENT_TIMEOUT],
    ['aborted', ClientErrorCode.CLIENT_REQUEST_ABORTED],
  ] as const)('tags a %s failure with its own client code', (kind, expected) => {
    expect(platformErrorFromTransport(new Error('x'), kind).code).toBe(expected);
  });

  it('keeps the original failure as `cause` for debugging', () => {
    const cause = new Error('socket hang up');

    expect(platformErrorFromTransport(cause).cause).toBe(cause);
  });

  it('reports status 0, since no response was received', () => {
    expect(platformErrorFromTransport(new Error('x')).status).toBe(0);
  });
});

describe('ApiError.isRetryable — the single input to the retry policy', () => {
  const httpError = (status: number) =>
    new ApiError('failed', { kind: 'http', status });

  it.each([
    ['500 Internal Server Error', 500],
    ['502 Bad Gateway', 502],
    ['503 Service Unavailable', 503],
    ['408 Request Timeout', 408],
    ['429 Too Many Requests', 429],
  ])('retries %s, because a second attempt can plausibly succeed', (_label, status) => {
    expect(httpError(status).isRetryable).toBe(true);
  });

  it.each([
    ['400 Bad Request', 400],
    ['401 Unauthorized', 401],
    ['403 Forbidden', 403],
    ['404 Not Found', 404],
    ['409 Conflict', 409],
    ['422 Unprocessable Entity', 422],
  ])(
    'does not retry %s, because it fails identically and only delays the message',
    (_label, status) => {
      expect(httpError(status).isRetryable).toBe(false);
    }
  );

  it('retries a network failure', () => {
    expect(new ApiError('offline', { kind: 'network' }).isRetryable).toBe(true);
  });

  it('retries a timeout', () => {
    expect(new ApiError('slow', { kind: 'timeout' }).isRetryable).toBe(true);
  });

  it('never retries a deliberate abort — the caller asked for it to stop', () => {
    expect(new ApiError('cancelled', { kind: 'aborted' }).isRetryable).toBe(false);
  });
});

describe('ApiError status predicates', () => {
  it.each([
    ['isUnauthenticated', 401],
    ['isForbidden', 403],
    ['isNotFound', 404],
    ['isConflict', 409],
  ] as const)('%s is true only for %i', (predicate, status) => {
    expect(new ApiError('x', { kind: 'http', status })[predicate]).toBe(true);
    expect(new ApiError('x', { kind: 'http', status: 500 })[predicate]).toBe(false);
  });

  it('treats both 400 and 422 as validation failures the user can fix', () => {
    expect(new ApiError('x', { kind: 'http', status: 400 }).isValidation).toBe(true);
    expect(new ApiError('x', { kind: 'http', status: 422 }).isValidation).toBe(true);
  });
});

describe('ApiError.fieldErrorMap — binding server errors onto form inputs', () => {
  it('keys each message by its field', () => {
    const error = platformErrorFromBody(400, validationEnvelope);

    expect(error.fieldErrorMap()).toEqual({
      displayName: 'must not be blank',
      version: 'must match semver',
    });
  });

  it('joins multiple messages for one field so a single input can show them all', () => {
    const error = platformErrorFromBody(400, {
      code: 'VALIDATION_FAILED',
      message: 'Invalid.',
      errors: [
        { field: 'version', message: 'must match semver' },
        { field: 'version', message: 'must be greater than the current version' },
      ],
    });

    expect(error.fieldErrorMap().version).toBe(
      'must match semver; must be greater than the current version'
    );
  });

  it('is empty when the failure was not a validation failure', () => {
    expect(platformErrorFromTransport(new Error('x')).fieldErrorMap()).toEqual({});
  });
});

describe('ApiError.toLogContext — what may be sent to telemetry', () => {
  const error = platformErrorFromBody(500, {
    code: 'INTERNAL_ERROR',
    message: 'Something went wrong.',
    trackingId: 'trk-1',
  }, 'req-9', 'GetRESTAPI');

  it('carries the identifiers needed to correlate a report with a server log', () => {
    expect(error.toLogContext()).toMatchObject({
      code: 'INTERNAL_ERROR',
      kind: 'http',
      operation: 'GetRESTAPI',
      requestId: 'req-9',
      status: 500,
      trackingId: 'trk-1',
    });
  });

  it('omits the request body and headers, so the error channel cannot become a credential sink', () => {
    const context = error.toLogContext();

    expect(context).not.toHaveProperty('body');
    expect(context).not.toHaveProperty('headers');
    expect(context).not.toHaveProperty('cause');
  });

  it('reports only the count of field errors, not their contents', () => {
    const validation = platformErrorFromBody(400, validationEnvelope);

    expect(validation.toLogContext().fieldErrorCount).toBe(2);
    expect(validation.toLogContext()).not.toHaveProperty('fieldErrors');
  });
});

describe('type guards used by callers', () => {
  it('isApiError distinguishes our error from any other throwable', () => {
    expect(isApiError(platformErrorFromTransport(new Error('x')))).toBe(true);
    expect(isApiError(new Error('plain'))).toBe(false);
    expect(isApiError('a string')).toBe(false);
  });

  it('isErrorCode matches any of the codes it is given', () => {
    const error = platformErrorFromBody(404, {
      code: 'REST_API_NOT_FOUND',
      message: 'Gone.',
    });

    expect(isErrorCode(error, ErrorCode.REST_API_NOT_FOUND)).toBe(true);
    expect(isErrorCode(error, ErrorCode.CONFLICT, ErrorCode.REST_API_NOT_FOUND)).toBe(
      true
    );
    expect(isErrorCode(error, ErrorCode.CONFLICT)).toBe(false);
  });

  it('isErrorCode is false for a non-ApiError, so callers need no extra guard', () => {
    expect(isErrorCode(new Error('plain'), ErrorCode.CONFLICT)).toBe(false);
  });
});

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

import { ApiError, type FieldError } from '@/api/core/errors';
import { toCreateApiFormErrors } from './serverFieldErrors';

const rejection = (status: number, fieldErrors: FieldError[] = [], message = 'Rejected.') =>
  new ApiError(message, { fieldErrors, kind: 'http', status });

describe('toCreateApiFormErrors', () => {
  it('pins each field error to the input that produced it', () => {
    const result = toCreateApiFormErrors(
      rejection(400, [
        { field: 'displayName', message: 'Name contains an unsupported character.' },
        { field: 'upstream.main.url', message: 'Must be an http or https URL.' },
      ]),
    );

    expect(result?.fields).toEqual({
      displayName: 'Name contains an unsupported character.',
      targetUrl: 'Must be an http or https URL.',
    });
    expect(result?.unmapped).toEqual([]);
  });

  it('reads the aliases the server uses for the same input', () => {
    // `field` is prose in the spec, not an enum — the identifier has been
    // named both `id` and `handle`, and the base path both ways too.
    expect(toCreateApiFormErrors(rejection(400, [{ field: 'handle', message: 'Taken.' }]))?.fields)
      .toEqual({ id: 'Taken.' });
    expect(
      toCreateApiFormErrors(rejection(400, [{ field: 'basePath', message: 'In use.' }]))?.fields,
    ).toEqual({ context: 'In use.' });
  });

  it('ignores case and array indices when matching a field path', () => {
    const result = toCreateApiFormErrors(
      rejection(400, [{ field: 'Upstream.Main[0].Url', message: 'Unreachable.' }]),
    );

    expect(result?.fields).toEqual({ targetUrl: 'Unreachable.' });
  });

  it('keeps a field error the form has no input for, rather than dropping it', () => {
    // Silently swallowing it would leave the user staring at a form with
    // nothing marked and no idea what the server objected to.
    const result = toCreateApiFormErrors(
      rejection(400, [{ field: 'projectId', message: 'Unknown project.' }]),
    );

    expect(result?.fields).toEqual({});
    expect(result?.unmapped).toEqual(['Unknown project.']);
  });

  it('treats a conflict with no field detail as the form’s to fix', () => {
    // The most common real rejection: a handle or base path already in use,
    // described in prose with no `errors[]` to pin it to a field.
    const result = toCreateApiFormErrors(
      rejection(409, [], 'An API with context /orders and version 1.0.0 already exists.'),
    );

    expect(result).not.toBeNull();
    expect(result?.fields).toEqual({});
    expect(result?.message).toBe(
      'An API with context /orders and version 1.0.0 already exists.',
    );
  });

  it('declines a failure that editing the form cannot fix', () => {
    // These belong to the progress screen: retyping a name does not fix a
    // 500, a dropped connection, or a missing scope.
    expect(toCreateApiFormErrors(rejection(500))).toBeNull();
    expect(toCreateApiFormErrors(rejection(403))).toBeNull();
    expect(
      toCreateApiFormErrors(new ApiError('Offline.', { kind: 'network', status: 0 })),
    ).toBeNull();
  });
});

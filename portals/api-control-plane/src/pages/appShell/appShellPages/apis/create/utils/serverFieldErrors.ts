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

import { type ApiError } from '@/api/core/errors';

/** The creation form's inputs that a server error can be pinned to. */
export type CreateApiFormField = 'context' | 'displayName' | 'id' | 'targetUrl' | 'version';

/** A rejected create, expressed in the form's own terms. */
export type CreateApiFormErrors = {
  /** Server messages pinned to the input that caused them. */
  fields: Partial<Record<CreateApiFormField, string>>;
  /** The server's own sentence about the request as a whole. */
  message?: string;
  /**
   * Field errors naming something this form has no input for — a bad
   * `projectId`, an operation the imported contract carried. They have nowhere
   * to point, so they are listed in the summary rather than dropped: an error
   * the user cannot see is worse than one they cannot click.
   */
  unmapped: string[];
};

/**
 * Request-body paths, as the server names them, mapped onto form fields.
 *
 * Keys are normalized (lower-cased, array indices stripped) before lookup, and
 * the aliases exist because the field name is prose in the spec, not an enum —
 * the same input has been called `id` and `handle` in different messages.
 */
const FIELD_BY_SERVER_PATH: Record<string, CreateApiFormField> = {
  apiid: 'id',
  basepath: 'context',
  context: 'context',
  displayname: 'displayName',
  handle: 'id',
  id: 'id',
  upstream: 'targetUrl',
  'upstream.main': 'targetUrl',
  'upstream.main.url': 'targetUrl',
  version: 'version',
};

/** Lower-cased, with array indices dropped: `errors[0].field` names one input. */
const normalizePath = (field: string): string =>
  field
    .trim()
    .toLowerCase()
    .replace(/\[\d+\]/g, '');

/**
 * Re-expresses a failed create as something the form can show, or `null` when
 * the form is the wrong place for it.
 *
 * Only a rejection the user can act on by editing gets a form: a validation
 * failure or a conflict (a handle or base path already in use — by far the
 * most common real failure here). A 5xx, a dropped connection or a permission
 * problem is not fixed by retyping, so it stays on the progress screen instead
 * of dumping the user back into a form with nothing to change.
 */
export const toCreateApiFormErrors = (error: ApiError): CreateApiFormErrors | null => {
  const hasFieldDetail = error.fieldErrors.length > 0;
  if (!hasFieldDetail && !error.isValidation && !error.isConflict) return null;

  const fields: CreateApiFormErrors['fields'] = {};
  const unmapped: string[] = [];

  for (const [path, message] of Object.entries(error.fieldErrorMap())) {
    const field = FIELD_BY_SERVER_PATH[normalizePath(path)];
    if (field === undefined) {
      unmapped.push(message);
      continue;
    }
    // First wins: two messages for one input would overwrite each other, and
    // `fieldErrorMap` has already joined the ones the server sent together.
    fields[field] ??= message;
  }

  return { fields, message: error.message, unmapped };
};

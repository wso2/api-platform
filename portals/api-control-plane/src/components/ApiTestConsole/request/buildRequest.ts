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

import type {
  ConsoleAutoParam,
  ConsoleBody,
  ConsoleBodyOption,
  ConsoleBodyPart,
  ConsoleOperation,
  ConsoleParameter,
  ConsoleSendRequest,
} from '../types';
import { encodePathValue, headerProblem, isSafePath } from './safety';

/**
 * Turns what the user filled in for one operation into a request — or into
 * field-level problems, never both.
 *
 * Validation is explicit and local: required values, number and enum
 * conformance, body well-formedness, and header safety. Problems are returned
 * as codes (not sentences) so the view can word them in the user's language.
 */

export type DraftHeader = { id: string; name: string; value: string };

export type RequestDraft = {
  /** Values by `in:name`. */
  params: Record<string, string>;
  /**
   * The path (and optional `?query`) the user typed into the URL bar. When set, it is sent
   * instead of the operation's path with its `{placeholders}` filled in.
   */
  pathOverride?: string;
  headers: DraftHeader[];
  bodyMediaType: string;
  bodyText: string;
  /** The named example the body was last set from, so Reset returns to it after edits. */
  bodyExample?: string;
  /** Text fields of a form body, by field name. */
  formValues: Record<string, string>;
  /** Chosen files, by field name, or {@link BINARY_FILE_KEY} for a binary body. */
  files: Record<string, File | undefined>;
};

export const BINARY_FILE_KEY = '__binary__';

export type ProblemCode =
  | 'required'
  | 'integer'
  | 'number'
  | 'boolean'
  | 'enum'
  | 'min'
  | 'max'
  | 'json'
  | 'xml'
  | 'headerName'
  | 'headerForbidden'
  | 'headerValue'
  | 'path'
  | 'file';

export type Problem = { code: ProblemCode; min?: number; max?: number };

export const paramKey = (parameter: Pick<ConsoleParameter, 'in' | 'name'>): string =>
  `${parameter.in}:${parameter.name}`;

export const paramProblemKey = (parameter: Pick<ConsoleParameter, 'in' | 'name'>): string =>
  `param:${paramKey(parameter)}`;

export const headerProblemKey = (id: string): string => `header:${id}`;
export const fieldProblemKey = (name: string): string => `field:${name}`;
export const BODY_PROBLEM_KEY = 'body';
export const PATH_PROBLEM_KEY = 'path';

export type BuildResult = {
  request?: ConsoleSendRequest;
  /** The target as far as it is filled in, for the URL bar. Valid parts only. */
  preview: { path: string; query: [string, string][] };
  problems: Record<string, Problem>;
  /** Lower-cased header names and exact query names whose values are credentials. */
  secretHeaders: ReadonlySet<string>;
  secretQuery: ReadonlySet<string>;
};

/** Own-property read, so a field named `constructor` or `__proto__` cannot reach the prototype. */
const own = <T>(record: Record<string, T>, key: string): T | undefined =>
  Object.prototype.hasOwnProperty.call(record, key) ? record[key] : undefined;

/** Joiner for a non-exploded array query value, by OpenAPI `style`. */
const ARRAY_DELIMITERS: Record<string, string> = {
  form: ',',
  pipeDelimited: '|',
  spaceDelimited: ' ',
};

/** Validates one parameter value; `undefined` when it is acceptable. */
const parameterProblem = (parameter: ConsoleParameter, value: string): Problem | undefined => {
  if (value.trim() === '') return parameter.required ? { code: 'required' } : undefined;

  if (parameter.type === 'integer' && !/^-?\d+$/.test(value)) return { code: 'integer' };
  if (parameter.type === 'number' && !(value === value.trim() && Number.isFinite(Number(value)))) {
    return { code: 'number' };
  }
  if (parameter.type === 'boolean' && value !== 'true' && value !== 'false') {
    return { code: 'boolean' };
  }
  if (parameter.enum && parameter.type !== 'array' && !parameter.enum.includes(value)) {
    return { code: 'enum' };
  }
  if (parameter.type === 'integer' || parameter.type === 'number') {
    const n = Number(value);
    if (parameter.minimum !== undefined && n < parameter.minimum) {
      return { code: 'min', min: parameter.minimum, max: parameter.maximum };
    }
    if (parameter.maximum !== undefined && n > parameter.maximum) {
      return { code: 'max', min: parameter.minimum, max: parameter.maximum };
    }
  }
  return undefined;
};

/** Whether text is well-formed XML. `DOMParser` never loads external entities or DTDs. */
const isWellFormedXml = (text: string): boolean => {
  if (typeof DOMParser === 'undefined') return true;
  const document = new DOMParser().parseFromString(text, 'application/xml');
  return document.getElementsByTagName('parsererror').length === 0;
};

const bodyProblem = (option: ConsoleBodyOption, text: string): Problem | undefined => {
  if (option.kind === 'json') {
    try {
      JSON.parse(text);
    } catch {
      return { code: 'json' };
    }
  }
  if (option.kind === 'xml' && !isWellFormedXml(text)) return { code: 'xml' };
  return undefined;
};

type BuildArgs = {
  operation: ConsoleOperation;
  draft: RequestDraft;
  baseUrl: string;
  autoHeaders: readonly ConsoleAutoParam[];
  autoQuery: readonly ConsoleAutoParam[];
};

export const buildSendRequest = ({
  operation,
  draft,
  baseUrl,
  autoHeaders,
  autoQuery,
}: BuildArgs): BuildResult => {
  const problems: Record<string, Problem> = {};
  const query: [string, string][] = [];
  const headers: [string, string][] = [];
  // A wildcard route (`/*`, `/files/*`) names every path below it; the request goes to the
  // route's base, so the `*` is dropped rather than sent literally.
  let path = operation.path.replace(/\/\*$/, '/');

  // Console-owned credentials replace anything the user typed under the same name.
  const autoHeaderNames = new Set(autoHeaders.map((h) => h.name.toLowerCase()));
  const autoQueryNames = new Set(autoQuery.map((q) => q.name));

  // A path typed into the URL bar replaces the operation's own; its query joins the others.
  // Names the user filled in on the Params tab win over the typed ones, and the console's own
  // credentials win over both, so no name is sent twice.
  const editorQueryNames = new Set(
    operation.parameters
      .filter(
        (parameter) =>
          parameter.in === 'query' && (own(draft.params, paramKey(parameter)) ?? '').trim() !== '',
      )
      .map((parameter) => parameter.name),
  );
  const typedQueryNames = new Set<string>();
  const override = draft.pathOverride;
  if (override !== undefined) {
    const split = override.indexOf('?');
    path = split === -1 ? override : override.slice(0, split);
    const typedQuery = split === -1 ? '' : override.slice(split + 1);
    // eslint-disable-next-line no-control-regex
    if (/[#\u0000-\u001f\u007f]/.test(typedQuery)) problems[PATH_PROBLEM_KEY] = { code: 'path' };
    else {
      new URLSearchParams(typedQuery).forEach((value, name) => {
        if (autoQueryNames.has(name) || editorQueryNames.has(name)) return;
        query.push([name, value]);
        if (value.trim() !== '') typedQueryNames.add(name);
      });
    }
  }

  if (!isSafePath(path)) problems[PATH_PROBLEM_KEY] = { code: 'path' };

  operation.parameters.forEach((parameter) => {
    // The console supplies these itself, so what the user typed (or left blank) is moot.
    if (parameter.in === 'query' && autoQueryNames.has(parameter.name)) return;
    if (parameter.in === 'header' && autoHeaderNames.has(parameter.name.toLowerCase())) return;
    // Content-Type is owned by the body editor's media-type choice.
    if (parameter.in === 'header' && parameter.name.toLowerCase() === 'content-type') return;

    // The typed path already carries whatever the placeholders stood for.
    if (parameter.in === 'path' && override !== undefined) return;

    // Blank is judged on the trimmed text; typed values are validated exactly as they are sent.
    const value = own(draft.params, paramKey(parameter)) ?? '';
    // A required query value the typed path already supplies is not missing.
    if (parameter.in === 'query' && value.trim() === '' && typedQueryNames.has(parameter.name)) {
      return;
    }
    const problem = parameterProblem(parameter, value);
    if (problem) {
      problems[paramProblemKey(parameter)] = problem;
      return;
    }
    if (value.trim() === '') return;

    if (parameter.in === 'path') {
      path = path.split(`{${parameter.name}}`).join(encodePathValue(value));
    } else if (parameter.in === 'query') {
      if (parameter.type === 'array') {
        const items = value
          .split(',')
          .map((v) => v.trim())
          .filter((v) => v !== '');
        // `explode` defaults to true for `form`, which is a repeated parameter.
        const delimiter = ARRAY_DELIMITERS[parameter.style ?? 'form'];
        if (parameter.explode === false && delimiter !== undefined) {
          if (items.length > 0) query.push([parameter.name, items.join(delimiter)]);
        } else {
          items.forEach((v) => query.push([parameter.name, v]));
        }
      } else {
        query.push([parameter.name, value]);
      }
    } else {
      headers.push([parameter.name, value]);
    }
  });

  draft.headers
    .filter((header) => header.name.trim() !== '')
    .forEach((header) => {
      const name = header.name.trim();
      const problem = headerProblem(name, header.value);
      if (problem) {
        problems[headerProblemKey(header.id)] = {
          code:
            problem === 'name'
              ? 'headerName'
              : problem === 'forbidden'
                ? 'headerForbidden'
                : 'headerValue',
        };
        return;
      }
      // Content-Type is owned by the body editor's media-type choice.
      if (name.toLowerCase() === 'content-type') return;
      if (autoHeaderNames.has(name.toLowerCase())) return;
      headers.push([name, header.value]);
    });

  autoHeaders.forEach((header) => headers.push([header.name, header.value]));
  autoQuery.forEach((param) => query.push([param.name, param.value]));

  // Body. A GET or HEAD cannot carry one, so none is read or sent.
  let body: ConsoleBody | undefined;
  const canHaveBody = operation.method !== 'GET' && operation.method !== 'HEAD';
  const option = operation.body?.options.find((o) => o.mediaType === draft.bodyMediaType);
  if (canHaveBody && operation.body && !option && operation.body.required) {
    // A required body with no usable media type must not be dropped silently.
    problems[BODY_PROBLEM_KEY] = { code: 'required' };
  }
  if (canHaveBody && operation.body && option) {
    if (option.kind === 'json' || option.kind === 'xml' || option.kind === 'text') {
      const text = draft.bodyText;
      if (text.trim() === '') {
        if (operation.body.required) problems[BODY_PROBLEM_KEY] = { code: 'required' };
      } else {
        const problem = bodyProblem(option, text);
        if (problem) problems[BODY_PROBLEM_KEY] = problem;
        else body = { kind: 'text', mediaType: option.mediaType, text };
      }
    } else if (option.kind === 'urlencoded') {
      const fields: [string, string][] = [];
      option.formFields.forEach((field) => {
        const value = own(draft.formValues, field.name) ?? '';
        if (value === '') {
          if (field.required) problems[fieldProblemKey(field.name)] = { code: 'required' };
          return;
        }
        fields.push([field.name, value]);
      });
      if (fields.length > 0) body = { kind: 'form', fields };
    } else if (option.kind === 'multipart') {
      const parts: ConsoleBodyPart[] = [];
      option.formFields.forEach((field) => {
        if (field.binary) {
          const file = own(draft.files, field.name);
          if (file) parts.push({ name: field.name, file });
          else if (field.required) problems[fieldProblemKey(field.name)] = { code: 'file' };
          return;
        }
        const value = own(draft.formValues, field.name) ?? '';
        if (value === '') {
          if (field.required) problems[fieldProblemKey(field.name)] = { code: 'required' };
          return;
        }
        parts.push({ name: field.name, value });
      });
      if (parts.length > 0) body = { kind: 'multipart', parts };
    } else {
      const file = own(draft.files, BINARY_FILE_KEY);
      if (file) body = { kind: 'binary', mediaType: option.mediaType, file };
      else if (operation.body.required) problems[BODY_PROBLEM_KEY] = { code: 'file' };
    }
  }

  // Body presence is independent of field-level requirements: a required body whose form
  // fields are all optional and empty still has to carry something.
  if (
    canHaveBody &&
    operation.body?.required &&
    option &&
    body === undefined &&
    !problems[BODY_PROBLEM_KEY]
  ) {
    problems[BODY_PROBLEM_KEY] = { code: 'required' };
  }

  if (body?.kind === 'text') headers.push(['Content-Type', body.mediaType]);
  if (body?.kind === 'form') headers.push(['Content-Type', 'application/x-www-form-urlencoded']);
  if (body?.kind === 'binary') headers.push(['Content-Type', body.mediaType]);
  // Multipart: the boundary belongs to whoever serialises the body, so no Content-Type here.

  const secretHeaders = new Set(
    autoHeaders.filter((h) => h.secret).map((h) => h.name.toLowerCase()),
  );
  const secretQuery = new Set(autoQuery.filter((q) => q.secret).map((q) => q.name));

  const preview = { path, query };
  if (Object.keys(problems).length > 0) return { preview, problems, secretHeaders, secretQuery };

  return {
    preview,
    problems,
    secretHeaders,
    secretQuery,
    request: {
      method: operation.method,
      baseUrl: baseUrl.trim().replace(/\/+$/, ''),
      path,
      query,
      headers,
      body,
    },
  };
};

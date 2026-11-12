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
  headers: DraftHeader[];
  bodyMediaType: string;
  bodyText: string;
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

/** Validates one parameter value; `undefined` when it is acceptable. */
const parameterProblem = (parameter: ConsoleParameter, value: string): Problem | undefined => {
  if (value === '') return parameter.required ? { code: 'required' } : undefined;

  if (parameter.type === 'integer' && !/^-?\d+$/.test(value)) return { code: 'integer' };
  if (parameter.type === 'number' && !(value.trim() !== '' && Number.isFinite(Number(value)))) {
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
  let path = operation.path;

  if (!isSafePath(operation.path)) problems[PATH_PROBLEM_KEY] = { code: 'path' };

  // Console-owned credentials replace anything the user typed under the same name.
  const autoHeaderNames = new Set(autoHeaders.map((h) => h.name.toLowerCase()));
  const autoQueryNames = new Set(autoQuery.map((q) => q.name));

  operation.parameters.forEach((parameter) => {
    const value = (draft.params[paramKey(parameter)] ?? '').trim();
    const problem = parameterProblem(parameter, value);
    if (problem) {
      problems[paramProblemKey(parameter)] = problem;
      return;
    }
    if (value === '') return;

    if (parameter.in === 'path') {
      path = path.split(`{${parameter.name}}`).join(encodePathValue(value));
    } else if (parameter.in === 'query') {
      // An array is entered comma-separated and sent as repeated parameters.
      const values = parameter.type === 'array' ? value.split(',').map((v) => v.trim()) : [value];
      if (!autoQueryNames.has(parameter.name)) {
        values.filter((v) => v !== '').forEach((v) => query.push([parameter.name, v]));
      }
    } else if (!autoHeaderNames.has(parameter.name.toLowerCase())) {
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
          code: problem === 'name' ? 'headerName' : problem === 'forbidden' ? 'headerForbidden' : 'headerValue',
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
        const value = draft.formValues[field.name] ?? '';
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
          const file = draft.files[field.name];
          if (file) parts.push({ name: field.name, file });
          else if (field.required) problems[fieldProblemKey(field.name)] = { code: 'file' };
          return;
        }
        const value = draft.formValues[field.name] ?? '';
        if (value === '') {
          if (field.required) problems[fieldProblemKey(field.name)] = { code: 'required' };
          return;
        }
        parts.push({ name: field.name, value });
      });
      if (parts.length > 0) body = { kind: 'multipart', parts };
    } else {
      const file = draft.files[BINARY_FILE_KEY];
      if (file) body = { kind: 'binary', mediaType: option.mediaType, file };
      else if (operation.body.required) problems[BODY_PROBLEM_KEY] = { code: 'file' };
    }
  }

  if (body?.kind === 'text') headers.push(['Content-Type', body.mediaType]);
  if (body?.kind === 'form') headers.push(['Content-Type', 'application/x-www-form-urlencoded']);
  if (body?.kind === 'binary') headers.push(['Content-Type', body.mediaType]);
  // Multipart: the boundary belongs to whoever serialises the body, so no Content-Type here.

  const secretHeaders = new Set(autoHeaders.filter((h) => h.secret).map((h) => h.name.toLowerCase()));
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

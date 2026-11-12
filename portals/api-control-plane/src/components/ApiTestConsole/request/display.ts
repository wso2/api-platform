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

import type { ConsoleSendRequest } from '../types';

/**
 * Renders a request for the user to read and copy.
 *
 * Two audiences, deliberately different: what is *shown* masks credentials,
 * and what is *copied* reveals them, because a command pasted with a
 * placeholder where the key belongs does not work. Callers choose with
 * `reveal`; nothing here decides it for them.
 */

/** Printed where a credential would be, when secrets are masked. */
export const MASKED_VALUE = '<redacted>';

export type DisplayOptions = {
  reveal: boolean;
  /** Lower-cased header names whose values are credentials. */
  secretHeaders?: ReadonlySet<string>;
  /** Exact query parameter names whose values are credentials. */
  secretQuery?: ReadonlySet<string>;
};

/** Wraps arbitrary text in POSIX `sh` single quotes, so no value can end the quoting. */
export const shellQuote = (value: string): string => `'${value.split("'").join(`'\\''`)}'`;

const queryValue = (name: string, value: string, options: DisplayOptions): string =>
  !options.reveal && options.secretQuery?.has(name) ? MASKED_VALUE : value;

const headerValue = (name: string, value: string, options: DisplayOptions): string =>
  !options.reveal && options.secretHeaders?.has(name.toLowerCase()) ? MASKED_VALUE : value;

/** The full URL: gateway base, path, then the query string (names and values encoded). */
export const displayUrl = (request: ConsoleSendRequest, options: DisplayOptions): string => {
  const query = request.query
    .map(
      ([name, value]) =>
        `${encodeURIComponent(name)}=${encodeURIComponent(queryValue(name, value, options))}`,
    )
    .join('&');
  return `${request.baseUrl}${request.path}${query ? `?${query}` : ''}`;
};

const bodyFlags = (request: ConsoleSendRequest): string[] => {
  const { body } = request;
  if (!body) return [];
  switch (body.kind) {
    case 'text':
      return [`-d ${shellQuote(body.text)}`];
    case 'form':
      return body.fields.map(([name, value]) => `--data-urlencode ${shellQuote(`${name}=${value}`)}`);
    case 'multipart':
      return body.parts.map((part) =>
        // `--form-string` keeps a leading `@` or `<` in a value from being read as a file.
        'file' in part
          ? `-F ${shellQuote(`${part.name}=@${part.file.name}`)}`
          : `--form-string ${shellQuote(`${part.name}=${part.value}`)}`,
      );
    case 'binary':
      return [`--data-binary ${shellQuote(`@${body.file.name}`)}`];
  }
};

/** A line-continued curl command equivalent to the request. */
export const toCurl = (request: ConsoleSendRequest, options: DisplayOptions): string => {
  const lines = [
    `curl -X ${shellQuote(request.method)} ${shellQuote(displayUrl(request, options))}`,
    ...request.headers.map(
      ([name, value]) => `-H ${shellQuote(`${name}: ${headerValue(name, value, options)}`)}`,
    ),
    ...bodyFlags(request),
  ];
  return lines.join(' \\\n  ');
};

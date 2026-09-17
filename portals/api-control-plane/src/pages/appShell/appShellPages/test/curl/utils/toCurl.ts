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

import {
  activeRows,
  bodyKindOf,
  contentTypeFor,
  hasBody,
  type ConsoleRequest,
  type KeyValueRow,
} from '../../utils/types';

/**
 * Renders a `ConsoleRequest` as a copy-pasteable curl command.
 *
 * The command is derived from the console's request model, ensuring it matches
 * the executed request. User-supplied values are safely shell-quoted.
 */

/** Placeholder shown where a credential would be, when secrets are masked. */
export const MASKED_VALUE = '<redacted>';

export type ToCurlOptions = {
  /**
   * Print credential values in full.
   *
   * `false` for on-screen display; `true` for clipboard and file export, since
   * a command copied with `<redacted>` in it does not work. The two callers are
   * intentionally different — see `CurlCommandPanel`.
   */
  revealSecrets: boolean;
};

/** Wraps arbitrary text in safe POSIX `sh` single quotes. */
export const shellQuote = (value: string): string => `'${value.split("'").join(`'\\''`)}'`;

/** A row's value, masked unless secrets are being revealed. */
const displayValue = (row: KeyValueRow, options: ToCurlOptions): string =>
  row.secret && !options.revealSecrets ? MASKED_VALUE : row.value;

/**
 * The full request URL: gateway base, then path, then query string.
 *
 * The path is emitted as typed rather than URL-encoded. It legitimately carries
 * spec placeholders (`/payments/{paymentId}`) and pre-encoded segments, and
 * encoding it would turn a user's `/a/b` into `%2Fa%2Fb`. Query *values*, by
 * contrast, are always encoded — that is where user text lands, and an
 * unencoded `&` there would silently split one parameter into two.
 */
export const buildRequestUrl = (
  request: ConsoleRequest,
  options: ToCurlOptions = { revealSecrets: true },
): string => {
  const base = request.baseUrl.trim().replace(/\/+$/, '');
  const rawPath = request.path.trim();
  const path = rawPath === '' || rawPath.startsWith('/') ? rawPath : `/${rawPath}`;

  // Mask secret query values consistently with secret headers. Defaults to
  // revealing so callers building a real URL can dial it.
  const query = activeRows(request.queryParams)
    .map(
      (row) =>
        `${encodeURIComponent(row.name.trim())}=${encodeURIComponent(displayValue(row, options))}`,
    )
    .join('&');

  return `${base}${path}${query ? `?${query}` : ''}`;
};

/**
 * A one-line summary of what the command does — "POST · 2 headers · JSON body".
 *
 * Returned as parts rather than a sentence because the console renders it as
 * translated copy; joining it here would bake English word order in.
 */
export const curlSummary = (
  request: ConsoleRequest,
): { method: string; headerCount: number; bodyKind: string } => ({
  bodyKind: bodyKindOf(request),
  headerCount: activeRows(request.headers).length,
  method: request.method,
});

/**
 * Returns curl body flags, preserving raw bodies and encoding form fields
 * safely. Multipart fields use `-F` so curl generates the boundary.
 */
const bodyFlags = (request: ConsoleRequest, options: ToCurlOptions): string[] => {
  if (!hasBody(request)) return [];

  if (request.bodyMode === 'raw') {
    return [`-d ${shellQuote(request.body)}`];
  }

  const flag = request.bodyMode === 'form-data' ? '--form-string' : '--data-urlencode';
  return activeRows(request.formFields).map(
    (field) => `${flag} ${shellQuote(`${field.name.trim()}=${displayValue(field, options)}`)}`,
  );
};

/**
 * The curl command for a request.
 *
 * Line-continued across multiple lines to stay readable in a narrow panel and
 * to paste into a terminal unchanged. `-X` is emitted even for GET so the
 * command still says what it does after someone edits the URL.
 */
export const toCurl = (request: ConsoleRequest, options: ToCurlOptions): string => {
  const lines = [
    `curl -X ${request.method} \\`,
    `  ${shellQuote(buildRequestUrl(request, options))} \\`,
  ];

  const headers = activeRows(request.headers);
  headers.forEach((row) => {
    lines.push(`  -H ${shellQuote(`${row.name.trim()}: ${displayValue(row, options)}`)} \\`);
  });

  /**
   * Derive Content-Type from the body unless the user explicitly set a header.
   */
  const declaresContentType = headers.some(
    (row) => row.name.trim().toLowerCase() === 'content-type',
  );
  const contentType = declaresContentType ? undefined : contentTypeFor(request);
  if (contentType) {
    lines.push(`  -H ${shellQuote(`Content-Type: ${contentType}`)} \\`);
  }

  bodyFlags(request, options).forEach((flag) => lines.push(`  ${flag} \\`));

  // Drop the trailing continuation from whichever line ended up last, rather
  // than tracking which one that is while building.
  return lines.join('\n').replace(/ \\$/, '');
};

/**
 * The command wrapped as a runnable shell script, for the `.sh` export.
 *
 * `set -euo pipefail` is not ceremony here: without `-e` a failed curl in a
 * downloaded script exits 0, which reads as success.
 */
export const toCurlScript = (request: ConsoleRequest): string =>
  ['#!/usr/bin/env sh', 'set -eu', '', toCurl(request, { revealSecrets: true }), ''].join('\n');

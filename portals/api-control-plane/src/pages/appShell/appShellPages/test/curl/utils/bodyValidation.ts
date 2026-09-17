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

import type { RawFormat } from '../../utils/types';

/**
 * As-you-type validation for a raw request body.
 *
 * Pure and separated from the editor so the parsing rules can be tested without
 * mounting anything, and so the editor holds layout rather than logic.
 *
 * The parser's own message is passed back verbatim rather than replaced with
 * wording of our own: it names the offending position, which is the only part
 * that helps someone fix the body. The editor presents it as an untranslated
 * value.
 */

export type BodyValidation =
  | { state: 'empty' }
  | { state: 'valid' }
  | { state: 'invalid'; reason: string }
  | { state: 'unchecked' };

/** Parses JSON purely to find out whether it parses. */
const validateJson = (body: string): BodyValidation => {
  try {
    JSON.parse(body);
    return { state: 'valid' };
  } catch (error) {
    return { reason: error instanceof Error ? error.message : '', state: 'invalid' };
  }
};

/**
 * Checks XML for well-formedness with the browser's own parser.
 *
 * `DOMParser` is used rather than a dependency, and only ever to answer
 * "is this well-formed": the result is inspected for a `parsererror` node and
 * then discarded. The document is never inserted anywhere, so nothing here can
 * render markup the user typed.
 *
 * A `DOCTYPE` is rejected before parsing. Browsers do not resolve external
 * entities in `DOMParser`, so this is not the XXE hole it would be on a server
 * — but a body carrying a DTD is far more likely to be a mistake than an
 * intention, and refusing it keeps this path aligned with the portal's rule for
 * XML input everywhere else.
 */
const validateXml = (body: string): BodyValidation => {
  if (/<!DOCTYPE/i.test(body)) {
    return { reason: 'A DOCTYPE declaration is not allowed', state: 'invalid' };
  }

  if (typeof DOMParser === 'undefined') return { state: 'unchecked' };

  const parsed = new DOMParser().parseFromString(body, 'application/xml');
  const failure = parsed.getElementsByTagName('parsererror')[0];
  if (!failure) return { state: 'valid' };

  // Browsers phrase this differently and often across several lines; the first
  // line is the part that names the problem.
  const reason = (failure.textContent ?? '').trim().split('\n')[0];
  return { reason: reason || 'Malformed XML', state: 'invalid' };
};

/** Validates a raw body according to its declared format. */
export const validateBody = (body: string, format: RawFormat): BodyValidation => {
  if (body.trim() === '') return { state: 'empty' };
  if (format === 'json') return validateJson(body);
  if (format === 'xml') return validateXml(body);
  // Plain text has nothing to be wrong about.
  return { state: 'unchecked' };
};

/**
 * Re-indents a body, or returns `undefined` when it cannot be.
 *
 * Only JSON is re-indentable here: `JSON.parse`/`stringify` round-trips it
 * exactly. Pretty-printing XML would mean inserting whitespace into text
 * content, which changes the bytes the server receives, and plain text has no
 * structure to indent — so both are left alone rather than mangled.
 */
export const formatBody = (body: string, format: RawFormat): string | undefined => {
  if (format !== 'json') return undefined;
  try {
    return JSON.stringify(JSON.parse(body), null, 2);
  } catch {
    return undefined;
  }
};

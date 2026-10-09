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

import { VERSION_PATTERN } from '../../utils/basicInfoRules';

/**
 * A name and version for an API created from an endpoint, read off the
 * backend URL instead of starting every one as "Untitled API" 1.0.0.
 *
 * The path, not the host, carries the name. Many APIs share a host, so a host
 * name collides as soon as a second API arrives, and it says where the API
 * lives rather than what it is. Walking the path from the right and skipping
 * versions and generic routing words lands on the resource:
 * `…/reading-list-api-service/v1.0` → "Reading List API Service".
 *
 * Both guesses are suggestions in an editable form, so a wrong one costs a
 * keystroke; an empty one falls back to the form's own default.
 */

/** `v1`, `v1.0`, `v2beta`, `42`, `2026-01-01`: a version or an id, never a name. */
const VERSION_SEGMENT = /^(v\d+[a-z0-9.-]*|\d+(\.\d+)*|\d{4}-\d{2}-\d{2})$/i;

/** Segments that appear in most API URLs and identify nothing. */
const GENERIC_SEGMENTS = new Set([
  'api',
  'apis',
  'default',
  'gateway',
  'public',
  'rest',
  'restapi',
  'service',
  'services',
  'svc',
]);

/** Written in capitals by convention. */
const ACRONYMS = new Set(['ai', 'api', 'http', 'id', 'ml', 'sms', 'ui', 'url']);

/** Longest suggested name; long names make for unwieldy identifiers. */
const NAME_MAX_LENGTH = 60;

const humanise = (raw: string): string =>
  raw
    .split(/[-_.]+/)
    .filter(Boolean)
    .map((word) =>
      ACRONYMS.has(word.toLowerCase())
        ? word.toUpperCase()
        : word.charAt(0).toUpperCase() + word.slice(1).toLowerCase(),
    )
    .join(' ');

/** A candidate worth offering: starts with a letter, a sensible length. */
const usable = (name: string): string =>
  /^[A-Za-z]/.test(name) && name.length >= 3 && name.length <= NAME_MAX_LENGTH ? name : '';

const parse = (url: string): URL | undefined => {
  try {
    return new URL(url.trim());
  } catch {
    return undefined;
  }
};

/** A readable API name from a backend URL, or '' when nothing usable is there. */
export const nameFromEndpoint = (endpointUrl: string): string => {
  const parsed = parse(endpointUrl);
  if (!parsed) return '';

  const segments = parsed.pathname.split('/').filter(Boolean);
  for (let index = segments.length - 1; index >= 0; index -= 1) {
    const segment = decodeURIComponent(segments[index]);
    const lower = segment.toLowerCase();
    if (VERSION_SEGMENT.test(lower) || GENERIC_SEGMENTS.has(lower)) continue;
    const name = usable(humanise(segment));
    if (name) return name;
  }

  // An IP address names nothing; a single-label host (`billing`, a container
  // or cluster service name) often does.
  if (/^[\d.]+$/.test(parsed.hostname) || parsed.hostname.startsWith('[')) return '';
  const host = parsed.hostname.replace(/^(www|api|apis|gateway)\./i, '').split('.')[0];
  return usable(humanise(host));
};

/**
 * The version a backend URL states, as `<major>.<minor>.0`, or '' when it
 * states none. Only an explicit `v<major>[.<minor>]` segment counts: a bare
 * number is far more often an id. The API's version is its own, not the
 * backend's, so this is a suggestion that happens to be right most of the time.
 */
export const versionFromEndpoint = (endpointUrl: string): string => {
  const parsed = parse(endpointUrl);
  if (!parsed) return '';

  const segments = parsed.pathname.split('/').filter(Boolean);
  for (let index = segments.length - 1; index >= 0; index -= 1) {
    const match = segments[index].match(/^v(0|[1-9]\d{0,5})(?:\.(0|[1-9]\d{0,5}))?$/i);
    if (!match) continue;
    const candidate = `${match[1]}.${match[2] ?? '0'}.0`;
    if (VERSION_PATTERN.test(candidate)) return candidate;
  }
  return '';
};

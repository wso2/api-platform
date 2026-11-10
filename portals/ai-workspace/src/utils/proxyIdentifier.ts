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

import { getErrorCode } from './apiError';
import { toSlug } from './slug';

/** The longest identifier the Platform API accepts for a proxy. */
export const MAX_PROXY_ID_LENGTH = 40;

/** The error codes the API answers with when an identifier is already taken. */
export const PROXY_ID_TAKEN_CODE = 'LLM_PROXY_EXISTS';
export const MCP_PROXY_ID_TAKEN_CODE = 'MCP_PROXY_EXISTS';
export const AGENT_PROXY_ID_TAKEN_CODE = 'AGENT_PROXY_EXISTS';

/** How many times a failed create may be re-run under a new identifier. */
export const MAX_ID_RETRIES = 2;

/** Upper bound on counters tried, so a misbehaving lookup can't loop forever. */
const MAX_ID_COUNTER = 99;

/** Derive a URL-safe identifier from a display name. */
export const toProxyId = (name: string): string => toSlug(name);

/** The context a proxy is served under by default: /<project>/<id>. */
export const buildProxyContext = (projectSlug: string, proxyId: string): string =>
  projectSlug ? `/${projectSlug}/${proxyId}` : `/${proxyId}`;

/** An identifier together with the counter it carries (1 is the plain one). */
export interface ProxyIdChoice {
  id: string;
  n: number;
}

/**
 * The identifier for counter `n`: the plain one for 1, otherwise "<base>-<n>".
 * The base is shortened so the suffix still fits the length limit, without
 * leaving a dangling hyphen.
 */
export const proxyIdWithCounter = (base: string, n: number): string => {
  if (n <= 1) return base;
  const suffix = `-${n}`;
  const room = MAX_PROXY_ID_LENGTH - suffix.length;
  return `${base.slice(0, room).replace(/-+$/, '')}${suffix}`;
};

/**
 * Pick an identifier that is not taken yet.
 *
 * Two proxies may share a display name, so the identifier — not the name —
 * has to be unique. The first candidate is the name's own identifier
 * ("openai-proxy"); when `exists` reports it taken, a hyphenated counter is
 * appended ("openai-proxy-2", "openai-proxy-3", ...) until one is free. The
 * hyphen keeps the counter apart from digits already in the name.
 *
 * `startAt` is for a caller whose create just failed on an identifier: the
 * first counter not yet known to be taken, so nothing already refused is asked
 * about again. Without it the plain identifier is checked first.
 */
export const resolveUniqueProxyId = async (
  displayName: string,
  exists: (candidate: string) => Promise<boolean>,
  options: { startAt?: number } = {}
): Promise<ProxyIdChoice> => {
  const base = toProxyId(displayName);
  if (!base) return { id: '', n: 0 };
  const startAt = options.startAt && options.startAt >= 2 ? options.startAt : 1;
  for (let n = startAt; n <= MAX_ID_COUNTER; n += 1) {
    const candidate = proxyIdWithCounter(base, n);
    if (!(await exists(candidate))) return { id: candidate, n };
  }
  throw new Error('Could not find an available identifier for this name.');
};

/**
 * After a failed create, decide whether to run it again and under which
 * identifier. Returns null — meaning "report the original error" — unless the
 * failure was an identifier clash, retries remain, and a free identifier was
 * found. A lookup that fails is reported through `onLookupError`, not thrown.
 */
export const nextProxyIdAfterFailure = async (args: {
  error: unknown;
  displayName: string;
  failedCounter: number;
  attempt: number;
  exists: (candidate: string) => Promise<boolean>;
  onLookupError?: (error: unknown) => void;
  /** The error code that means "identifier taken"; defaults to the LLM proxy's. */
  takenCode?: string;
}): Promise<ProxyIdChoice | null> => {
  const { error, displayName, failedCounter, attempt, exists, onLookupError } = args;
  const takenCode = args.takenCode ?? PROXY_ID_TAKEN_CODE;
  if (getErrorCode(error) !== takenCode || attempt >= MAX_ID_RETRIES) return null;
  try {
    const choice = await resolveUniqueProxyId(displayName, exists, {
      startAt: failedCounter + 1,
    });
    return choice.id ? choice : null;
  } catch (lookupError) {
    onLookupError?.(lookupError);
    return null;
  }
};

/**
 * Run `create` under the name's own identifier and, if the API reports it
 * taken, again under the next free one, up to MAX_ID_RETRIES times. For a
 * create that has nothing to undo between attempts. Any other failure, or an
 * identifier that cannot be resolved, is rethrown as the original error.
 * Resolves with what `create` returned and the identifier it succeeded under.
 */
export const createWithUniqueId = async <T>(args: {
  displayName: string;
  create: (choice: ProxyIdChoice) => Promise<T>;
  exists: (candidate: string) => Promise<boolean>;
  takenCode: string;
  onLookupError?: (error: unknown) => void;
}): Promise<{ result: T; choice: ProxyIdChoice }> => {
  const { displayName, create, exists, takenCode, onLookupError } = args;
  let choice: ProxyIdChoice = { id: toProxyId(displayName), n: 1 };
  for (let attempt = 0; ; attempt += 1) {
    try {
      return { result: await create(choice), choice };
    } catch (error) {
      const next = await nextProxyIdAfterFailure({
        error,
        displayName,
        failedCounter: choice.n,
        attempt,
        exists,
        onLookupError,
        takenCode,
      });
      if (!next) throw error;
      choice = next;
    }
  }
};

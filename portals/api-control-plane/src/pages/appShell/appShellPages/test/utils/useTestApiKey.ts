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

import { useCallback, useEffect, useState } from 'react';

import { useCreateApiKey, type CreateApiKeyResponse } from '@/api/resources/apiKeys';
import type { ApiError } from '@/api/core/errors';
import { isTestKeyExpired, testApiKeyBody, toTestApiKey, type TestApiKey } from './testApiKey';

/**
 * Obtains the test console's key, minting one through `apiKeys` when needed.
 *
 * ## Why a hook here rather than a resource module
 *
 * A test key is not its own resource: it is an ordinary API key with a short
 * expiry, so it is created through `useCreateApiKey` like any other. What the
 * console needs on top of that is *memory* — obtaining a key and creating one
 * are the same operation, because the plaintext is returned exactly once and no
 * endpoint can hand it back. Without somewhere to keep it, every remount would
 * mint another real, persisted key into the user's key list.
 *
 * That memory deliberately does not live in the query cache.
 * `apiKeys.hooks.ts` makes the point in its own words: a secret that only
 * exists once should not sit in a store other components can read. So it lives
 * here, in the one feature that needs it, in a module-scoped map that dies with
 * the tab — never `localStorage`, never anything that outlives the session.
 *
 * The practical result is one key per API per browser session: navigating away
 * and back re-uses it, and only an expiry or an explicit "New key" mints
 * another.
 */

/**
 * Keys minted this session, by REST API id.
 *
 * Module scope rather than component state so the value survives the page
 * unmounting. Values are read back only through `liveKey`, which discards an
 * expired entry rather than handing back a credential that no longer works.
 */
const mintedKeys = new Map<string, TestApiKey>();

/**
 * Mints in flight, by REST API id.
 *
 * Without this, two things each mint a duplicate key: React's development
 * double-invoked effects, and two components asking at once. Sharing the
 * promise makes the second caller await the first request instead of issuing
 * its own.
 */
const pendingMints = new Map<string, Promise<TestApiKey>>();

/** A remembered key that is still usable, or `undefined`. */
const liveKey = (restApiId: string | undefined, now: number): TestApiKey | undefined => {
  if (!restApiId) return undefined;
  const key = mintedKeys.get(restApiId);
  if (!key) return undefined;
  if (isTestKeyExpired(key, now)) {
    mintedKeys.delete(restApiId);
    return undefined;
  }
  return key;
};

/** Clears everything this session remembered. For tests. */
export const resetTestApiKeys = (): void => {
  mintedKeys.clear();
  pendingMints.clear();
};

export type UseTestApiKeyResult = {
  key?: TestApiKey;
  /** True while the first key for this API is being minted. */
  isPending: boolean;
  /** True while an explicit replacement is being minted. */
  isRegenerating: boolean;
  error?: ApiError;
  /** Issues a replacement key and forgets the previous one. */
  regenerate: () => void;
};

/**
 * The console's key for one API.
 *
 * `enabled` is what gates minting: an API whose `api-key-auth` policy does not
 * demand a key must never have one created against it, so passing `false`
 * leaves this hook completely inert rather than merely hiding the result.
 */
export const useTestApiKey = (
  restApiId: string | undefined,
  enabled: boolean,
): UseTestApiKeyResult => {
  const createApiKey = useCreateApiKey();
  const [key, setKey] = useState<TestApiKey | undefined>(() => liveKey(restApiId, Date.now()));
  const [isPending, setIsPending] = useState(false);
  const [isRegenerating, setIsRegenerating] = useState(false);
  const [error, setError] = useState<ApiError | undefined>(undefined);

  const { mutateAsync } = createApiKey;

  /**
   * Mints a key, sharing an in-flight request rather than duplicating it.
   *
   * `requestedAt` is captured before the call so the derived deadline is
   * conservative — see `toTestApiKey`.
   */
  const mint = useCallback(
    (id: string): Promise<TestApiKey> => {
      const existing = pendingMints.get(id);
      if (existing) return existing;

      const requestedAt = Date.now();
      const promise = mutateAsync({ body: testApiKeyBody(), restApiId: id })
        .then((response: CreateApiKeyResponse) => {
          const minted = toTestApiKey(response, requestedAt);
          mintedKeys.set(id, minted);
          return minted;
        })
        .finally(() => {
          pendingMints.delete(id);
        });

      pendingMints.set(id, promise);
      return promise;
    },
    // Depends only on the mutation: the id is an argument, so this cannot mint
    // against a stale one after a navigation.
    [mutateAsync],
  );

  /** Mints the first key for this API, once, when one is actually required. */
  useEffect(() => {
    if (!enabled || !restApiId) return;

    const remembered = liveKey(restApiId, Date.now());
    if (remembered) {
      setKey(remembered);
      return;
    }

    let active = true;
    setIsPending(true);
    setError(undefined);

    mint(restApiId)
      .then((minted) => {
        // Guarded so a resolved mint cannot write into a component that has
        // since unmounted or moved to another API. The key itself is already
        // remembered above, so nothing is lost by dropping the state write.
        if (active) setKey(minted);
      })
      .catch((cause: ApiError) => {
        if (active) setError(cause);
      })
      .finally(() => {
        if (active) setIsPending(false);
      });

    return () => {
      active = false;
    };
  }, [enabled, mint, restApiId]);

  const regenerate = useCallback(() => {
    if (!restApiId) return;

    // Forgotten first, so a failed replacement cannot leave the previous key on
    // screen looking current while the request that was meant to replace it has
    // already been abandoned.
    mintedKeys.delete(restApiId);
    setIsRegenerating(true);
    setError(undefined);

    mint(restApiId)
      .then(setKey)
      .catch((cause: ApiError) => setError(cause))
      .finally(() => setIsRegenerating(false));
  }, [mint, restApiId]);

  return {
    error,
    isPending,
    isRegenerating,
    key: enabled ? key : undefined,
    regenerate,
  };
};

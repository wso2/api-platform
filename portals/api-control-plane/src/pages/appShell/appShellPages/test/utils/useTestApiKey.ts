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
 * Obtains the test console's API key, creating one when necessary.
 *
 * Test keys are ordinary, short-lived API keys created through
 * `useCreateApiKey`. Because their plaintext is returned only once, this hook
 * retains each key in a module-scoped map rather than the query cache or
 * persistent storage. The key is therefore reused across remounts within the
 * browser session and replaced only after expiry or an explicit request.
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

/** The held key and the API it was minted against. */
type HeldKey = { key: TestApiKey; ownerId: string };

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
  const [held, setHeld] = useState<HeldKey | undefined>(() => {
    const remembered = restApiId ? liveKey(restApiId, Date.now()) : undefined;
    return remembered && restApiId ? { key: remembered, ownerId: restApiId } : undefined;
  });
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
      setHeld({ key: remembered, ownerId: restApiId });
      return;
    }

    let active = true;
    setIsPending(true);
    setError(undefined);

    mint(restApiId)
      .then((minted) => {
        // Ignore results after unmount or API changes; the key is already cached.
        if (active) setHeld({ key: minted, ownerId: restApiId });
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

    // Forget both the session value and the rendered key before replacement.
    mintedKeys.delete(restApiId);
    setHeld(undefined);
    setIsRegenerating(true);
    setError(undefined);

    mint(restApiId)
      .then((minted) => setHeld({ key: minted, ownerId: restApiId }))
      .catch((cause: ApiError) => setError(cause))
      .finally(() => setIsRegenerating(false));
  }, [mint, restApiId]);

  // Return the key only for the API it was minted against.
  const key = held && held.ownerId === restApiId ? held.key : undefined;

  return {
    error,
    isPending,
    isRegenerating,
    key: enabled ? key : undefined,
    regenerate,
  };
};

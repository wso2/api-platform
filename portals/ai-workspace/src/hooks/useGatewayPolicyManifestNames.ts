/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import { useEffect, useState } from 'react';
import { getGatewayPolicyManifest } from '../apis/gatewayPolicyApis';
import { getGatewayManifestPolicyNameSet } from '../utils/policyCompatibility';
import { logger } from '../utils/logger';

interface UseGatewayPolicyManifestNamesResult {
  /**
   * Normalized policy names installed on the gateway. `null` while loading,
   * when no gateway was given, or when the manifest could not be read —
   * callers should treat `null` as "unknown" rather than "no policies
   * installed" and skip flagging anything as missing in that case.
   */
  policyNameSet: Set<string> | null;
  isLoading: boolean;
  error: Error | null;
}

/**
 * Module-level cache so repeat mounts for the same gateway (re-expanding a
 * card, navigating away and back to a deploy page) don't re-fetch its
 * manifest every time — a deploy page can render one card per gateway, each
 * independently calling this hook. Short TTL rather than session-lived: a
 * custom policy synced to the gateway elsewhere (Gateway Policies tab) should
 * show up here again within a bounded window, not only after a full reload.
 */
const MANIFEST_CACHE_TTL_MS = 60_000;

interface CacheEntry {
  names: Set<string>;
  expiresAt: number;
}

const manifestCache = new Map<string, CacheEntry>();
const inFlightRequests = new Map<string, Promise<Set<string>>>();

const getFreshCachedNames = (gatewayId: string): Set<string> | null => {
  const entry = manifestCache.get(gatewayId);
  if (!entry || Date.now() >= entry.expiresAt) return null;
  return entry.names;
};

/** Fetches (and caches/dedupes) the policy name set for one gateway. */
const fetchPolicyNameSet = (gatewayId: string): Promise<Set<string>> => {
  const cached = getFreshCachedNames(gatewayId);
  if (cached) return Promise.resolve(cached);

  const inFlight = inFlightRequests.get(gatewayId);
  if (inFlight) return inFlight;

  const request = getGatewayPolicyManifest(gatewayId)
    .then((manifest) => {
      const names = getGatewayManifestPolicyNameSet(manifest.policies ?? []);
      manifestCache.set(gatewayId, { names, expiresAt: Date.now() + MANIFEST_CACHE_TTL_MS });
      return names;
    })
    .finally(() => {
      inFlightRequests.delete(gatewayId);
    });

  inFlightRequests.set(gatewayId, request);
  return request;
};

/**
 * Reads the policy manifest for a single gateway and reduces it to a set of
 * installed policy names. Pass `null` to skip the fetch (e.g. while the
 * caller has nothing to compare against, or lacks the read scope).
 */
export function useGatewayPolicyManifestNames(
  gatewayId: string | null
): UseGatewayPolicyManifestNamesResult {
  const initialCached = gatewayId ? getFreshCachedNames(gatewayId) : null;
  const [policyNameSet, setPolicyNameSet] = useState<Set<string> | null>(initialCached);
  const [isLoading, setIsLoading] = useState(Boolean(gatewayId) && !initialCached);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (!gatewayId) {
      setPolicyNameSet(null);
      setIsLoading(false);
      setError(null);
      return;
    }

    const cached = getFreshCachedNames(gatewayId);
    if (cached) {
      setPolicyNameSet(cached);
      setIsLoading(false);
      setError(null);
      return;
    }

    let cancelled = false;
    setIsLoading(true);
    setError(null);

    fetchPolicyNameSet(gatewayId)
      .then((names) => {
        if (cancelled) return;
        setPolicyNameSet(names);
      })
      .catch((err) => {
        if (cancelled) return;
        logger.error(`Failed to load policy manifest for gateway ${gatewayId}:`, err);
        setError(
          err instanceof Error ? err : new Error('Failed to load gateway policy manifest')
        );
        setPolicyNameSet(null);
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [gatewayId]);

  return { policyNameSet, isLoading, error };
}

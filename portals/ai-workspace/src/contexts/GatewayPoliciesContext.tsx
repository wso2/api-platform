/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import {
  getGatewayCustomPolicy,
  getGatewayCustomPolicies,
  getGatewayPolicyManifest,
  syncGatewayCustomPolicy,
} from '../apis/gatewayPolicyApis';
import type {
  GatewayCustomPolicy,
  GatewayManifestPolicy,
} from '../apis/gatewayPolicyApis';
import { getPolicies } from '../apis/policyHubApis';
import type { PolicyHubPolicy } from '../utils/types';
import { useAppAuth } from './AppAuthContext';
import { SCOPES } from '../auth/permissions';

export type GatewayPolicyRow = {
  key: string;
  policyName: string;
  name: string;
  version: string;
  description: string;
  policyType: "Policy Hub" | "Custom";
  /**
   * "Unknown" is used when the organization's custom policies could not be read
   * (no scope, or a failed request), so whether this policy is synced cannot be
   * determined — distinct from a policy known to be unsynced.
   */
  syncStatus: "N/A" | "Synced" | "Not synced" | "Unknown";
  customPolicyId?: string;
};

type GatewayPoliciesContextValue = {
  policies: GatewayPolicyRow[];
  isLoading: boolean;
  error: Error | null;
  /**
   * Non-fatal load failures: the manifest arrived but a supplementary source
   * (the org's custom policies, or the Policy Hub catalogue) did not. The rows
   * are still usable, so these are shown as warnings instead of replacing the
   * table with an error.
   */
  warnings: string[];
  refresh: () => Promise<void>;
  syncPolicy: (policyName: string, version: string) => Promise<GatewayCustomPolicy>;
  syncingPolicyKey: string | null;
  /** False when the caller can read neither source the policy view lists. */
  canViewPolicies: boolean;
  /** False when the caller cannot read this gateway's policy manifest. */
  canViewManifest: boolean;
  /** False when the caller cannot read the organization's custom policies. */
  canViewCustomPolicies: boolean;
  /** False when the caller may view policies but not sync them into the org. */
  canSyncPolicies: boolean;
};

const GatewayPoliciesContext = createContext<GatewayPoliciesContextValue | null>(null);

const normalizedVersion = (version: string) => version.replace(/^v/i, "");
const policyKey = (name: string, version: string) =>
  `${name.trim().toLowerCase()}@${normalizedVersion(version)}`;
const isCustomManifestPolicy = (policy: GatewayManifestPolicy) => {
  if (typeof policy.isCustomPolicy === "boolean") return policy.isCustomPolicy;
  const managedBy = policy.managedBy?.trim().toLowerCase();
  return managedBy === "organization" || managedBy === "customer";
};

function mergePolicies(
  manifestPolicies: GatewayManifestPolicy[],
  customPolicies: GatewayCustomPolicy[],
  hubPolicies: PolicyHubPolicy[],
  options: {
    /**
     * True when the org's custom policies could not be read, so sync status is
     * reported as "Unknown" rather than being inferred from an empty list.
     */
    syncStatusUnknown: boolean;
    /**
     * True when the manifest could not be read. The org's custom policies then
     * become the only listable source, so they are surfaced as rows on their own
     * instead of only enriching manifest rows.
     */
    listCustomPoliciesAsRows: boolean;
  },
): GatewayPolicyRow[] {
  const rows = new Map<string, GatewayPolicyRow>();
  const hubPolicyByKey = new Map(
    hubPolicies.map((policy) => [policyKey(policy.name, policy.version), policy]),
  );

  const synced = new Map(
    customPolicies.map((policy) => [policyKey(policy.name, policy.version), policy]),
  );
  manifestPolicies.forEach((policy) => {
    const key = policyKey(policy.name, policy.version);
    const hubPolicy = hubPolicyByKey.get(key);
    const syncedPolicy = synced.get(key);
    const isCustomPolicy = isCustomManifestPolicy(policy);
    rows.set(key, {
      key,
      policyName: policy.name,
      name: policy.displayName || hubPolicy?.displayName || policy.name,
      // Keep the controller-reported value (for example, "v1.0.0") for API
      // calls because SyncCustomPolicy matches the stored manifest version.
      version: policy.version,
      description: policy.description || hubPolicy?.description || "—",
      policyType: isCustomPolicy ? "Custom" : "Policy Hub",
      syncStatus: !isCustomPolicy
        ? "N/A"
        : options.syncStatusUnknown
          ? "Unknown"
          : syncedPolicy
            ? "Synced"
            : "Not synced",
      customPolicyId: syncedPolicy?.uuid,
    });
  });

  if (options.listCustomPoliciesAsRows) {
    customPolicies.forEach((policy) => {
      const key = policyKey(policy.name, policy.version);
      if (rows.has(key)) return;
      const hubPolicy = hubPolicyByKey.get(key);
      rows.set(key, {
        key,
        policyName: policy.name,
        name: policy.displayName || hubPolicy?.displayName || policy.name,
        version: policy.version,
        description: policy.description || hubPolicy?.description || "—",
        policyType: "Custom",
        // Present in the organization by definition — that is what this list is.
        syncStatus: "Synced",
        customPolicyId: policy.uuid,
      });
    });
  }

  return [...rows.values()].sort((left, right) => left.name.localeCompare(right.name));
}

export function GatewayPoliciesProvider({ gatewayId, children }: { gatewayId: string; children: React.ReactNode }) {
  const [policies, setPolicies] = useState<GatewayPolicyRow[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [syncingPolicyKey, setSyncingPolicyKey] = useState<string | null>(null);

  // This view lists two sources: the gateway's policy manifest and the org's
  // custom policies. They are scoped independently, so each is requested only
  // when its own scope is held — holding just one still gives a usable (if
  // partial) list, and the missing half is reported as a warning. Only when
  // neither is readable does the consumer fall back to a permission notice.
  const { hasPermission } = useAppAuth();
  const canViewManifest = hasPermission(SCOPES.GATEWAY_MANIFEST_READ);
  const canViewCustomPolicies = hasPermission(SCOPES.GATEWAY_CUSTOM_POLICY_READ);
  const canViewPolicies = canViewManifest || canViewCustomPolicies;
  const canSyncPolicies = hasPermission(SCOPES.GATEWAY_CUSTOM_POLICY_CREATE);

  const refresh = useCallback(async () => {
    if (!gatewayId) return;
    if (!canViewPolicies) {
      setPolicies([]);
      setWarnings([]);
      setError(null);
      setIsLoading(false);
      return;
    }
    setIsLoading(true);
    setError(null);
    setWarnings([]);

    // Every readable source starts together. The manifest is awaited on its own
    // so its failure is known without waiting on the supplementary requests;
    // `supplementary` is an allSettled promise, so it never rejects even when the
    // manifest fails first. Neither source is fatal by itself — the table is
    // rendered from whichever ones arrived, and the rest become warnings.
    const manifestPromise = canViewManifest
      ? getGatewayPolicyManifest(gatewayId)
      : null;
    const supplementary = Promise.allSettled([
      canViewCustomPolicies ? getGatewayCustomPolicies() : Promise.resolve(null),
      getPolicies(),
    ]);

    let manifestPolicies: GatewayManifestPolicy[] | null = null;
    let manifestFailure: unknown = null;
    if (manifestPromise) {
      try {
        manifestPolicies = (await manifestPromise).policies || [];
      } catch (cause) {
        manifestFailure = cause;
      }
    }
    const [customResult, hubResult] = await supplementary;

    const customPolicies =
      customResult.status === "fulfilled" ? customResult.value?.list || [] : [];
    const hubPolicies =
      hubResult.status === "fulfilled" ? hubResult.value.data || [] : [];
    const customPoliciesUnavailable =
      !canViewCustomPolicies || customResult.status === "rejected";

    // A manifest request that was actually made and failed is fatal. Unlike a
    // missing manifest scope — where the gateway's installed policies are
    // known to be out of view — a failed fetch leaves them unknown, so the
    // custom-policy list must not silently stand in for the full picture.
    // Beyond that, nothing listable from either source is the only full failure;
    // anything else degrades to a partial list plus a warning.
    if (manifestFailure || (manifestPolicies === null && customPoliciesUnavailable)) {
      const cause =
        manifestFailure ??
        (customResult.status === "rejected" ? customResult.reason : null);
      setPolicies([]);
      setWarnings([]);
      setError(
        cause instanceof Error
          ? cause
          : new Error("Failed to load gateway policies"),
      );
      setIsLoading(false);
      return;
    }

    const nextWarnings: string[] = [];
    if (!canViewManifest) {
      nextWarnings.push(
        "Policies installed on this gateway are not shown — you do not have permission to read the gateway manifest. Only custom policies synced to the organization are listed.",
      );
    }
    if (!canViewCustomPolicies) {
      nextWarnings.push(
        "Sync status is not shown - you do not have permission to read the organization's custom policies.",
      );
    } else if (customResult.status === "rejected") {
      nextWarnings.push(
        "Custom policies could not be loaded, so sync status is not shown.",
      );
    }
    if (hubResult.status === "rejected") {
      nextWarnings.push(
        "Policy Hub details could not be loaded, so some names and descriptions may be missing.",
      );
    }

    setWarnings(nextWarnings);
    setPolicies(
      mergePolicies(manifestPolicies || [], customPolicies, hubPolicies, {
        syncStatusUnknown: customPoliciesUnavailable,
        listCustomPoliciesAsRows: manifestPolicies === null,
      }),
    );
    setIsLoading(false);
  }, [gatewayId, canViewPolicies, canViewManifest, canViewCustomPolicies]);

  useEffect(() => { void refresh(); }, [refresh]);

  const syncPolicy = useCallback(async (policyName: string, version: string) => {
    const key = policyKey(policyName, version);
    setSyncingPolicyKey(key);
    try {
      const syncedPolicy = await syncGatewayCustomPolicy(gatewayId, policyName, version);

      // Read the persisted version back through the detail resource before the UI
      // reports success. This also ensures the returned UUID/version can be used by
      // subsequent custom-policy operations.
      const persistedPolicy = await getGatewayCustomPolicy(
        syncedPolicy.uuid,
        syncedPolicy.version,
      );
      await refresh();
      return persistedPolicy;
    } finally {
      setSyncingPolicyKey(null);
    }
  }, [gatewayId, refresh]);

  const value = useMemo(
    () => ({
      policies,
      isLoading,
      error,
      warnings,
      refresh,
      syncPolicy,
      syncingPolicyKey,
      canViewPolicies,
      canViewManifest,
      canViewCustomPolicies,
      canSyncPolicies,
    }),
    [
      policies,
      isLoading,
      error,
      warnings,
      refresh,
      syncPolicy,
      syncingPolicyKey,
      canViewPolicies,
      canViewManifest,
      canViewCustomPolicies,
      canSyncPolicies,
    ],
  );
  return <GatewayPoliciesContext.Provider value={value}>{children}</GatewayPoliciesContext.Provider>;
}

export function useGatewayPolicies() {
  const context = useContext(GatewayPoliciesContext);
  if (!context) throw new Error("useGatewayPolicies must be used within GatewayPoliciesProvider");
  return context;
}

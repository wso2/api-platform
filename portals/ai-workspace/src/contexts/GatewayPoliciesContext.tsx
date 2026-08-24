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
  syncStatus: "N/A" | "Synced" | "Not synced";
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
  /** False when the caller lacks the scopes the policy view reads. */
  canViewPolicies: boolean;
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
      syncStatus: isCustomPolicy ? (syncedPolicy ? "Synced" : "Not synced") : "N/A",
      customPolicyId: syncedPolicy?.uuid,
    });
  });

  return [...rows.values()].sort((left, right) => left.name.localeCompare(right.name));
}

export function GatewayPoliciesProvider({ gatewayId, children }: { gatewayId: string; children: React.ReactNode }) {
  const [policies, setPolicies] = useState<GatewayPolicyRow[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [syncingPolicyKey, setSyncingPolicyKey] = useState<string | null>(null);

  // This view reads the gateway manifest and the org's custom policies. Without
  // both scopes the platform API returns 403, so skip the request entirely and
  // let the consumer render a permission notice instead of a load failure.
  const { hasPermission } = useAppAuth();
  const canViewPolicies =
    hasPermission(SCOPES.GATEWAY_MANIFEST_READ) &&
    hasPermission(SCOPES.GATEWAY_CUSTOM_POLICY_READ);
  const canSyncPolicies = hasPermission(SCOPES.GATEWAY_CUSTOM_POLICY_CREATE);

  const refresh = useCallback(async () => {
    if (!gatewayId) return;
    if (!canViewPolicies) {
      setPolicies([]);
      setWarnings([]);
      setIsLoading(false);
      return;
    }
    setIsLoading(true);
    setError(null);
    setWarnings([]);
    try {
      // Only the manifest is essential — custom policies and the Policy Hub
      // catalogue enrich the rows, so one of them failing degrades the view
      // with a warning rather than blocking the whole table.
      const [manifestResult, customResult, hubResult] = await Promise.allSettled([
        getGatewayPolicyManifest(gatewayId),
        getGatewayCustomPolicies(),
        getPolicies(),
      ]);
      if (manifestResult.status === "rejected") throw manifestResult.reason;
      const manifest = manifestResult.value;

      const nextWarnings: string[] = [];
      if (customResult.status === "rejected") {
        nextWarnings.push(
          "Custom policies could not be loaded, so sync status may be incomplete.",
        );
      }
      if (hubResult.status === "rejected") {
        nextWarnings.push(
          "Policy Hub details could not be loaded, so some names and descriptions may be missing.",
        );
      }
      setWarnings(nextWarnings);
      setPolicies(
        mergePolicies(
          manifest.policies || [],
          customResult.status === "fulfilled" ? customResult.value.list || [] : [],
          hubResult.status === "fulfilled" ? hubResult.value.data || [] : [],
        ),
      );
    } catch (cause) {
      setError(cause instanceof Error ? cause : new Error("Failed to load gateway policies"));
    } finally {
      setIsLoading(false);
    }
  }, [gatewayId, canViewPolicies]);

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

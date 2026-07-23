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

export type GatewayPolicyRow = {
  key: string;
  policyName: string;
  name: string;
  version: string;
  displayVersion: string;
  description: string;
  policyType: "Policy Hub" | "Custom";
  syncStatus: "N/A" | "Synced" | "Not synced";
  customPolicyId?: string;
};

type GatewayPoliciesContextValue = {
  policies: GatewayPolicyRow[];
  isLoading: boolean;
  error: Error | null;
  refresh: () => Promise<void>;
  syncPolicy: (policyName: string, version: string) => Promise<GatewayCustomPolicy>;
  syncingPolicyKey: string | null;
};

const GatewayPoliciesContext = createContext<GatewayPoliciesContextValue | null>(null);

const normalizedVersion = (version: string) => version.replace(/^v/i, "");
const displayVersion = (version: string) => normalizedVersion(version).split(".")[0];
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
      displayVersion: displayVersion(policy.version),
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
  const [syncingPolicyKey, setSyncingPolicyKey] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (!gatewayId) return;
    setIsLoading(true);
    setError(null);
    try {
      const [manifest, customResponse, hubResponse] = await Promise.all([
        getGatewayPolicyManifest(gatewayId),
        getGatewayCustomPolicies(),
        getPolicies(),
      ]);
      setPolicies(
        mergePolicies(
          manifest.policies || [],
          customResponse.list || [],
          hubResponse.data || [],
        ),
      );
    } catch (cause) {
      setError(cause instanceof Error ? cause : new Error("Failed to load gateway policies"));
    } finally {
      setIsLoading(false);
    }
  }, [gatewayId]);

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
    () => ({ policies, isLoading, error, refresh, syncPolicy, syncingPolicyKey }),
    [policies, isLoading, error, refresh, syncPolicy, syncingPolicyKey],
  );
  return <GatewayPoliciesContext.Provider value={value}>{children}</GatewayPoliciesContext.Provider>;
}

export function useGatewayPolicies() {
  const context = useContext(GatewayPoliciesContext);
  if (!context) throw new Error("useGatewayPolicies must be used within GatewayPoliciesProvider");
  return context;
}

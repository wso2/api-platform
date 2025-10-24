// src/context/DeploymentContext.tsx
import React, {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  useEffect,
  type ReactNode,
} from "react";
import {
  useDeploymentsApi,
  type DeployTargetRequest,
  type DeployRevisionResponseItem,
  type ApiDeploymentRecord,
} from "../hooks/deployments";
import { useOrganization } from "./OrganizationContext";

type DeploymentContextValue = {
  /** in-memory records per API */
  deploymentsByApi: Record<string, ApiDeploymentRecord[]>;
  loading: boolean;
  error: string | null;

  /** Fire a deployment call (returns the server response items). */
  deployApiRevision: (
    apiId: string,
    revisionId: string | number,
    targets: DeployTargetRequest[]
  ) => Promise<DeployRevisionResponseItem[]>;

  /** Optional: fetch and hydrate deployments for an API from server */
  refreshApiDeployments: (apiId: string) => Promise<ApiDeploymentRecord[]>;
};

const DeploymentContext = createContext<DeploymentContextValue | undefined>(
  undefined
);

export const DeploymentProvider = ({ children }: { children: ReactNode }) => {
  const { organization, loading: orgLoading } = useOrganization();
  const { deployRevision, fetchApiDeployments } = useDeploymentsApi();

  const [deploymentsByApi, setDeploymentsByApi] = useState<
    Record<string, ApiDeploymentRecord[]>
  >({});
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  /** Reset state if org changes / logs out */
  useEffect(() => {
    if (orgLoading) return;
    if (!organization) {
      setDeploymentsByApi({});
      setLoading(false);
      setError(null);
    }
  }, [organization, orgLoading]);

  const deployApiRevision = useCallback<
    DeploymentContextValue["deployApiRevision"]
  >(
    async (apiId, revisionId, targets) => {
      setError(null);
      setLoading(true);
      try {
        const resp = await deployRevision(apiId, revisionId, targets);

        // Merge response into in-memory list for that API
        setDeploymentsByApi((prev) => {
          const existing = prev[apiId] ?? [];
          const merged = [...existing];

          resp.forEach((item) => {
            const rec: ApiDeploymentRecord = {
              apiId,
              revisionId: String(item.revisionId ?? revisionId),
              gatewayId: item.gatewayId,
              status: item.status,
              vhost: item.vhost,
              displayOnDevportal: item.displayOnDevportal,
              deployedTime: item.deployedTime,
              successDeployedTime: item.successDeployedTime,
            };

            // replace any existing record with same revision+gateway
            const idx = merged.findIndex(
              (r) =>
                r.gatewayId === rec.gatewayId &&
                r.revisionId === rec.revisionId
            );
            if (idx >= 0) merged[idx] = rec;
            else merged.unshift(rec);
          });

          return { ...prev, [apiId]: merged };
        });

        return resp;
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Failed to deploy revision";
        setError(msg);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [deployRevision]
  );

  const refreshApiDeployments = useCallback<
    DeploymentContextValue["refreshApiDeployments"]
  >(
    async (apiId) => {
      setError(null);
      setLoading(true);
      try {
        const list = await fetchApiDeployments(apiId);
        setDeploymentsByApi((prev) => ({ ...prev, [apiId]: list }));
        return list;
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Failed to fetch deployments";
        setError(msg);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [fetchApiDeployments]
  );

  const value = useMemo<DeploymentContextValue>(
    () => ({
      deploymentsByApi,
      loading,
      error,
      deployApiRevision,
      refreshApiDeployments,
    }),
    [deploymentsByApi, loading, error, deployApiRevision, refreshApiDeployments]
  );

  return (
    <DeploymentContext.Provider value={value}>
      {children}
    </DeploymentContext.Provider>
  );
};

export const useDeployment = () => {
  const ctx = useContext(DeploymentContext);
  if (!ctx) throw new Error("useDeployment must be used within DeploymentProvider");
  return ctx;
};

// src/hooks/deployments.ts
import { useCallback } from "react";
import { getApiConfig } from "./apiConfig";

/** ----- Types ----- */

export type DeploymentStatus =
  | "CREATED"
  | "IN_PROGRESS"
  | "ACTIVE"
  | "FAILED"
  | "ROLLED_BACK"
  | "UNKNOWN";

export type DeployTargetRequest = {
  gatewayId: string;
  vhost?: string;
  displayOnDevportal?: boolean;
};

export type DeployRevisionResponseItem = {
  revisionId: string;              // stringified (e.g., "7")
  gatewayId: string;
  status: DeploymentStatus | string;
  vhost?: string;
  displayOnDevportal?: boolean;
  deployedTime?: string;           // ISO
  successDeployedTime?: string;    // ISO
};

export type DeployRevisionResponse = DeployRevisionResponseItem[];

/** Optional shape if you later want to list what’s deployed */
export type ApiDeploymentRecord = {
  apiId: string;
  revisionId: string;
  gatewayId: string;
  status: DeploymentStatus | string;
  vhost?: string;
  displayOnDevportal?: boolean;
  deployedTime?: string;
  successDeployedTime?: string;
};

/** ----- Hook ----- */

export const useDeploymentsApi = () => {
  /**
   * Deploy a specific API revision to one or more gateways.
   * @param apiId       API ID (e.g., fb78b0f8-...)
   * @param revisionId  numeric or string (server accepts "7")
   * @param targets     array of { gatewayId, vhost?, displayOnDevportal? }
   */
  const deployRevision = useCallback(
    async (
      apiId: string,
      revisionId: string | number,
      targets: DeployTargetRequest[]
    ): Promise<DeployRevisionResponse> => {
      const { token, baseUrl } = getApiConfig();
      const url = `${baseUrl}/api/v1/apis/${encodeURIComponent(
        apiId
      )}/deploy-revision?revisionId=${encodeURIComponent(String(revisionId))}`;

      const res = await fetch(url, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(targets),
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(
          `Failed to deploy revision: ${res.status} ${res.statusText} ${text}`
        );
      }

      const data = (await res.json()) as DeployRevisionResponse;
      return data;
    },
    []
  );

  /**
   * (Optional) If your backend exposes a read API for deployments,
   * wire it here. Keeping the signature ready.
   */
  const fetchApiDeployments = useCallback(
    async (apiId: string): Promise<ApiDeploymentRecord[]> => {
      const { token, baseUrl } = getApiConfig();
      const url = `${baseUrl}/api/v1/apis/${encodeURIComponent(apiId)}/deployments`;

      const res = await fetch(url, {
        method: "GET",
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(
          `Failed to fetch deployments: ${res.status} ${res.statusText} ${text}`
        );
      }

      const data = (await res.json()) as ApiDeploymentRecord[];
      return data;
    },
    []
  );

  return { deployRevision, fetchApiDeployments };
};

import { useCallback } from "react";
import { getApiConfig } from "./apiConfig";

export type GatewayType = "hybrid" | "cloud";

type GatewayApiShape = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  description?: string | null;
  host?: string | null;
  vhost?: string | null;
  createdAt: string;
  updatedAt: string;
  type?: GatewayType;
  // backend may also return these in future; keep payload-side as optional
  isCritical?: boolean;
  functionalityType?: string;
};

export type Gateway = GatewayApiShape;

export type CreateGatewayPayload = {
  name: string;
  displayName: string;
  description?: string;
  vhost?: string;
  type?: GatewayType;
  isCritical?: boolean;
  functionalityType?: string; // "regular"
};

export type DeleteGatewayPayload = { gatewayId: string };

type GatewayListResponse = {
  count: number;
  list: GatewayApiShape[];
  pagination: {
    total: number;
    offset: number;
    limit: number;
  };
};

export type RotateTokenResponse = {
  id: string;
  token: string;
  createdAt: string;
  message: string;
};

// ---- NEW: Status types ----
export type GatewayStatus = {
  id: string;
  name: string;
  isActive: boolean;
  isCritical: boolean;
};

type GatewayStatusListResponse = {
  count: number;
  list: GatewayStatus[];
  pagination: { total: number; offset: number; limit: number };
};

const normalizeGateway = (gateway: GatewayApiShape): Gateway => {
  const resolvedVhost = gateway.vhost ?? gateway.host ?? undefined;
  return {
    ...gateway,
    host: resolvedVhost,
    vhost: resolvedVhost,
  };
};

export const useGatewaysApi = () => {
  const createGateway = useCallback(
    async (payload: CreateGatewayPayload): Promise<Gateway> => {
      const { token, baseUrl } = getApiConfig();
      const response = await fetch(`${baseUrl}/api/v1/gateways`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to create gateway: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      const data: GatewayApiShape = await response.json();
      const normalized = normalizeGateway(data);
      return {
        ...normalized,
        description: payload.description ?? normalized.description,
        host: payload.vhost ?? normalized.host,
        vhost: payload.vhost ?? normalized.vhost,
        type: payload.type ?? normalized.type,
      };
    },
    []
  );

  const fetchGateways = useCallback(async (): Promise<Gateway[]> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/gateways`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(
        `Failed to fetch gateways: ${response.status} ${response.statusText} ${errorBody}`
      );
    }

    const data: GatewayListResponse = await response.json();
    return (data.list ?? []).map(normalizeGateway);
  }, []);

  const fetchGateway = useCallback(
    async (gatewayId: string): Promise<Gateway> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(`${baseUrl}/api/v1/gateways/${gatewayId}`, {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to fetch gateway ${gatewayId}: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      const data: GatewayApiShape = await response.json();
      return normalizeGateway(data);
    },
    []
  );

  const deleteGateway = useCallback(
    async (gatewayId: string): Promise<void> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(`${baseUrl}/api/v1/gateways/${gatewayId}`, {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to delete gateway ${gatewayId}: ${response.status} ${response.statusText} ${errorBody}`
        );
      }
    },
    []
  );

  // delete via payload (body) — same endpoint, accepts { gatewayId } body
  const deleteGatewayWithPayload = useCallback(
    async (payload: DeleteGatewayPayload): Promise<void> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(
        `${baseUrl}/api/v1/gateways/${payload.gatewayId}`,
        {
          method: "DELETE",
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ gatewayId: payload.gatewayId }),
        }
      );

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to delete gateway ${payload.gatewayId}: ${response.status} ${response.statusText} ${errorBody}`
        );
      }
    },
    []
  );

  const rotateGatewayToken = useCallback(
    async (gatewayId: string): Promise<RotateTokenResponse> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(
        `${baseUrl}/api/v1/gateways/${gatewayId}/tokens`,
        {
          method: "POST",
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to rotate gateway token: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      const data: RotateTokenResponse = await response.json();
      return data;
    },
    []
  );

  // ---- NEW: fetch statuses (optionally for a single gateway) ----
  const fetchGatewayStatuses = useCallback(
    async (gatewayId?: string): Promise<GatewayStatus[]> => {
      const { token, baseUrl } = getApiConfig();
      const url = new URL(`${baseUrl}/api/v1/status/gateways`);
      if (gatewayId) url.searchParams.set("gatewayId", gatewayId);

      const response = await fetch(url.toString(), {
        method: "GET",
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to fetch gateway status: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      const data: GatewayStatusListResponse = await response.json();
      return data.list ?? [];
    },
    []
  );

  return {
    createGateway,
    fetchGateways,
    fetchGateway,
    deleteGateway,
    deleteGatewayWithPayload,
    rotateGatewayToken,
    fetchGatewayStatuses,
  };
};

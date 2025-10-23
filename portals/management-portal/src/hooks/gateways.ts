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
};

export type Gateway = GatewayApiShape;

export type CreateGatewayPayload = {
  name: string;
  displayName: string;
  description?: string;
  vhost?: string;
  type?: GatewayType;
};

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

  const fetchGateway = useCallback(async (gatewayId: string): Promise<Gateway> => {
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
  }, []);

  const deleteGateway = useCallback(async (gatewayId: string): Promise<void> => {
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
  }, []);

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

  return {
    createGateway,
    fetchGateways,
    fetchGateway,
    deleteGateway,
    rotateGatewayToken,
  };
};

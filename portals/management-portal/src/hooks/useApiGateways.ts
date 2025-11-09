import { useCallback, useEffect, useRef, useState } from "react";
import { getApiConfig } from "./apiConfig";

// Backend API Model - matches raw server response exactly
export type GatewayAPIModel = {
  id: string;
  organizationId?: string;
  name: string;
  displayName?: string;
  description?: string;
  vhost?: string;
  functionalityType?: string;
  isActive?: boolean;
  createdAt?: string;
  updatedAt?: string;
};

// Frontend UI Model - extends API model with UI-specific fields
export type GatewayUIModel = GatewayAPIModel & {
  // Add UI-specific fields here if needed in the future
};

// Mapper function to convert API model to UI model
export const mapGatewayToUI = (apiModel: GatewayAPIModel): GatewayUIModel => {
  return {
    ...apiModel,
  };
};

type GatewayListResponse = {
  count?: number;
  list?: GatewayAPIModel[];
  pagination?: {
    total: number;
    offset: number;
    limit: number;
  };
};

type UseApiGatewaysReturn = {
  gateways: GatewayUIModel[];
  loading: boolean;
  error: Error | null;
  refetch: () => Promise<void>;
};

const gatewayCache = new Map<string, { data: GatewayUIModel[]; timestamp: number }>();
const CACHE_DURATION = 5 * 60 * 1000; // 5 minutes

export const useApiGateways = (apiId: string, options?: { enabled?: boolean }): UseApiGatewaysReturn => {
  const [gateways, setGateways] = useState<GatewayUIModel[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  const enabled = options?.enabled ?? true;

  const fetchGateways = useCallback(async (force = false) => {
    if (!apiId || !enabled) return;

    // Check cache first
    const cached = gatewayCache.get(apiId);
    if (!force && cached && Date.now() - cached.timestamp < CACHE_DURATION) {
      setGateways(cached.data);
      setLoading(false);
      setError(null);
      return;
    }

    // Cancel previous request
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }

    const abortController = new AbortController();
    abortControllerRef.current = abortController;

    setLoading(true);
    setError(null);

    try {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(
        `${baseUrl}/api/v1/apis/${encodeURIComponent(apiId)}/gateways`,
        {
          method: "GET",
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          signal: abortController.signal,
        }
      );

      if (!response.ok) {
        if (response.status === 404) {
          const emptyResult: GatewayUIModel[] = [];
          setGateways(emptyResult);
          gatewayCache.set(apiId, { data: emptyResult, timestamp: Date.now() });
          return;
        }

        const errorBody = await response.text();
        throw new Error(
          `Failed to fetch gateways for API ${apiId}: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

  const data = (await response.json()) as GatewayListResponse | GatewayAPIModel[];

      let gatewayList: GatewayAPIModel[];
      if (Array.isArray(data)) {
        gatewayList = data;
      } else if (data?.list && Array.isArray(data.list)) {
        gatewayList = data.list;
      } else {
        gatewayList = [];
      }

      // Map API models to UI models
      const uiGateways = gatewayList.map(mapGatewayToUI);
      setGateways(uiGateways);
      gatewayCache.set(apiId, { data: uiGateways, timestamp: Date.now() });

    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') {
        // Request was cancelled, don't set error state
        return;
      }

      const error = err instanceof Error ? err : new Error('Unknown error occurred');
  setError(error);
    } finally {
      setLoading(false);
    }
  }, [apiId, enabled]);

  const refetch = useCallback(async () => {
    await fetchGateways(true);
  }, [fetchGateways]);

  useEffect(() => {
    fetchGateways();

    // Cleanup function to abort any ongoing requests
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, [fetchGateways]);

  return {
    gateways,
    loading,
    error,
    refetch,
  };
};
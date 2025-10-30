import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  useGatewaysApi,
  type CreateGatewayPayload,
  type Gateway,
  type RotateTokenResponse,
  type DeleteGatewayPayload,
  type GatewayStatus, // NEW
} from "../hooks/gateways";
import { useOrganization } from "./OrganizationContext";

const mergeGateway = (
  incoming: Gateway,
  existing?: Gateway,
  overrides: Partial<Gateway> = {}
): Gateway => {
  const merged = {
    ...existing,
    ...incoming,
    ...overrides,
  };

  const resolvedDescription =
    overrides.description ??
    incoming.description ??
    existing?.description ??
    undefined;

  const resolvedVhost =
    overrides.vhost ??
    overrides.host ??
    incoming.vhost ??
    incoming.host ??
    existing?.vhost ??
    existing?.host ??
    undefined;

  const resolvedType =
    overrides.type ?? incoming.type ?? existing?.type ?? "hybrid";

  return {
    ...merged,
    description: resolvedDescription,
    host: resolvedVhost,
    vhost: resolvedVhost,
    type: resolvedType,
  };
};

type GatewayContextValue = {
  gateways: Gateway[];
  loading: boolean;
  error: string | null;
  refreshGateways: () => Promise<Gateway[]>;
  createGateway: (payload: CreateGatewayPayload) => Promise<Gateway>;
  updateGateway: (
    gatewayId: string,
    updates: Partial<Gateway>
  ) => Gateway | undefined;
  deleteGateway: (gatewayId: string) => Promise<void>;
  deleteGatewayByPayload: (payload: DeleteGatewayPayload) => Promise<void>;
  fetchGatewayById: (gatewayId: string) => Promise<Gateway>;
  rotateGatewayToken: (gatewayId: string) => Promise<RotateTokenResponse>;
  gatewayTokens: Record<string, RotateTokenResponse>;

  // NEW: statuses API exposure
  gatewayStatuses: Record<string, GatewayStatus>;
  refreshGatewayStatuses: (gatewayId?: string) => Promise<Record<string, GatewayStatus>>;
  getGatewayStatus: (gatewayId: string) => GatewayStatus | undefined;
};

const GatewayContext = createContext<GatewayContextValue | undefined>(undefined);

type GatewayProviderProps = {
  children: ReactNode;
};

export const GatewayProvider = ({ children }: GatewayProviderProps) => {
  const {
    createGateway: createGatewayRequest,
    fetchGateways,
    fetchGateway,
    deleteGateway: deleteGatewayRequest,
    deleteGatewayWithPayload,
    rotateGatewayToken: rotateGatewayTokenRequest,
    fetchGatewayStatuses, // NEW
  } = useGatewaysApi();

  const { organization, loading: organizationLoading } = useOrganization();

  const TOKEN_STORAGE_KEY = "gatewayTokens";

  const loadStoredTokens = useCallback((): Record<string, RotateTokenResponse> => {
    if (typeof window === "undefined") {
      return {};
    }
    try {
      const raw = window.localStorage.getItem(TOKEN_STORAGE_KEY);
      if (!raw) {
        return {};
      }
      const parsed = JSON.parse(raw) as Record<string, RotateTokenResponse>;
      if (parsed && typeof parsed === "object") {
        return parsed;
      }
    } catch {
      /* ignore malformed storage */
    }
    return {};
  }, []);

  const persistTokens = useCallback((tokens: Record<string, RotateTokenResponse>) => {
    if (typeof window === "undefined") {
      return;
    }
    try {
      window.localStorage.setItem(TOKEN_STORAGE_KEY, JSON.stringify(tokens));
    } catch {
      /* ignore storage errors */
    }
  }, []);

  const [gateways, setGateways] = useState<Gateway[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [gatewayTokens, setGatewayTokens] = useState<
    Record<string, RotateTokenResponse>
  >(loadStoredTokens);

  // NEW: status state
  const [gatewayStatuses, setGatewayStatuses] = useState<Record<string, GatewayStatus>>({});

  const refreshGateways = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const result = await fetchGateways();
      let normalized: Gateway[] = [];
      setGateways((prev) => {
        normalized = result.map((gateway) => {
          const existing = prev.find((item) => item.id === gateway.id);
          return mergeGateway(gateway, existing);
        });
        return normalized;
      });
      setGatewayTokens((prev) => {
        const next: Record<string, RotateTokenResponse> = {};
        normalized.forEach((gateway) => {
          const token = prev[gateway.id];
          if (token) {
            next[gateway.id] = token;
          }
        });
        persistTokens(next);
        return next;
      });
      return normalized;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Unknown error occurred";
      setError(message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [fetchGateways, persistTokens]);

  // NEW: refresh statuses (all or single)
  const refreshGatewayStatuses = useCallback(
    async (gatewayId?: string) => {
      try {
        const list = await fetchGatewayStatuses(gatewayId);
        setGatewayStatuses((prev) => {
          const next = { ...prev };
          for (const s of list) next[s.id] = s;
          return next;
        });
        return list.reduce<Record<string, GatewayStatus>>((acc, s) => {
          acc[s.id] = s;
          return acc;
        }, {});
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to fetch gateway status";
        setError(message);
        throw err;
      }
    },
    [fetchGatewayStatuses]
  );

  const createGateway = useCallback(
    async (payload: CreateGatewayPayload) => {
      setError(null);

      try {
        const gateway = await createGatewayRequest(payload);
        const normalized = mergeGateway(gateway, undefined, {
          description: payload.description ?? gateway.description ?? undefined,
          vhost: payload.vhost ?? gateway.vhost ?? gateway.host ?? undefined,
          type: payload.type ?? gateway.type ?? "hybrid",
        });

        setGateways((prev) => {
          const next = prev.filter((item) => item.id !== normalized.id);
          return [normalized, ...next];
        });

        // rotate token in background
        rotateGatewayTokenRequest(gateway.id)
          .then((tokenResponse) => {
            setGatewayTokens((prev) => {
              const next = { ...prev, [gateway.id]: tokenResponse };
              persistTokens(next);
              return next;
            });
          })
          .catch((err) => {
            const message =
              err instanceof Error ? err.message : "Failed to rotate token";
            setError(message);
          });

        // NEW: refresh statuses after create
        refreshGatewayStatuses().catch(() => {});

        return normalized;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      }
    },
    [createGatewayRequest, persistTokens, rotateGatewayTokenRequest, refreshGatewayStatuses]
  );

  const updateGateway = useCallback(
    (gatewayId: string, updates: Partial<Gateway>) => {
      let updatedGateway: Gateway | undefined;
      setGateways((prev) =>
        prev.map((gateway) => {
          if (gateway.id === gatewayId) {
            const resolvedVhost =
              updates.vhost ??
              updates.host ??
              gateway.vhost ??
              gateway.host ??
              undefined;
            updatedGateway = {
              ...gateway,
              ...updates,
              host: resolvedVhost,
              vhost: resolvedVhost,
            };
            return updatedGateway;
          }
          return gateway;
        })
      );
      // Optional: refresh status for that single gateway if relevant fields changed
      // refreshGatewayStatuses(gatewayId).catch(() => {});
      return updatedGateway;
    },
    []
  );

  const deleteGateway = useCallback(
    async (gatewayId: string) => {
      setError(null);

      try {
        await deleteGatewayRequest(gatewayId);
        setGateways((prev) =>
          prev.filter((gateway) => gateway.id !== gatewayId)
        );
        setGatewayTokens((prev) => {
          const next = { ...prev };
          if (gatewayId in next) {
            delete next[gatewayId];
            persistTokens(next);
          }
          return next;
        });

        // NEW: refresh statuses after delete
        refreshGatewayStatuses().catch(() => {});
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      }
    },
    [deleteGatewayRequest, persistTokens, refreshGatewayStatuses]
  );

  const deleteGatewayByPayload = useCallback(
    async (payload: DeleteGatewayPayload) => {
      setError(null);

      try {
        await deleteGatewayWithPayload(payload);
        const id = payload.gatewayId;

        setGateways((prev) => prev.filter((gateway) => gateway.id !== id));

        setGatewayTokens((prev) => {
          const next = { ...prev };
          if (id in next) {
            delete next[id];
            persistTokens(next);
          }
          return next;
        });

        // NEW: refresh statuses after delete
        refreshGatewayStatuses().catch(() => {});
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      }
    },
    [deleteGatewayWithPayload, persistTokens, refreshGatewayStatuses]
  );

  const fetchGatewayById = useCallback(
    async (gatewayId: string) => {
      setError(null);

      try {
        const gateway = await fetchGateway(gatewayId);
        let normalized: Gateway | undefined;
        setGateways((prev) => {
          const existing = prev.find((item) => item.id === gateway.id);
          normalized = mergeGateway(gateway, existing);
          const others = prev.filter((item) => item.id !== gateway.id);
          return normalized ? [normalized, ...others] : prev;
        });

        if (!normalized) {
          normalized = mergeGateway(gateway);
        }

        // NEW: also refresh status for the fetched gateway
        refreshGatewayStatuses(gatewayId).catch(() => {});

        return normalized;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      }
    },
    [fetchGateway, refreshGatewayStatuses]
  );

  const rotateGatewayToken = useCallback(
    async (gatewayId: string) => {
      setError(null);

      try {
        const tokenResponse = await rotateGatewayTokenRequest(gatewayId);

        setGatewayTokens((prev) => {
          const next = { ...prev, [gatewayId]: tokenResponse };
          persistTokens(next);
          return next;
        });

        return tokenResponse;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to rotate token";
        setError(message);
        throw err;
      }
    },
    [rotateGatewayTokenRequest, persistTokens]
  );

  // When org changes
  useEffect(() => {
    if (organizationLoading) {
      return;
    }

    if (!organization) {
      setGateways([]);
      setGatewayTokens((prev) => {
        if (Object.keys(prev).length > 0) {
          persistTokens({});
        }
        return {};
      });
      setGatewayStatuses({});
      setLoading(false);
      return;
    }

    refreshGateways().catch(() => {
      /* errors captured in state */
    });
  }, [organization, organizationLoading, refreshGateways, persistTokens]);

  // NEW: Poll statuses every 15s while org is present
  useEffect(() => {
    if (organizationLoading) return;
    if (!organization) {
      setGatewayStatuses({});
      return;
    }

    // initial fetch
    refreshGatewayStatuses().catch(() => {});

    const interval = setInterval(() => {
      refreshGatewayStatuses().catch(() => {});
    }, 15000);

    return () => clearInterval(interval);
  }, [organization, organizationLoading, refreshGatewayStatuses]);

  const getGatewayStatus = useCallback(
    (gatewayId: string) => gatewayStatuses[gatewayId],
    [gatewayStatuses]
  );

  const value = useMemo<GatewayContextValue>(
    () => ({
      gateways,
      loading,
      error,
      refreshGateways,
      createGateway,
      updateGateway,
      deleteGateway,
      deleteGatewayByPayload,
      fetchGatewayById,
      rotateGatewayToken,
      gatewayTokens,

      // NEW
      gatewayStatuses,
      refreshGatewayStatuses,
      getGatewayStatus,
    }),
    [
      gateways,
      loading,
      error,
      refreshGateways,
      createGateway,
      updateGateway,
      deleteGateway,
      deleteGatewayByPayload,
      fetchGatewayById,
      rotateGatewayToken,
      gatewayTokens,

      // NEW
      gatewayStatuses,
      refreshGatewayStatuses,
      getGatewayStatus,
    ]
  );

  return (
    <GatewayContext.Provider value={value}>{children}</GatewayContext.Provider>
  );
};

export const useGateways = () => {
  const context = useContext(GatewayContext);

  if (!context) {
    throw new Error("useGateways must be used within a GatewayProvider");
  }

  return context;
};

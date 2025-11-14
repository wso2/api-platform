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
  useDevPortalsApi,
  type CreatePortalData,
  type Portal,
  type UpdatePortalPayload,
} from "../hooks/devportals";
import { useOrganization } from "./OrganizationContext";

type DevPortalContextValue = {
  devportals: Portal[];
  loading: boolean;
  error: string | null;
  refreshDevPortals: () => Promise<Portal[]>;
  createDevPortal: (payload: CreatePortalData) => Promise<Portal>;
  updateDevPortal: (
    portalId: string,
    updates: UpdatePortalPayload
  ) => Promise<Portal>;
  deleteDevPortal: (portalId: string) => Promise<void>;
  fetchDevPortalById: (portalId: string) => Promise<Portal>;
  activateDevPortal: (portalId: string) => Promise<void>;
};

export const DevPortalContext = createContext<DevPortalContextValue | undefined>(undefined);

type DevPortalProviderProps = {
  children: ReactNode;
};

export const DevPortalProvider = ({ children }: DevPortalProviderProps) => {
  const {
    createDevPortal: createDevPortalRequest,
    fetchDevPortals,
    fetchDevPortal,
    updateDevPortal: updateDevPortalRequest,
    deleteDevPortal: deleteDevPortalRequest,
    activateDevPortal: activateDevPortalRequest,
  } = useDevPortalsApi();

  const { organization, loading: organizationLoading } = useOrganization();

  const [devportals, setDevportals] = useState<Portal[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refreshDevPortals = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const result = await fetchDevPortals();
      setDevportals(result);
      return result;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Unknown error occurred";
      setError(message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [fetchDevPortals]);

  const createDevPortal = useCallback(
    async (payload: CreatePortalData) => {
      setError(null);

      try {
        const portal = await createDevPortalRequest(payload);
        setDevportals((prev) => [portal, ...prev]);
        return portal;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      }
    },
    [createDevPortalRequest]
  );

  const updateDevPortal = useCallback(
    async (portalId: string, updates: UpdatePortalPayload) => {
      setError(null);

      try {
        const portal = await updateDevPortalRequest(portalId, updates as any);
        setDevportals((prev) =>
          prev.map((p) => (p.uuid === portalId ? portal : p))
        );
        return portal;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      }
    },
    [updateDevPortalRequest]
  );

  const deleteDevPortal = useCallback(
    async (portalId: string) => {
      setError(null);

      try {
        await deleteDevPortalRequest(portalId);
        setDevportals((prev) =>
          prev.filter((portal) => portal.uuid !== portalId)
        );
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      }
    },
    [deleteDevPortalRequest]
  );

  const fetchDevPortalById = useCallback(
    async (portalId: string) => {
      setError(null);

      try {
        const portal = await fetchDevPortal(portalId);
        let normalized: Portal | undefined;
        setDevportals((prev) => {
          const existing = prev.find((item) => item.uuid === portal.uuid);
          normalized = existing ? { ...existing, ...portal } : portal;
          const others = prev.filter((item) => item.uuid !== portal.uuid);
          return normalized ? [normalized, ...others] : prev;
        });

        if (!normalized) {
          normalized = portal;
        }

        return normalized;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      }
    },
    [fetchDevPortal]
  );

  const activateDevPortal = useCallback(
    async (portalId: string) => {
      setError(null);

      try {
        await activateDevPortalRequest(portalId);
        setDevportals((prev) =>
          prev.map((portal) =>
            portal.uuid === portalId
              ? { ...portal, isActive: true }
              : portal
          )
        );
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to activate devportal";
        setError(message);
        throw err;
      }
    },
    [activateDevPortalRequest]
  );

  // When org changes
  useEffect(() => {
    if (organizationLoading) {
      return;
    }

    if (!organization) {
      setDevportals([]);
      setLoading(false);
      return;
    }

    refreshDevPortals().catch(() => {
      /* errors captured in state */
    });
  }, [organization, organizationLoading, refreshDevPortals]);

  const value = useMemo<DevPortalContextValue>(
    () => ({
      devportals,
      loading,
      error,
      refreshDevPortals,
      createDevPortal,
      updateDevPortal,
      deleteDevPortal,
      fetchDevPortalById,
      activateDevPortal,
    }),
    [
      devportals,
      loading,
      error,
      refreshDevPortals,
      createDevPortal,
      updateDevPortal,
      deleteDevPortal,
      fetchDevPortalById,
      activateDevPortal,
    ]
  );

  return (
    <DevPortalContext.Provider value={value}>
      {children}
    </DevPortalContext.Provider>
  );
};

export const useDevPortals = () => {
  const context = useContext(DevPortalContext);

  if (!context) {
    throw new Error("useDevPortals must be used within a DevPortalProvider");
  }

  return context;
};

export type { DevPortalContextValue };
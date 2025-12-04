import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  useRef,
  type ReactNode,
} from 'react';

import {
  useDevPortalsApi,
  type Portal,
  type CreatePortalPayload,
  type UpdatePortalPayload,
} from '../hooks/devportals';
import { useOrganization } from './OrganizationContext';
import { useNotifications } from './NotificationContext';

/* -------------------------------------------------------------------------- */
/*                              Type Definitions                              */
/* -------------------------------------------------------------------------- */

export interface DevPortalContextValue {
  devportals: Portal[];
  loading: boolean;

  refreshDevPortals: () => Promise<Portal[]>;
  createDevPortal: (payload: CreatePortalPayload) => Promise<Portal>;
  updateDevPortal: (
    uuid: string,
    updates: UpdatePortalPayload
  ) => Promise<Portal>;
  deleteDevPortal: (uuid: string) => Promise<void>;
  fetchDevPortalById: (uuid: string) => Promise<Portal>;
  activateDevPortal: (uuid: string) => Promise<void>;
}

/* -------------------------------------------------------------------------- */
/*                                 Context Init                               */
/* -------------------------------------------------------------------------- */

const DevPortalContext = createContext<DevPortalContextValue | undefined>(
  undefined
);

export const DevPortalProvider = ({ children }: { children: ReactNode }) => {
  const {
    fetchDevPortals,
    fetchDevPortal,
    createDevPortal: createRequest,
    updateDevPortal: updateRequest,
    deleteDevPortal: deleteRequest,
    activateDevPortal: activateRequest,
  } = useDevPortalsApi();

  const { organization, loading: orgLoading } = useOrganization();
  const { showNotification } = useNotifications();

  const [devportals, setDevportals] = useState<Portal[]>([]);
  const [loading, setLoading] = useState(true);
  const lastFetchedOrgRef = useRef<string | null>(null);

  /* --------------------------- Error Helper --------------------------- */

  const handleError = useCallback(
    (err: unknown, fallback: string) => {
      const msg = err instanceof Error ? err.message : fallback;
      showNotification(msg, 'error');
    },
    [showNotification]
  );

  /* --------------------------- Core Actions --------------------------- */

  const refreshDevPortals = useCallback(async () => {
    setLoading(true);

    try {
      const result = await fetchDevPortals();
      setDevportals(result);
      return result;
    } catch (err) {
      handleError(err, 'Failed to fetch devportals');
      return []; // Return empty array on error
    } finally {
      setLoading(false);
    }
  }, [fetchDevPortals, handleError]);

  const createDevPortal = useCallback(
    async (payload: CreatePortalPayload) => {
      try {
        const portal = await createRequest(payload);
        setDevportals((prev) => [portal, ...prev]);
        return portal;
      } catch (err) {
        handleError(err, 'Failed to create devportal');
        throw err;
      }
    },
    [createRequest, handleError]
  );

  const updateDevPortal = useCallback(
    async (uuid: string, updates: UpdatePortalPayload) => {
      try {
        const portal = await updateRequest(uuid, updates);
        setDevportals((prev) =>
          prev.map((p) => (p.uuid === uuid ? portal : p))
        );
        return portal;
      } catch (err) {
        handleError(err, 'Failed to update devportal');
        throw err;
      }
    },
    [updateRequest, handleError]
  );

  const deleteDevPortal = useCallback(
    async (uuid: string) => {
      try {
        await deleteRequest(uuid);
        setDevportals((prev) => prev.filter((portal) => portal.uuid !== uuid));
      } catch (err) {
        handleError(err, 'Failed to delete devportal');
        throw err;
      }
    },
    [deleteRequest, handleError]
  );

  const fetchDevPortalById = useCallback(
    async (uuid: string) => {
      try {
        const portal = await fetchDevPortal(uuid);
        setDevportals((prev) => {
          const existing = prev.find((p) => p.uuid === portal.uuid);
          return existing
            ? prev.map((p) => (p.uuid === uuid ? { ...p, ...portal } : p))
            : [portal, ...prev];
        });
        return portal;
      } catch (err) {
        handleError(err, 'Failed to fetch portal details');
        throw err;
      }
    },
    [fetchDevPortal, handleError]
  );

  const activateDevPortal = useCallback(
    async (uuid: string) => {
      try {
        await activateRequest(uuid);
        setDevportals((prev) =>
          prev.map((portal) =>
            portal.uuid === uuid ? { ...portal, isEnabled: true } : portal
          )
        );
      } catch (err) {
        handleError(err, 'Failed to activate devportal');
        throw err;
      }
    },
    [activateRequest, handleError]
  );

  /* ------------------------- Handle Org Change ------------------------ */

  useEffect(() => {
    if (orgLoading) return;

    if (!organization) {
      setDevportals([]);
      setLoading(false);
      lastFetchedOrgRef.current = null;
      return;
    }

    if (lastFetchedOrgRef.current === organization.id) return; // Already fetched

    lastFetchedOrgRef.current = organization.id;
    refreshDevPortals();
  }, [organization, orgLoading, refreshDevPortals]);

  /* ---------------------------- Context Value ---------------------------- */

  const value = useMemo<DevPortalContextValue>(
    () => ({
      devportals,
      loading,
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

/* -------------------------------------------------------------------------- */
/*                                    Hook                                    */
/* -------------------------------------------------------------------------- */

export const useDevPortals = () => {
  const context = useContext(DevPortalContext);
  if (!context) {
    throw new Error('useDevPortals must be used within a DevPortalProvider');
  }
  return context;
};

import {
  createContext,
  useCallback,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { useDevPortalsApi, type DevPortalUIModel, mapDevPortalToUI } from "../hooks/devportals";
import { useOrganization } from "./OrganizationContext";

type DevPortalContextValue = {
  devportals: DevPortalUIModel[];
  loading: boolean;
  error: string | null;
  refreshDevPortals: () => Promise<DevPortalUIModel[]>;
  activateDevPortal: (uuid: string) => Promise<void>;
  createDevPortal: (portalData: {
    name: string;
    identifier: string;
    apiUrl: string;
    hostname: string;
    apiKey: string;
    headerKeyName: string;
    description: string;
  }) => Promise<DevPortalUIModel>;
  updateDevPortal: (uuid: string, portalData: {
    name: string;
    identifier: string;
    apiUrl: string;
    hostname: string;
    apiKey: string;
    headerKeyName: string;
    description: string;
  }) => Promise<DevPortalUIModel>;
};

const DevPortalContext = createContext<DevPortalContextValue | undefined>(undefined);

export type { DevPortalContextValue };
export { DevPortalContext };

type DevPortalProviderProps = {
  children: ReactNode;
};

export const DevPortalProvider = ({ children }: DevPortalProviderProps) => {
  const { fetchDevPortals, activateDevPortal: activateRequest, createDevPortal: createRequest, updateDevPortal: updateRequest } = useDevPortalsApi();
  const { organization, loading: organizationLoading } = useOrganization();

  const [devportals, setDevportals] = useState<DevPortalUIModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refreshDevPortals = useCallback(async (): Promise<DevPortalUIModel[]> => {
    if (!organization?.id) {
      setDevportals([]);
      return [];
    }

    try {
      setLoading(true);
      setError(null);
      const data = await fetchDevPortals();
      setDevportals(data);
      return data;
    } catch (err) {
      setError('An error occurred while loading developer portals.');
      throw err;
    } finally {
      setLoading(false);
    }
  }, [fetchDevPortals, organization?.id]);

  const activateDevPortal = useCallback(async (uuid: string): Promise<void> => {
    await activateRequest(uuid);
    // Refresh the list after activation (non-blocking)
    try {
      await refreshDevPortals();
    } catch (err) {
      console.error('Failed to refresh portal list after activation:', err);
      // Don't throw - activation was successful
    }
  }, [activateRequest, refreshDevPortals]);

  const createDevPortal = useCallback(async (portalData: {
    name: string;
    identifier: string;
    apiUrl: string;
    hostname: string;
    apiKey: string;
    headerKeyName: string;
    description: string;
  }): Promise<DevPortalUIModel> => {
    const createdPortal = await createRequest(portalData);
    // Refresh the list after creation (non-blocking)
    try {
      await refreshDevPortals();
    } catch (err) {
      console.error('Failed to refresh portal list after creation:', err);
      // Don't throw - creation was successful
    }
    return mapDevPortalToUI(createdPortal);
  }, [createRequest, refreshDevPortals]);

  const updateDevPortal = useCallback(async (uuid: string, portalData: {
    name: string;
    identifier: string;
    apiUrl: string;
    hostname: string;
    apiKey: string;
    headerKeyName: string;
    description: string;
  }): Promise<DevPortalUIModel> => {
    const updatedPortal = await updateRequest(uuid, portalData);
    // Refresh the list after update (non-blocking)
    try {
      await refreshDevPortals();
    } catch (err) {
      console.error('Failed to refresh portal list after update:', err);
      // Don't throw - update was successful
    }
    return mapDevPortalToUI(updatedPortal);
  }, [updateRequest, refreshDevPortals]);

  useEffect(() => {
    if (!organizationLoading && organization?.id) {
      refreshDevPortals();
    } else if (!organization?.id) {
      setDevportals([]);
      setLoading(false);
    }
  }, [organization?.id, organizationLoading, refreshDevPortals]);

  const value: DevPortalContextValue = {
    devportals,
    loading,
    error,
    refreshDevPortals,
    activateDevPortal,
    createDevPortal,
    updateDevPortal,
  };

  return (
    <DevPortalContext.Provider value={value}>
      {children}
    </DevPortalContext.Provider>
  );
};
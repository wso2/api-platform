import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  useApisApi,
  type ApiSummary,
  type CreateApiPayload,
  type ApiGatewaySummary,
} from "../hooks/apis";
import { useProjects } from "./ProjectContext";

type ApiContextValue = {
  apis: ApiSummary[];
  loading: boolean;
  error: string | null;
  refreshApis: (projectId?: string) => Promise<ApiSummary[]>;
  createApi: (payload: CreateApiPayload) => Promise<ApiSummary>;
  fetchApiById: (apiId: string) => Promise<ApiSummary>;
  deleteApi: (apiId: string) => Promise<void>;
  /** Gateways bound to an API id */
  fetchGatewaysForApi: (apiId: string) => Promise<ApiGatewaySummary[]>;
};

const ApiContext = createContext<ApiContextValue | undefined>(undefined);

type ApiProviderProps = {
  children: ReactNode;
};

export const ApiProvider = ({ children }: ApiProviderProps) => {
  const {
    fetchProjectApis,
    fetchApi,
    createApi: createApiRequest,
    deleteApi: deleteApiRequest,
    fetchApiGateways,
  } = useApisApi();
  const { selectedProject } = useProjects();

  const [apisByProject, setApisByProject] = useState<
    Record<string, ApiSummary[]>
  >({});
  const [gatewaysByApi, setGatewaysByApi] = useState<
    Record<string, ApiGatewaySummary[]>
  >({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const lastFetchedProjectRef = useRef<string | null>(null);

  const currentProjectId = selectedProject?.id ?? null;

  const apis = useMemo(() => {
    if (!currentProjectId) return [];
    return apisByProject[currentProjectId] ?? [];
  }, [apisByProject, currentProjectId]);

  const refreshApis = useCallback(
    async (projectIdParam?: string) => {
      const projectId = projectIdParam ?? currentProjectId;
      if (!projectId) return [];

      setLoading(true);
      setError(null);

      try {
        const result = await fetchProjectApis(projectId);
        setApisByProject((prev) => ({
          ...prev,
          [projectId]: result,
        }));
        lastFetchedProjectRef.current = projectId;
        return result;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to fetch APIs";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [currentProjectId, fetchProjectApis]
  );

  const createApi = useCallback(
    async (payload: CreateApiPayload) => {
      setLoading(true);
      setError(null);

      try {
        const api = await createApiRequest(payload);
        const projectId = api.projectId ?? payload.projectId;

        if (projectId) {
          setApisByProject((prev) => {
            const existing = prev[projectId] ?? [];
            const next = [
              api,
              ...existing.filter((item) => item.id !== api.id),
            ];
            return { ...prev, [projectId]: next };
          });
        }

        return api;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to create API";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [createApiRequest]
  );

  const fetchApiById = useCallback(
    async (apiId: string) => {
      setLoading(true);
      setError(null);

      try {
        const api = await fetchApi(apiId);
        const projectId = api.projectId;
        if (projectId) {
          setApisByProject((prev) => {
            const existing = prev[projectId] ?? [];
            const index = existing.findIndex((item) => item.id === api.id);
            const next = [...existing];
            if (index === -1) next.unshift(api);
            else next[index] = api;
            return { ...prev, [projectId]: next };
          });
        }
        return api;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to fetch API";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [fetchApi]
  );

  const deleteApi = useCallback(
    async (apiId: string) => {
      setLoading(true);
      setError(null);

      try {
        await deleteApiRequest(apiId);
        setApisByProject((prev) => {
          const entries = Object.entries(prev);
          let changed = false;
          const next: Record<string, ApiSummary[]> = {};

          for (const [projectId, items] of entries) {
            const filtered = items.filter((item) => item.id !== apiId);
            next[projectId] = filtered;
            if (filtered.length !== items.length) changed = true;
          }

          return changed ? next : prev;
        });
        // also clear any cached gateways for this API
        setGatewaysByApi((prev) => {
          if (prev[apiId]) {
            const { [apiId]: _removed, ...rest } = prev;
            return rest;
          }
          return prev;
        });
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to delete API";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [deleteApiRequest]
  );

  /** Fetch + cache gateways for an API */
  const fetchGatewaysForApi = useCallback(
    async (apiId: string) => {
      setLoading(true);
      setError(null);
      try {
        const list = await fetchApiGateways(apiId);
        setGatewaysByApi((prev) => ({ ...prev, [apiId]: list }));
        return list;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to fetch API gateways";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [fetchApiGateways]
  );

  useEffect(() => {
    if (!currentProjectId) return;
    if (lastFetchedProjectRef.current === currentProjectId) return;

    refreshApis(currentProjectId).catch(() => {
      /* error captured in state */
    });
  }, [currentProjectId, refreshApis]);

  const value = useMemo<ApiContextValue>(
    () => ({
      apis,
      loading,
      error,
      refreshApis,
      createApi,
      fetchApiById,
      deleteApi,
      fetchGatewaysForApi,
    }),
    [
      apis,
      loading,
      error,
      refreshApis,
      createApi,
      fetchApiById,
      deleteApi,
      fetchGatewaysForApi,
    ]
  );

  return <ApiContext.Provider value={value}>{children}</ApiContext.Provider>;
};

export const useApisContext = () => {
  const context = useContext(ApiContext);
  if (!context) {
    throw new Error("useApisContext must be used within an ApiProvider");
  }
  return context;
};

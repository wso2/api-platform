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
  type ImportOpenApiRequest,
} from "../hooks/apis";
import { useProjects } from "./ProjectContext";
import { slugify } from "../utils/slug";

type ApiContextValue = {
  apis: ApiSummary[];
  currentApi: ApiSummary | null;
  currentApiSlug: string | null;
  loading: boolean;
  error: string | null;
  refreshApis: (projectId?: string) => Promise<ApiSummary[]>;
  createApi: (payload: CreateApiPayload) => Promise<ApiSummary>;
  fetchApiById: (apiId: string) => Promise<ApiSummary>;
  deleteApi: (apiId: string) => Promise<void>;
  selectApi: (api: ApiSummary | null, options?: { slug?: string }) => void;
  /** Gateways bound to an API id */
  fetchGatewaysForApi: (apiId: string) => Promise<ApiGatewaySummary[]>;
  importOpenApi: (payload: ImportOpenApiRequest, opts?: { signal?: AbortSignal }) => Promise<ApiSummary>;
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
    importOpenApi,
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
  const [currentApi, setCurrentApi] = useState<ApiSummary | null>(null);
  const [currentApiSlug, setCurrentApiSlug] = useState<string | null>(null);
  const currentApiIdRef = useRef<string | null>(null);
  const lastFetchedProjectRef = useRef<string | null>(null);

  const currentProjectId = selectedProject?.id ?? null;

  const apis = useMemo(() => {
    if (!currentProjectId) return [];
    return apisByProject[currentProjectId] ?? [];
  }, [apisByProject, currentProjectId]);

  const selectApi = useCallback(
    (api: ApiSummary | null, options?: { slug?: string }) => {
      setCurrentApi(api);
      if (api) {
        currentApiIdRef.current = api.id;
        const nextSlug = options?.slug ?? slugify(api.name);
        setCurrentApiSlug(nextSlug);
      } else {
        currentApiIdRef.current = null;
        setCurrentApiSlug(null);
      }
    },
    []
  );

  useEffect(() => {
    if (!currentProjectId) {
      if (currentApi) {
        selectApi(null);
      }
      return;
    }
    if (currentApi && currentApi.projectId !== currentProjectId) {
      selectApi(null);
    }
  }, [currentApi, currentProjectId, selectApi]);

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
        const currentId = currentApiIdRef.current;
        if (currentId) {
          const updated = result.find((item) => item.id === currentId) ?? null;
          if (updated) {
            setCurrentApi(updated);
            setCurrentApiSlug(slugify(updated.name));
          } else {
            selectApi(null);
          }
        }
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
    [currentProjectId, fetchProjectApis, selectApi]
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
        if (currentApiIdRef.current === api.id) {
          setCurrentApi(api);
          setCurrentApiSlug(slugify(api.name));
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
        if (currentApiIdRef.current === apiId) {
          selectApi(null);
        }
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to delete API";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [deleteApiRequest, selectApi]
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
      currentApi,
      currentApiSlug,
      loading,
      error,
      refreshApis,
      createApi,
      fetchApiById,
      deleteApi,
      selectApi,
      fetchGatewaysForApi,
      importOpenApi,
    }),
    [
      apis,
      currentApi,
      currentApiSlug,
      loading,
      error,
      refreshApis,
      createApi,
      fetchApiById,
      deleteApi,
      selectApi,
      fetchGatewaysForApi,
      importOpenApi,
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

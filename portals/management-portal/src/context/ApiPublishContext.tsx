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
  useApiPublishApi,
  type UnpublishResponse,
  type ApiPublicationWithPortal,
  type ApiPublishPayload,
  type PublishResponse,
} from "../hooks/apiPublish";
import { useOrganization } from "./OrganizationContext";

type ApiPublishContextValue = {
  publishedApis: Record<string, ApiPublicationWithPortal[]>; // keyed by apiId
  loading: boolean;
  error: string | null;
  refreshPublishedApis: (apiId: string) => Promise<ApiPublicationWithPortal[]>;
  publishApiToDevPortal: (apiId: string, payload: ApiPublishPayload) => Promise<PublishResult | void>;
  unpublishApiFromDevPortal: (
    apiId: string,
    devPortalId: string
  ) => Promise<UnpublishResponse>;
  getPublishStatus: (apiId: string, devPortalId: string) => ApiPublicationWithPortal | undefined;
};

// The publish hook returns a PublishResponse type; expose that as the context result
type PublishResult = PublishResponse;

const ApiPublishContext = createContext<ApiPublishContextValue | undefined>(undefined);

type ApiPublishProviderProps = {
  children: ReactNode;
};

export const ApiPublishProvider = ({ children }: ApiPublishProviderProps) => {
  const { organization, loading: organizationLoading } = useOrganization();

  const { fetchPublications, publishApiToDevPortal: publishRequest, unpublishApiFromDevPortal: unpublishRequest } =
    useApiPublishApi();

  const [publishedApis, setPublishedApis] = useState<Record<string, ApiPublicationWithPortal[]>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refreshPublishedApis = useCallback(
    async (apiId: string) => {
      setLoading(true);
      setError(null);

      try {
        const publishedList = await fetchPublications(apiId);
        setPublishedApis((prev) => ({ ...prev, [apiId]: publishedList }));
        return publishedList;
      } catch (err) {
        const message = err instanceof Error ? err.message : "Failed to fetch published APIs";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [fetchPublications]
  );

  const publishApiToDevPortal = useCallback(
    async (apiId: string, payload: ApiPublishPayload): Promise<PublishResult | void> => {
      setError(null);
      setLoading(true);

      try {
        const res = await publishRequest(apiId, payload);

        // If backend returned a publication or reference, attempt to refresh/merge
        try {
          await refreshPublishedApis(apiId);
        } catch {
          // swallow — refresh failure will be exposed via error state already
        }

        return res;
      } catch (err) {
        const message = err instanceof Error ? err.message : "Failed to publish API to devportal";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [publishRequest, refreshPublishedApis]
  );


  const getPublishStatus = useCallback(
    (apiId: string, devPortalId: string) => {
      const apiPublished = publishedApis[apiId] || [];
      return apiPublished.find(p => p.uuid === devPortalId);
    },
    [publishedApis]
  );

  const unpublishApiFromDevPortal = useCallback(
    async (apiId: string, devPortalId: string) => {
      setError(null);
      setLoading(true);

      try {
        const result = await unpublishRequest(apiId, devPortalId);

        // Update local state: remove the devPortal entry from publishedApis[apiId]
        setPublishedApis((prev) => {
          const list = prev[apiId] ?? [];
          const nextList = list.filter((p) => p.uuid !== devPortalId);
          return { ...prev, [apiId]: nextList };
        });

        return result;
      } catch (err) {
        const message = err instanceof Error ? err.message : "Failed to unpublish API from devportal";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [unpublishRequest]
  );

  // Clear published APIs when organization changes (including when switching between orgs)
  useEffect(() => {
    if (organizationLoading) return;
    // Always clear when org changes, not just when it becomes null
    // This prevents data leakage when switching between organizations
    setPublishedApis({});
    setLoading(false);
  }, [organization, organizationLoading]);

  const value = useMemo<ApiPublishContextValue>(
    () => ({
      publishedApis,
      loading,
      error,
      refreshPublishedApis,
      publishApiToDevPortal,
      unpublishApiFromDevPortal,
      getPublishStatus,
    }),
    [
      publishedApis,
      loading,
      error,
      refreshPublishedApis,
      publishApiToDevPortal,
      unpublishApiFromDevPortal,
      getPublishStatus,
    ]
  );

  return (
    <ApiPublishContext.Provider value={value}>
      {children}
    </ApiPublishContext.Provider>
  );
};

export const useApiPublishing = () => {
  const context = useContext(ApiPublishContext);

  if (!context) {
    throw new Error("useApiPublishing must be used within an ApiPublishProvider");
  }

  return context;
};
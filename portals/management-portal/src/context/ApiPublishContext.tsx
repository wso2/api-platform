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
  useApiPublishApi,
  type UnpublishResponse,
  type ApiPublicationWithPortal,
  type ApiPublishPayload,
  type PublishResponse,
} from "../hooks/apiPublish";

import { useOrganization } from './OrganizationContext';

/* -------------------------------------------------------------------------- */
/*                                Type Definitions                            */
/* -------------------------------------------------------------------------- */

export type ApiPublishContextValue = {
  publishedApis: ApiPublicationWithPortal[];
  loading: boolean;

  refreshPublishedApis: (apiId: string) => Promise<ApiPublicationWithPortal[]>;
  publishApiToDevPortal: (
    apiId: string,
    payload: ApiPublishPayload
  ) => Promise<PublishResponse | void>;
  unpublishApiFromDevPortal: (
    apiId: string,
    devPortalId: string
  ) => Promise<UnpublishResponse>;
  getPublication: (
    devPortalId: string
  ) => ApiPublicationWithPortal | undefined;
  clearPublishedApis: () => void;
};

type ApiPublishProviderProps = { children: ReactNode };

/* -------------------------------------------------------------------------- */
/*                                   Context                                  */
/* -------------------------------------------------------------------------- */

const ApiPublishContext = createContext<ApiPublishContextValue | undefined>(
  undefined
);

/* -------------------------------------------------------------------------- */
/*                                   Provider                                 */
/* -------------------------------------------------------------------------- */

export const ApiPublishProvider = ({ children }: ApiPublishProviderProps) => {
  const { organization, loading: orgLoading } = useOrganization();
  const {
    fetchPublications,
    publishApiToDevPortal: publishRequest,
    unpublishApiFromDevPortal: unpublishRequest,
  } = useApiPublishApi();

  const [publishedApis, setPublishedApis] = useState<
    ApiPublicationWithPortal[]
  >([]);
  const [loading, setLoading] = useState(false);

  // Ref to track previous organization to avoid unnecessary clears
  const prevOrgRef = useRef<string | undefined>(undefined);

  /* ------------------------------- API Actions ------------------------------ */

  /** Refresh publications for a specific API */
  const refreshPublishedApis = useCallback(
    async (apiId: string) => {
      setLoading(true);

      try {
        const list = await fetchPublications(apiId);
        setPublishedApis(list);
        return list;
      } finally {
        setLoading(false);
      }
    },
    [fetchPublications]
  );

  /** Publish API to devportal */
  const publishApiToDevPortal = useCallback(
    async (apiId: string, payload: ApiPublishPayload) => {
      setLoading(true);

      try {
        const response = await publishRequest(apiId, payload);

        // Refresh state after a successful publish
        try {
          await refreshPublishedApis(apiId);
        } catch {
          // Publish succeeded but refresh failed - user may need to manually refresh
          console.warn('Failed to refresh publications after publish');
        }

        return response;
      } finally {
        setLoading(false);
      }
    },
    [publishRequest, refreshPublishedApis]
  );

  /** Unpublish API from devportal */
  const unpublishApiFromDevPortal = useCallback(
    async (apiId: string, devPortalId: string) => {
      setLoading(true);

      try {
        const result = await unpublishRequest(apiId, devPortalId);

        // Update local state
        setPublishedApis((prev) => prev.filter((p) => p.uuid !== devPortalId));

        return result;
      } finally {
        setLoading(false);
      }
    },
    [unpublishRequest]
  );

  /** Get publish status for specific (apiId, devPortalId) */
  const getPublication = useCallback(
    (devPortalId: string) => {
      return publishedApis.find((p) => p.uuid === devPortalId);
    },
    [publishedApis]
  );

  /** Clear published APIs */
  const clearPublishedApis = useCallback(() => {
    setPublishedApis([]);
  }, []);

  /* ----------------------------- Organization Switch ----------------------------- */

  useEffect(() => {
    if (orgLoading) return;

    const currentOrgId = organization?.id;
    const prevOrgId = prevOrgRef.current;

    // Only clear data if organization actually changed
    if (currentOrgId !== prevOrgId) {
      setPublishedApis([]);
      setLoading(false);
      prevOrgRef.current = currentOrgId;
    }
  }, [organization, orgLoading]);

  /* -------------------------------- Context Value ------------------------------- */

  const value = useMemo<ApiPublishContextValue>(
    () => ({
      publishedApis,
      loading,
      refreshPublishedApis,
      publishApiToDevPortal,
      unpublishApiFromDevPortal,
      getPublication,
      clearPublishedApis,
    }),
    [
      publishedApis,
      loading,
      refreshPublishedApis,
      publishApiToDevPortal,
      unpublishApiFromDevPortal,
      getPublication,
      clearPublishedApis,
    ]
  );

  return (
    <ApiPublishContext.Provider value={value}>
      {children}
    </ApiPublishContext.Provider>
  );
};

/* -------------------------------------------------------------------------- */
/*                                 Consumer Hook                               */
/* -------------------------------------------------------------------------- */

export const useApiPublishing = () => {
  const ctx = useContext(ApiPublishContext);
  if (!ctx) {
    throw new Error('useApiPublishing must be used within an ApiPublishProvider');
  }
  return ctx;
};

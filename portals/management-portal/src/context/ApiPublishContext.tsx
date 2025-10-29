import React, {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  useApiPublishApi,
  type PublishResponse,
  type UnpublishResponse,
} from "../hooks/apiPublish";

/* ---------- Local publish state shape ---------- */

export type ApiPublishState = {
  isPublished: boolean;
  lastPublishedAt?: string;
  lastUnpublishedAt?: string;
  apiPortalRefId?: string;
  lastMessage?: string;
};

type ApiPublishContextValue = {
  // Per-API publish cache
  publishStateByApi: Record<string, ApiPublishState>;

  // Status & errors
  loading: boolean;
  error: string | null;

  // Actions
  publish: (apiId: string) => Promise<PublishResponse>;
  unpublish: (apiId: string) => Promise<UnpublishResponse>;

  // Optional helper to query current state
  getPublishState: (apiId: string) => ApiPublishState | undefined;
};

const ApiPublishContext = createContext<ApiPublishContextValue | undefined>(undefined);

type Props = { children: ReactNode };

export const ApiPublishProvider = ({ children }: Props) => {
  const { publishApi, unpublishApi } = useApiPublishApi();

  const [publishStateByApi, setPublishStateByApi] = useState<Record<string, ApiPublishState>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const publish = useCallback(
    async (apiId: string) => {
      setLoading(true);
      setError(null);
      try {
        const resp = await publishApi(apiId);

        setPublishStateByApi((prev) => ({
          ...prev,
          [apiId]: {
            isPublished: true,
            lastPublishedAt: resp.publishedAt,
            apiPortalRefId: resp.apiPortalRefId,
            lastMessage: resp.message,
          },
        }));

        return resp;
      } catch (err) {
        const msg = err instanceof Error ? err.message : "Failed to publish API";
        setError(msg);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [publishApi]
  );

  const unpublish = useCallback(
    async (apiId: string) => {
      setLoading(true);
      setError(null);
      try {
        const resp = await unpublishApi(apiId);

        setPublishStateByApi((prev) => ({
          ...prev,
          [apiId]: {
            ...(prev[apiId] ?? { isPublished: false }),
            isPublished: false,
            lastUnpublishedAt: resp.unpublishedAt,
            lastMessage: resp.message,
          },
        }));

        return resp;
      } catch (err) {
        const msg = err instanceof Error ? err.message : "Failed to unpublish API";
        setError(msg);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [unpublishApi]
  );

  const getPublishState = useCallback(
    (apiId: string) => publishStateByApi[apiId],
    [publishStateByApi]
  );

  const value = useMemo<ApiPublishContextValue>(
    () => ({
      publishStateByApi,
      loading,
      error,
      publish,
      unpublish,
      getPublishState,
    }),
    [publishStateByApi, loading, error, publish, unpublish, getPublishState]
  );

  return <ApiPublishContext.Provider value={value}>{children}</ApiPublishContext.Provider>;
};

export const useApiPublishContext = () => {
  const ctx = useContext(ApiPublishContext);
  if (!ctx) {
    throw new Error("useApiPublishContext must be used within an ApiPublishProvider");
  }
  return ctx;
};

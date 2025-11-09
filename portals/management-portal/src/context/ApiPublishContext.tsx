import {
  createContext,
  useCallback,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { type PublicationAPIModel } from "../hooks/useApiPublications";
import { publishToDevPortalApi, unpublishFromDevPortalApi } from "../hooks/apiPublishApi";

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
  isPending: boolean;
  error: string | null;

  // Actions
  publishToDevPortal: (apiId: string, devPortalUuid: string, sandboxGatewayId: string, productionGatewayId: string) => Promise<PublicationAPIModel>;
  unpublishFromDevPortal: (apiId: string, publicationUuid: string) => Promise<void>;

  // Helper methods
  getPublishState: (apiId: string) => ApiPublishState | undefined;
};

const ApiPublishContext = createContext<ApiPublishContextValue | undefined>(undefined);
type Props = { children: ReactNode };

export const ApiPublishProvider = ({ children }: Props) => {
  const [publishStateByApi, setPublishStateByApi] = useState<Record<string, ApiPublishState>>({});
  const [error, setError] = useState<string | null>(null);
  const [isPending, setIsPending] = useState(false);


  const getPublishState = useCallback(
    (apiId: string) => publishStateByApi[apiId],
    [publishStateByApi]
  );

  // New publish method using the updated endpoint (delegates HTTP I/O to helper)
  const publishToDevPortal = useCallback(
    async (
      apiId: string,
      devPortalUuid: string,
      sandboxGatewayId: string,
      productionGatewayId: string
    ): Promise<PublicationAPIModel> => {
      setError(null);
      setIsPending(true);

      setPublishStateByApi((prev) => ({
        ...prev,
        [apiId]: {
          ...(prev[apiId] ?? { isPublished: false }),
          lastMessage: `Publishing to devportal ${devPortalUuid}`,
        },
      }));

      try {
        const publication = await publishToDevPortalApi(
          apiId,
          devPortalUuid,
          sandboxGatewayId,
          productionGatewayId
        );

        // Update local publish state
        setPublishStateByApi((prev) => ({
          ...prev,
          [apiId]: {
            ...(prev[apiId] ?? {}),
            isPublished: true,
            lastPublishedAt: new Date().toISOString(),
            apiPortalRefId: publication.devPortalUuid,
            lastMessage: "Published to devportal",
          },
        }));

        return publication;
      } catch (err) {
        const msg = err instanceof Error ? err.message : "Failed to publish API to devportal";
        setError(msg);
        // mark publish as failed
        setPublishStateByApi((prev) => ({
          ...prev,
          [apiId]: {
            ...(prev[apiId] ?? {}),
            isPublished: false,
            lastMessage: msg,
          },
        }));
        throw err;
      } finally {
        setIsPending(false);
      }
    },
    []
  );

  // New unpublish method (delegates HTTP I/O to helper)
  const unpublishFromDevPortal = useCallback(
    async (apiId: string, publicationUuid: string): Promise<void> => {
      setError(null);
      try {
        await unpublishFromDevPortalApi(apiId, publicationUuid);
        
        // Update publish state after successful unpublish
        setPublishStateByApi((prev) => ({
          ...prev,
          [apiId]: {
            ...(prev[apiId] ?? {}),
            isPublished: false,
            lastUnpublishedAt: new Date().toISOString(),
            lastMessage: "Unpublished from devportal",
            apiPortalRefId: undefined,
          },
        }));
      } catch (err) {
        const msg = err instanceof Error ? err.message : "Failed to unpublish API from devportal";
        setError(msg);
        throw err;
      }
    },
    []
  );

  const value = useMemo<ApiPublishContextValue>(
    () => ({
      publishStateByApi,
      isPending,
      error,
      publishToDevPortal,
      unpublishFromDevPortal,
      getPublishState,
    }),
    [publishStateByApi, isPending, error, publishToDevPortal, unpublishFromDevPortal, getPublishState]
  );

  return <ApiPublishContext.Provider value={value}>{children}</ApiPublishContext.Provider>;
};

export { ApiPublishContext };

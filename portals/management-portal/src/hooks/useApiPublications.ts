import { useCallback, useEffect, useRef, useState } from "react";
import { getApiConfig } from "./apiConfig";

export type PublicationEndpoint = {
  gatewayId: string;
  DisplayName?: string;
  FunctionalityType?: string;
  vhost?: string;
};

export type PublicationDetails = {
  sandboxEndpointUrl?: string;
  productionEndpointUrl?: string;
  apiVersion?: string;
};

// Backend API Model - matches raw server response exactly
export type PublicationAPIModel = {
  uuid: string;
  devPortalUuid: string;
  devPortalName: string;
  status: 'published' | 'unpublished' | 'failed' | 'publishing' | 'unpublishing';
  sandboxEndpoint?: PublicationEndpoint;
  productionEndpoint?: PublicationEndpoint;
  publicationDetails?: PublicationDetails;
};

// Frontend UI Model - extends API model with UI-specific fields
export type PublicationUIModel = PublicationAPIModel & {
  // Add UI-specific fields here if needed in the future
};

// Mapper function to convert API model to UI model
export const mapPublicationToUI = (apiModel: PublicationAPIModel): PublicationUIModel => {
  return {
    ...apiModel,
  };
};

type UseApiPublicationsReturn = {
  publications: PublicationUIModel[];
  loading: boolean;
  error: Error | null;
  refetch: () => Promise<void>;
};

export const useApiPublications = (
  apiId: string,
  options?: { enabled?: boolean }
): UseApiPublicationsReturn => {
  const [publications, setPublications] = useState<PublicationUIModel[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  const enabled = options?.enabled ?? true;

  const fetchPublications = useCallback(async () => {
    if (!apiId || !enabled) return;

    // Cancel previous request
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }

    const abortController = new AbortController();
    abortControllerRef.current = abortController;

    setLoading(true);
    setError(null);

    try {
      const { token, baseUrl } = getApiConfig();

      const url = new URL(`${baseUrl}/api/v1/apis/${encodeURIComponent(apiId)}/publications`);

      const response = await fetch(url.toString(), {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        signal: abortController.signal,
      });

      if (!response.ok) {
        if (response.status === 404) {
          setPublications([]);
          return;
        }

        const errorBody = await response.text();
        throw new Error(
          `Failed to fetch publications for API ${apiId}: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

  const data = await response.json();

      // The endpoint returns an array directly
      let publicationList: PublicationAPIModel[] = [];
      if (Array.isArray(data)) {
        publicationList = data;
      }

      // Map API models to UI models
  const uiPublications = publicationList.map(mapPublicationToUI);
  setPublications(uiPublications);

    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') {
        // Request was cancelled, don't set error state
        return;
      }

  const error = err instanceof Error ? err : new Error('Unknown error occurred');
  setError(error);
    } finally {
      setLoading(false);
    }
  }, [apiId, enabled]);

  const refetch = useCallback(async () => {
    await fetchPublications();
  }, [fetchPublications]);

  useEffect(() => {
    fetchPublications();

    // Cleanup function to abort any ongoing requests
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, [fetchPublications]);

  return {
    publications,
    loading,
    error,
    refetch,
  };
};
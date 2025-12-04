import { useCallback } from 'react';
import { getApiConfig } from './apiConfig';
import { parseApiError } from '../utils/apiErrorUtils';

// ---------------------------------------------------------
// Types
// ---------------------------------------------------------

export type PublishResponse = {
  success: boolean;
  message: string;
  timestamp: string;
};

export type UnpublishResponse = {
  success: boolean;
  message: string;
  timestamp: string;
};

export type Publication = {
  status: string;
  apiVersion: string;
  sandboxEndpoint: string;
  productionEndpoint: string;
  publishedAt: string;
  updatedAt: string;
};

export type ApiPublicationWithPortal = {
  uuid: string;
  name: string;
  identifier: string;
  description: string;
  portalUrl: string;
  apiUrl: string;
  hostname: string;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
  associatedAt: string;
  isPublished: boolean;
  publication: Publication;
};

export interface ApiPublishPayload {
  devPortalUUID: string;
  endPoints: {
    productionURL: string;
    sandboxURL: string;
  };
  apiInfo: {
    apiName: string;
    apiDescription: string;
    visibility: 'PUBLIC' | 'PRIVATE';
    tags: string[];
    labels: string[];
    owners: {
      technicalOwner: string;
      technicalOwnerEmail: string;
      businessOwner: string;
      businessOwnerEmail: string;
    };
  };
  subscriptionPolicies: string[];
}

export type PublicationsListResponse = {
  count: number;
  list: any[];
  pagination?: {
    total: number;
    offset: number;
    limit: number;
  } | null;
};

// ---------------------------------------------------------
// Helpers
// ---------------------------------------------------------

const normalizePublication = (item: any): ApiPublicationWithPortal => ({
  ...item,
  portalUrl: item.uiUrl ?? item.portalUrl,
});

// ---------------------------------------------------------
// API Hook: useApiPublishApi
// ---------------------------------------------------------

export const useApiPublishApi = () => {
  const fetchPublications = useCallback(
    async (apiId: string): Promise<ApiPublicationWithPortal[]> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(
        `${baseUrl}/api/v1/apis/${apiId}/publications`,
        {
          method: 'GET',
          headers: { Authorization: `Bearer ${token}` },
        }
      );

      if (!response.ok) {
        const errorMessage = await parseApiError(
          response,
          'fetch publications'
        );
        throw new Error(errorMessage);
      }

      const data = (await response.json()) as PublicationsListResponse;
      return (data.list ?? []).map(normalizePublication);
    },
    []
  );

  const publishApiToDevPortal = useCallback(
    async (
      apiId: string,
      payload: ApiPublishPayload
    ): Promise<PublishResponse> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(
        `${baseUrl}/api/v1/apis/${apiId}/devportals/publish`,
        {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(payload),
        }
      );

      if (!response.ok) {
        const errorMessage = await parseApiError(response, 'publish API');
        throw new Error(errorMessage);
      }

      return (await response.json()) as PublishResponse;
    },
    []
  );

  const unpublishApiFromDevPortal = useCallback(
    async (apiId: string, devPortalId: string): Promise<UnpublishResponse> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(
        `${baseUrl}/api/v1/apis/${apiId}/devportals/${devPortalId}/unpublish`,
        {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ devPortalId }),
        }
      );

      if (!response.ok) {
        const errorMessage = await parseApiError(response, 'unpublish API');
        throw new Error(errorMessage);
      }

      return (await response.json()) as UnpublishResponse;
    },
    []
  );

  return {
    fetchPublications,
    publishApiToDevPortal,
    unpublishApiFromDevPortal,
  };
};

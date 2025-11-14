import { useCallback } from "react";
import { getApiConfig } from "./apiConfig";

/* ---------- Response types ---------- */

export type PublishResponse = {
  message: string; // "API published successfully to API portal"
  apiId: string;
  apiPortalRefId: string;
  publishedAt: string; // ISO string
};

export type UnpublishResponse = {
  message: string; // "API unpublished successfully from API portal"
  apiId: string;
  unpublishedAt: string; // ISO string
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
  isActive: boolean;
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

type PublicationsListResponse = {
  count: number;
  list: any[];
  pagination?: { total: number; offset: number; limit: number } | null;
};

const normalizePublication = (item: any): ApiPublicationWithPortal => ({
  ...item,
  portalUrl: item.uiUrl ?? item.portalUrl,
});

// Hook that centralizes API calls related to publishing
export const useApiPublishApi = () => {
  const fetchPublications = useCallback(async (apiId: string): Promise<ApiPublicationWithPortal[]> => {
    const { token, baseUrl } = getApiConfig();
    const response = await fetch(`${baseUrl}/api/v1/apis/${apiId}/publications`, {
      method: "GET",
      headers: { Authorization: `Bearer ${token}` },
    });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(`Failed to fetch publications: ${response.status} ${response.statusText} ${errorBody}`);
    }

    const data = (await response.json()) as PublicationsListResponse;
    return (data.list ?? []).map(normalizePublication);
  }, []);

  const publishApiToDevPortal = useCallback(async (apiId: string, payload: ApiPublishPayload): Promise<PublishResponse> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/apis/${apiId}/devportals/publish`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(`Failed to publish API: ${response.status} ${response.statusText} ${errorBody}`);
    }

    const data = (await response.json()) as PublishResponse;
    return data;
  }, []);

  const unpublishApiFromDevPortal = useCallback(async (apiId: string, devPortalId: string): Promise<UnpublishResponse> => {
    const { token, baseUrl } = getApiConfig();

    // Assume unpublish endpoint follows this shape; adjust if backend differs
    const response = await fetch(`${baseUrl}/api/v1/apis/${apiId}/devportals/${devPortalId}/unpublish`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ devPortalId }),
    });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(`Failed to unpublish API: ${response.status} ${response.statusText} ${errorBody}`);
    }

    const data = (await response.json()) as UnpublishResponse;
    return data;
  }, []);

  return {
    fetchPublications,
    publishApiToDevPortal,
    unpublishApiFromDevPortal,
  };
};


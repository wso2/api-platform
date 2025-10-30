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

/* ---------- Low-level API hook ---------- */

export const useApiPublishApi = () => {
  const publishApi = useCallback(async (apiId: string): Promise<PublishResponse> => {
    const { token, baseUrl } = getApiConfig();

    const res = await fetch(`${baseUrl}/api/v1/apis/${apiId}/api-portals/publish`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!res.ok) {
      const body = await res.text();
      throw new Error(
        `Failed to publish API ${apiId}: ${res.status} ${res.statusText} ${body}`
      );
    }

    const data = (await res.json()) as PublishResponse;
    return data;
  }, []);

  const unpublishApi = useCallback(async (apiId: string): Promise<UnpublishResponse> => {
    const { token, baseUrl } = getApiConfig();

    const res = await fetch(`${baseUrl}/api/v1/apis/${apiId}/api-portals/unpublish`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!res.ok) {
      const body = await res.text();
      throw new Error(
        `Failed to unpublish API ${apiId}: ${res.status} ${res.statusText} ${body}`
      );
    }

    const data = (await res.json()) as UnpublishResponse;
    return data;
  }, []);

  return { publishApi, unpublishApi };
};

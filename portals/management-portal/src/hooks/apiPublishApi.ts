import { getApiConfig } from "./apiConfig";
import type { PublicationAPIModel } from "./useApiPublications";

export const publishToDevPortalApi = async (
  apiId: string,
  devPortalUuid: string,
  sandboxGatewayId: string,
  productionGatewayId: string
): Promise<PublicationAPIModel> => {
  const { token, baseUrl } = getApiConfig();

  const response = await fetch(`${baseUrl}/api/v1/apis/${apiId}/devportals/publish`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ devPortalUuid, sandboxGatewayId, productionGatewayId }),
  });

  if (!response.ok) {
    const body = await response.text();
    throw new Error(
      `Failed to publish API to devportal: ${response.status} ${response.statusText} ${body}`
    );
  }

  const publication = (await response.json()) as PublicationAPIModel;
  return publication;
};

export const unpublishFromDevPortalApi = async (
  apiId: string,
  publicationUuid: string
): Promise<void> => {
  const { token, baseUrl } = getApiConfig();

  const response = await fetch(`${baseUrl}/api/v1/apis/${apiId}/publications/${publicationUuid}`, {
    method: "DELETE",
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) {
    const body = await response.text();
    throw new Error(
      `Failed to unpublish API from devportal: ${response.status} ${response.statusText} ${body}`
    );
  }
};

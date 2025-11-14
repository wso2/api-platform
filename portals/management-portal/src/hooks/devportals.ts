import { useCallback } from "react";
import { getApiConfig } from "./apiConfig";

export type Portal = {
  logoSrc: string;
  logoAlt: string;
  uuid: string;
  organizationUuid: string;
  name: string;
  identifier: string;
  uiUrl: string;
  apiUrl: string;
  hostname: string;
  isActive: boolean;
  visibility: "public" | "private";
  description: string;
  headerKeyName: string;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
};

export type DevPortalAPIModel = Portal;


export type CreatePortalPayload = {
  name: string;
  identifier: string;
  description: string;
  apiUrl: string;
  hostname: string;
  apiKey: string;
  headerKeyName: string;
};

export type CreatePortalData = CreatePortalPayload;

export type UpdatePortalPayload = Partial<CreatePortalPayload>;

export type UpdatePortalData = CreatePortalPayload;

type PortalListResponse = {
  count: number;
  list: Portal[];
  pagination: {
    total: number;
    offset: number;
    limit: number;
  };
};

export const useDevPortalsApi = () => {
  // Fetch all dev portals
  const fetchDevPortals = useCallback(async (): Promise<Portal[]> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/devportals`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(
        `Failed to fetch devportals: ${response.status} ${response.statusText} ${errorBody}`
      );
    }

    const data: PortalListResponse = await response.json();
    return data.list ?? [];
  }, []);

  // Fetch single portal by ID
  const fetchDevPortal = useCallback(
    async (uuid: string): Promise<Portal> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(`${baseUrl}/api/v1/devportals/${uuid}`, {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to fetch devportal ${uuid}: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      return await response.json();
    },
    []
  );

  // Create portal
  const createDevPortal = useCallback(
    async (payload: CreatePortalPayload): Promise<Portal> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(`${baseUrl}/api/v1/devportals`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to create devportal: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      return await response.json();
    },
    []
  );

  // Update portal 
  const updateDevPortal = useCallback(
    async (
      uuid: string,
      portalData: UpdatePortalData
    ): Promise<DevPortalAPIModel> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(`${baseUrl}/api/v1/devportals/${uuid}`, {
        method: "PUT",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(portalData),
      });

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to update devportal ${uuid}: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      return await response.json();
    },
    []
  );

  // Delete portal
  const deleteDevPortal = useCallback(
    async (uuid: string): Promise<void> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(`${baseUrl}/api/v1/devportals/${uuid}`, {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to delete devportal ${uuid}: ${response.status} ${response.statusText} ${errorBody}`
        );
      }
    },
    []
  );

  // ACTION: Activate portal

  const activateDevPortal = useCallback(async (uuid: string): Promise<void> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/devportals/${uuid}/activate`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(
        `Failed to activate devportal ${uuid}: ${response.status} ${response.statusText} ${errorBody}`
      );
    }
  }, []);

  // Export all API operations

  return {
    createDevPortal,
    fetchDevPortals,
    fetchDevPortal,
    updateDevPortal,
    deleteDevPortal,
    activateDevPortal,
  };
};

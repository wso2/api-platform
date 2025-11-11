import { useCallback } from "react";
import { getApiConfig } from "./apiConfig";

// Backend API Model - matches raw server response exactly
export type DevPortalAPIModel = {
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

// Frontend UI Model - extends API model with UI-specific fields
export type DevPortalUIModel = DevPortalAPIModel & {
  description: string;
  logoSrc?: string;
  logoAlt?: string;
  portalUrl: string;
  userAuthLabel: string;
  authStrategyLabel: string;
  visibilityLabel: string;
};

type DevPortalListResponse = {
  count: number;
  list: DevPortalAPIModel[];
  pagination: {
    total: number;
    offset: number;
    limit: number;
  };
};

// Mapper function to convert API model to UI model
export const mapDevPortalToUI = (apiModel: DevPortalAPIModel): DevPortalUIModel => {
  return {
    ...apiModel,
    // description: `Developer portal for ${apiModel.name}`,
    logoSrc: undefined, // Will be set by UI layer with default if needed
    logoAlt: `${apiModel.name} logo`,
    portalUrl: apiModel.uiUrl,
    userAuthLabel: "Asgardeo Thunder",
    authStrategyLabel: apiModel.headerKeyName || "Auth-Key",
    visibilityLabel: apiModel.visibility === "public" ? "Public" : "Private",
  };
};

export const useDevPortalsApi = () => {
  const fetchDevPortals = useCallback(async (): Promise<DevPortalUIModel[]> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/devportals`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      const errorBody = await response.text();
      const errorMessage = `Failed to fetch devportals: ${response.status} ${response.statusText} ${errorBody}`;
      throw new Error(errorMessage);
    }

    const data: DevPortalListResponse = await response.json();

    // Map API models to UI models
    return data.list.map(mapDevPortalToUI);
  }, []);

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
      const errorMessage = `Failed to activate devportal ${uuid}: ${response.status} ${response.statusText} ${errorBody}`;
      throw new Error(errorMessage);
    }
  }, []);

  const createDevPortal = useCallback(async (portalData: {
    name: string;
    identifier: string;
    apiUrl: string;
    hostname: string;
    apiKey: string;
    headerKeyName: string;
    description: string;
  }): Promise<DevPortalAPIModel> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/devportals`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(portalData),
    });

    if (!response.ok) {
      const errorBody = await response.text();
      const errorMessage = `Failed to create devportal: ${response.status} ${response.statusText} ${errorBody}`;
      throw new Error(errorMessage);
    }

    const data: DevPortalAPIModel = await response.json();
    return data;
  }, []);

  const updateDevPortal = useCallback(async (uuid: string, portalData: {
    name: string;
    identifier: string;
    apiUrl: string;
    hostname: string;
    apiKey: string;
    headerKeyName: string;
    description: string;
  }): Promise<DevPortalAPIModel> => {
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
      const errorMessage = `Failed to update devportal ${uuid}: ${response.status} ${response.statusText} ${errorBody}`;
      throw new Error(errorMessage);
    }

    const data: DevPortalAPIModel = await response.json();
    return data;
  }, []);

  return {
    fetchDevPortals,
    activateDevPortal,
    createDevPortal,
    updateDevPortal,
  };
};
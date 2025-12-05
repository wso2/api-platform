import { useCallback } from 'react';
import { getApiConfig } from './apiConfig';
import { parseApiError } from '../utils/apiErrorUtils';

/* -------------------------------------------------------------------------- */
/*                               Type Definitions                             */
/* -------------------------------------------------------------------------- */

export interface Portal {
  uuid: string;
  organizationUuid: string;
  name: string;
  identifier: string;
  apiUrl: string;
  hostname: string;
  uiUrl: string;
  logoSrc: string;
  logoAlt: string;
  headerKeyName: string;
  description: string;
  isDefault: boolean;
  isEnabled: boolean;
  visibility: 'public' | 'private';
  createdAt: string;
  updatedAt: string;
}

export interface PortalUIModel {
  uuid: string;
  name: string;
  identifier: string;
  description: string;
  apiUrl: string;
  hostname: string;
  portalUrl: string;
  logoSrc?: string;
  logoAlt?: string;
  userAuthLabel: string;
  authStrategyLabel: string;
  visibilityLabel: string;
  isEnabled: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface CreatePortalPayload {
  name: string;
  identifier: string;
  description: string;
  apiUrl: string;
  hostname: string;
  apiKey: string;
  headerKeyName: string;
  visibility: 'public' | 'private';
}

export type CreatePortalRequest = CreatePortalPayload;
export type UpdatePortalPayload = Partial<CreatePortalPayload>;
export type PortalApiResponse = PortalUIModel;

export type DevPortalResponse = {
  success: boolean;
  message: string;
  timestamp: string;
};

export interface PortalListResponse {
  count: number;
  list: Portal[];
  pagination: {
    total: number;
    offset: number;
    limit: number;
  };
}

/* -------------------------------------------------------------------------- */
/*                               API Hook - Service                           */
/* -------------------------------------------------------------------------- */

export const useDevPortalsApi = () => {
  /** Fetch all dev portals */
  const fetchDevPortals = useCallback(async (): Promise<Portal[]> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/devportals`, {
      headers: { Authorization: `Bearer ${token}` },
    });

    if (!response.ok) {
      const errorMessage = await parseApiError(response, 'fetch devportals');
      throw new Error(errorMessage);
    }

    const data: PortalListResponse = await response.json();
    return data.list ?? [];
  }, []);

  /** Fetch single portal */
  const fetchDevPortal = useCallback(async (uuid: string): Promise<Portal> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/devportals/${uuid}`, {
      headers: { Authorization: `Bearer ${token}` },
    });

    if (!response.ok) {
      const errorMessage = await parseApiError(
        response,
        `fetch devportal (${uuid})`
      );
      throw new Error(errorMessage);
    }

    return await response.json();
  }, []);

  /** Create portal */
  const createDevPortal = useCallback(
    async (payload: CreatePortalPayload): Promise<Portal> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(`${baseUrl}/api/v1/devportals`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const errorMessage = await parseApiError(response, 'create devportal');
        throw new Error(errorMessage);
      }

      return await response.json();
    },
    []
  );

  /** Update portal */
  const updateDevPortal = useCallback(
    async (uuid: string, updates: UpdatePortalPayload): Promise<Portal> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(`${baseUrl}/api/v1/devportals/${uuid}`, {
        method: 'PUT',
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(updates),
      });

      if (!response.ok) {
        const errorMessage = await parseApiError(
          response,
          `update devportal (${uuid})`
        );
        throw new Error(errorMessage);
      }

      return await response.json();
    },
    []
  );

  /** Delete portal */
  const deleteDevPortal = useCallback(async (uuid: string): Promise<void> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/devportals/${uuid}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` },
    });

    if (!response.ok) {
      const errorMessage = await parseApiError(response, 'delete devportal');
      throw new Error(errorMessage);
    }
  }, []);

  /** Activate portal */
  const activateDevPortal = useCallback(async (uuid: string): Promise<void> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(
      `${baseUrl}/api/v1/devportals/${uuid}/activate`,
      {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      }
    );

    if (!response.ok) {
      const errorMessage = await parseApiError(response, 'activate devportal');
      throw new Error(errorMessage);
    }

    // Activation successful, no response body expected
  }, []);

  return {
    fetchDevPortals,
    fetchDevPortal,
    createDevPortal,
    updateDevPortal,
    deleteDevPortal,
    activateDevPortal,
  };
};

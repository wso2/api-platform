/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import {
  getGateways,
  registerGateway,
  rotateGatewayToken,
  updateGateway,
  deleteGateway,
  HybridGateway,
  RegisterGatewayRequest,
  UpdateGatewayRequest,
} from '../apis/gateway/gatewayApi';
import { useAIWorkspaceSnackbar } from './aiWorkspaceSnackbar';
import { useAppShell } from '../contexts/AppShellContext';
import {
  trackHybridGatewayCreate,
  trackHybridGatewayUpdate,
  trackHybridGatewayDelete,
} from '../utils/app-insights';
import { getErrorMessage } from '../utils/apiError';

interface UseGatewayListReturn {
  gateways: HybridGateway[];
  isLoading: boolean;
  error: Error | null;
  selectedGateway: HybridGateway | null;
  setSelectedGateway: (gateway: HybridGateway | null) => void;
  refetch: () => Promise<void>;
  createGateway: (data: RegisterGatewayRequest) => Promise<HybridGateway>;
  updateGatewayById: (
    id: string,
    data: UpdateGatewayRequest
  ) => Promise<HybridGateway>;
  deleteGatewayById: (id: string) => Promise<void>;
  isCreating: boolean;
  isUpdating: boolean;
  isDeleting: boolean;
}

type UseGatewayListOptions = {
  pollAlways?: boolean;
  pollingIntervalMs?: number;
};

export function useGatewayList(
  options: UseGatewayListOptions = {}
): UseGatewayListReturn {
  const { pollAlways = false, pollingIntervalMs = 5000 } = options;
  const [gateways, setGateways] = useState<HybridGateway[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [selectedGateway, setSelectedGateway] = useState<HybridGateway | null>(
    null
  );
  const [isCreating, setIsCreating] = useState(false);
  const [isUpdating, setIsUpdating] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  const showSnackbar = useAIWorkspaceSnackbar();
  const { currentOrganization } = useAppShell();
  const organizationId = currentOrganization?.uuid || '';

  const fetchGateways = useCallback(async () => {
    if (!organizationId) {
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const response = await getGateways(organizationId);
      const fetchedGateways: HybridGateway[] = (response.data?.list || []).map(
        (gateway) => ({
          ...gateway,
          status: gateway.isActive ? 'connected' : 'disconnected',
        })
      );

      setGateways(fetchedGateways);
    } catch (err) {
      console.error('Failed to fetch hybrid gateways:', err);
      setError(
        err instanceof Error ? err : new Error('Failed to fetch gateways')
      );
      setGateways([]);
    } finally {
      setIsLoading(false);
    }
  }, [organizationId]);

  useEffect(() => {
    fetchGateways();
  }, [fetchGateways]);

  // Poll while the page needs fresh status updates.
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    const hasInactive = gateways.some((gw) => !gw.isActive);
    const shouldPoll = pollAlways || hasInactive;

    if (shouldPoll && !isLoading) {
      if (!pollingRef.current) {
        pollingRef.current = setInterval(async () => {
          if (!organizationId) return;
          try {
            const response = await getGateways(organizationId);
            const fetchedGateways: HybridGateway[] = (
              response.data?.list || []
            ).map((gateway) => ({
              ...gateway,
              status: gateway.isActive ? 'connected' : 'disconnected',
            }));
            setGateways(fetchedGateways);
          } catch (err) {
            console.error('Polling failed:', err);
          }
        }, pollingIntervalMs);
      }
    } else if (pollingRef.current) {
      clearInterval(pollingRef.current);
      pollingRef.current = null;
    }

    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
        pollingRef.current = null;
      }
    };
  }, [gateways, isLoading, organizationId, pollAlways, pollingIntervalMs]);

  // Auto-select first gateway when gateways list changes and none is selected
  useEffect(() => {
    if (gateways.length > 0 && !selectedGateway) {
      setSelectedGateway(gateways[0]);
    }
  }, [gateways, selectedGateway]);

  const createGateway = useCallback(
    async (data: RegisterGatewayRequest): Promise<HybridGateway> => {
      if (!organizationId) {
        throw new Error('Organization ID is required');
      }

      setIsCreating(true);
      try {
        const response = await registerGateway(data, organizationId);
        let initialToken = response.data.token ?? null;

        // The create-gateway response does not always include the one-time
        // registration token, so generate it immediately for the first-view flow.
        if (!initialToken) {
          try {
            initialToken = await rotateGatewayToken(response.data.id, organizationId);
          } catch (tokenError) {
            console.error('Failed to generate initial gateway registration token:', tokenError);
            showSnackbar(
              'Gateway created, but the initial registration token could not be generated. Use Reconfigure on the gateway page.',
              'warning'
            );
          }
        }

        const newGateway: HybridGateway = {
          ...response.data,
          status: response.data.isActive ? 'connected' : 'pending',
          token: initialToken,
        };

        // Add to local state
        setGateways((prev) => [...prev, newGateway]);

        showSnackbar('Self-Hosted Gateway registered successfully', 'success');

        // Track gateway creation
        trackHybridGatewayCreate(
          organizationId,
          newGateway.id,
          newGateway.functionalityType,
          (newGateway as any).environment
        );

        return newGateway;
      } catch (err) {
        const errorMessage =
          err instanceof Error
            ? err.message
            : 'Failed to register self-hosted gateway';
        showSnackbar(errorMessage, 'error');
        throw err;
      } finally {
        setIsCreating(false);
      }
    },
    [organizationId, showSnackbar]
  );

  const updateGatewayById = useCallback(
    async (id: string, data: UpdateGatewayRequest): Promise<HybridGateway> => {
      if (!organizationId) {
        throw new Error('Organization ID is required');
      }

      setIsUpdating(true);
      try {
        const response = await updateGateway(id, data, organizationId);
        const updatedGateway: HybridGateway = {
          ...response.data,
          status: response.data.isActive ? 'connected' : 'disconnected',
        };

        // Update local state
        setGateways((prev) =>
          prev.map((gw) => (gw.id === id ? updatedGateway : gw))
        );

        // Update selected gateway if it was the one updated
        if (selectedGateway?.id === id) {
          setSelectedGateway(updatedGateway);
        }

        showSnackbar('Gateway updated successfully', 'success');

        // Track gateway update
        trackHybridGatewayUpdate(
          organizationId,
          updatedGateway.id,
          updatedGateway.functionalityType,
          (updatedGateway as any).environment
        );

        return updatedGateway;
      } catch (err) {
        const errorMessage =
          err instanceof Error ? err.message : 'Failed to update gateway';
        showSnackbar(errorMessage, 'error');
        throw err;
      } finally {
        setIsUpdating(false);
      }
    },
    [organizationId, selectedGateway, showSnackbar]
  );

  const deleteGatewayById = useCallback(
    async (id: string): Promise<void> => {
      if (!organizationId) {
        throw new Error('Organization ID is required');
      }

      setIsDeleting(true);
      try {
        // Capture gateway before deleting for tracking
        const gatewayToDelete = gateways.find((gw) => gw.id === id) || null;

        await deleteGateway(id, organizationId);

        // Remove from local state
        setGateways((prev) => prev.filter((gw) => gw.id !== id));

        // Clear selected gateway if it was the one deleted
        if (selectedGateway?.id === id) {
          setSelectedGateway(null);
        }

        showSnackbar('Gateway deleted successfully', 'success');

        // Track gateway delete
        trackHybridGatewayDelete(organizationId, gatewayToDelete?.id || id);
      } catch (err: any) {
        const errorMessage = getErrorMessage(err, 'Failed to delete gateway');
        showSnackbar(errorMessage, 'error');
        throw err;
      } finally {
        setIsDeleting(false);
      }
    },
    [organizationId, selectedGateway, showSnackbar]
  );

  return {
    gateways,
    isLoading,
    error,
    selectedGateway,
    setSelectedGateway,
    refetch: fetchGateways,
    createGateway,
    updateGatewayById,
    deleteGatewayById,
    isCreating,
    isUpdating,
    isDeleting,
  };
}

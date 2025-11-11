// src/hooks/usePortalOperations.ts
import { useCallback } from 'react';
import { useDevPortals } from './useDevPortals';
import { PORTAL_CONSTANTS } from '../constants/portal';
import type { PortalFormData, PortalUIModel } from '../types/portal';
import { PortalError } from '../types/portal';

export interface UsePortalOperationsReturn {
  createPortal: (formData: PortalFormData) => Promise<PortalUIModel>;
  updatePortal: (uuid: string, formData: PortalFormData) => Promise<void>;
  activatePortal: (uuid: string) => Promise<void>;
  isCreating: boolean;
  isUpdating: boolean;
  isActivating: boolean;
  error: string | null;
}

export const usePortalOperations = (): UsePortalOperationsReturn => {
  const { createDevPortal, updateDevPortal, activateDevPortal } = useDevPortals();

  const createPortal = useCallback(async (formData: PortalFormData): Promise<PortalUIModel> => {
    try {
      return await createDevPortal(formData);
    } catch {
      const portalError = new PortalError(
        PORTAL_CONSTANTS.MESSAGES.CREATION_FAILED,
        'CREATE_PORTAL_ERROR',
        500
      );
      throw portalError;
    }
  }, [createDevPortal]);

  const updatePortal = useCallback(async (uuid: string, formData: PortalFormData): Promise<void> => {
    try {
      await updateDevPortal(uuid, formData);
    } catch {
      const portalError = new PortalError(
        'Failed to update developer portal.',
        'UPDATE_PORTAL_ERROR',
        500
      );
      throw portalError;
    }
  }, [updateDevPortal]);

  const activatePortal = useCallback(async (uuid: string): Promise<void> => {
    try {
      await activateDevPortal(uuid);
    } catch {
      const portalError = new PortalError(
        PORTAL_CONSTANTS.MESSAGES.ACTIVATION_FAILED,
        'ACTIVATE_PORTAL_ERROR',
        500
      );
      throw portalError;
    }
  }, [activateDevPortal]);

  return {
    createPortal,
    updatePortal,
    activatePortal,
    isCreating: false, // TODO: Add loading states from context
    isUpdating: false, // TODO: Add loading states from context
    isActivating: false, // TODO: Add loading states from context
    error: null, // TODO: Add error states from context
  };
};
// src/pages/PortalManagement.tsx
import React, { useCallback, useMemo, useState } from 'react';
import { useNavigate, useParams, useLocation } from 'react-router-dom';
import { Box, Typography } from '@mui/material';
import { DevPortalProvider, useDevPortals } from '../context/DevPortalContext';
import { useNotifications } from '../context/NotificationContext';
import ErrorBoundary from '../components/ErrorBoundary';
import PortalList from './portals/PortalList';
import PortalForm from './portals/PortalForm';
import ThemeContainer from './portals/ThemeContainer';
import { PORTAL_CONSTANTS } from '../constants/portal';
import {
  getPortalMode,
  getPortalIdFromPath,
  navigateToPortalList,
  navigateToPortalCreate,
  navigateToPortalTheme,
  navigateToPortalEdit,
} from '../utils/portalUtils';
import type { CreatePortalPayload, UpdatePortalPayload } from '../hooks/devportals';

type PortalManagementProps = Record<string, never>;

const PortalManagementContent: React.FC<PortalManagementProps> = () => {
  // Context access (from DevPortalProvider)
  const {
    devportals,
    loading,
    createDevPortal,
    updateDevPortal,
    activateDevPortal,
  } = useDevPortals();

  // Router access
  const navigate = useNavigate();
  const location = useLocation();
  const params = useParams();

  const [creatingPortal, setCreatingPortal] = useState(false);
  const { showNotification } = useNotifications();

  const mode = useMemo(
    () => getPortalMode(location.pathname),
    [location.pathname]
  );
  const selectedPortalId = useMemo(
    () => getPortalIdFromPath(location.pathname) || params.portalId || null,
    [location.pathname, params.portalId]
  );

  const selectedPortal = useMemo(
    () => devportals.find((p) => p.uuid === selectedPortalId),
    [devportals, selectedPortalId]
  );

  const navigateToList = useCallback(
    () => navigateToPortalList(navigate, location.pathname),
    [navigate, location.pathname]
  );

  const navigateToCreate = useCallback(
    () => navigateToPortalCreate(navigate, location.pathname),
    [navigate, location.pathname]
  );

  const navigateToTheme = useCallback(
    (portalId: string) =>
      navigateToPortalTheme(navigate, location.pathname, portalId),
    [navigate, location.pathname]
  );

  const navigateToEdit = useCallback(
    (portalId: string) =>
      navigateToPortalEdit(navigate, location.pathname, portalId),
    [navigate, location.pathname]
  );

  const handlePortalClick = useCallback(
    (portalId: string) => {
      navigateToTheme(portalId);
    },
    [navigateToTheme]
  );

  const handlePortalActivate = useCallback(
    async (portalId: string) => {
      try {
        await activateDevPortal(portalId);
        showNotification(PORTAL_CONSTANTS.MESSAGES.PORTAL_ACTIVATED, 'success');
      } catch (error) {
        const errorMessage =
          error instanceof Error
            ? error.message
            : PORTAL_CONSTANTS.MESSAGES.ACTIVATION_FAILED;
        showNotification(errorMessage, 'error');
      }
    },
    [activateDevPortal, showNotification]
  );

  const handlePortalEdit = useCallback(
    (portalId: string) => {
      navigateToEdit(portalId);
    },
    [navigateToEdit]
  );

  const handleCreatePortal = React.useCallback(
    async (formData: CreatePortalPayload | UpdatePortalPayload) => {
      const fullData = formData as CreatePortalPayload;
      setCreatingPortal(true);
      try {
        const createdPortal = await createDevPortal(fullData);
        const activationMessage = createdPortal.isEnabled
          ? 'Developer portal created and enabled successfully.'
          : 'Developer portal created successfully, but not enabled.';
        showNotification(activationMessage, 'success');
        // Navigate to theme screen for the new portal
        navigateToTheme(createdPortal.uuid);
      } catch (error) {
        const errorMessage =
          error instanceof Error
            ? error.message
            : PORTAL_CONSTANTS.MESSAGES.CREATION_FAILED;
        showNotification(errorMessage, 'error');
      } finally {
        setCreatingPortal(false);
      }
    },
    [createDevPortal, navigateToTheme, showNotification]
  );

  const handleUpdatePortal = useCallback(
    async (formData: UpdatePortalPayload) => {
      if (!selectedPortalId) return;

      try {
        await updateDevPortal(selectedPortalId, formData);
        showNotification('Developer Portal updated successfully.', 'success');
        navigateToList();
      } catch (error) {
        const errorMessage =
          error instanceof Error
            ? error.message
            : PORTAL_CONSTANTS.MESSAGES.CREATION_FAILED;
        showNotification(errorMessage, 'error');
      }
    },
    [selectedPortalId, updateDevPortal, navigateToList, showNotification]
  );

  const renderContent = () => {
    switch (mode) {
      case PORTAL_CONSTANTS.MODES.LIST:
        return (
          <PortalList
            portals={devportals}
            loading={loading}
            onPortalClick={handlePortalClick}
            onPortalActivate={handlePortalActivate}
            onPortalEdit={handlePortalEdit}
            onCreateNew={navigateToCreate}
          />
        );

      case PORTAL_CONSTANTS.MODES.FORM:
        return (
          <PortalForm
            onSubmit={handleCreatePortal}
            onCancel={navigateToList}
            isSubmitting={creatingPortal}
          />
        );

      case PORTAL_CONSTANTS.MODES.EDIT:
        return (
          <PortalForm
            onSubmit={handleUpdatePortal}
            onCancel={navigateToList}
            initialData={
              selectedPortal
                ? {
                    name: selectedPortal.name,
                    identifier: selectedPortal.identifier,
                    description: selectedPortal.description,
                    apiUrl: selectedPortal.apiUrl,
                    hostname: selectedPortal.hostname,
                    apiKey: '**********', // Masked for security
                    headerKeyName: selectedPortal.headerKeyName,
                    visibility: selectedPortal.visibility,
                  }
                : undefined
            }
            isEdit={true}
          />
        );

      case PORTAL_CONSTANTS.MODES.THEME:
        return (
          <ThemeContainer
            portalName={selectedPortal?.name}
            onBack={navigateToList}
            onPublish={undefined}
          />
        );

      default:
        return null;
    }
  };

  return (
    <ErrorBoundary>
      <Box sx={{ overflowX: 'auto' }}>
        {/* Header */}
        {mode === PORTAL_CONSTANTS.MODES.LIST && (
          <Box>
            <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
              <Typography variant="h3" fontWeight={700}>
                Developer Portals
              </Typography>
            </Box>

            <Typography variant="body2" sx={{ mt: 0.5, mb: 3, maxWidth: 760 }}>
              Define visibility of your portal and publish your first API. You
              can modify your selections later.
            </Typography>
          </Box>
        )}

        {/* Content */}
        {renderContent()}
      </Box>
    </ErrorBoundary>
  );
};

const PortalManagement: React.FC<PortalManagementProps> = (props) => {
  return (
    <DevPortalProvider>
      <PortalManagementContent {...props} />
    </DevPortalProvider>
  );
};

export default PortalManagement;

// src/pages/PortalManagement.tsx
import React from "react";
import { Box, Typography, Snackbar, Alert } from "@mui/material";
import { DevPortalProvider } from "../context/DevPortalContext";
import { useDevPortals } from "../hooks/useDevPortals";
import { usePortalNavigation } from "../hooks/usePortalNavigation";
import { usePortalOperations } from "../hooks/usePortalOperations";
import ErrorBoundary from "../components/ErrorBoundary";
import PortalList from "./portals/PortalList";
import PortalForm from "./portals/PortalForm";
import ThemeContainer from "./portals/ThemeContainer";
import { PORTAL_CONSTANTS } from "../constants/portal";
import type { PortalManagementProps, PortalFormData } from "../types/portal";

const PortalManagementContent: React.FC<PortalManagementProps> = () => {
  const { devportals, loading, error } = useDevPortals();
  const { mode, selectedPortalId, navigateToList, navigateToCreate, navigateToTheme, navigateToEdit } = usePortalNavigation();
  const { createPortal, updatePortal, activatePortal } = usePortalOperations();

  const [snackbar, setSnackbar] = React.useState<{
    open: boolean;
    message: string;
    severity: 'success' | 'error';
  }>({
    open: false,
    message: '',
    severity: 'success',
  });

  const [creatingPortal, setCreatingPortal] = React.useState(false);

  const selectedPortal = React.useMemo(() =>
    devportals.find(p => p.uuid === selectedPortalId),
    [devportals, selectedPortalId]
  );

  const handlePortalClick = React.useCallback((portalId: string) => {
    navigateToTheme(portalId);
  }, [navigateToTheme]);

  const handlePortalActivate = React.useCallback(async (portalId: string) => {
    try {
      await activatePortal(portalId);
      setSnackbar({
        open: true,
        message: PORTAL_CONSTANTS.MESSAGES.PORTAL_ACTIVATED,
        severity: 'success',
      });
    } catch (error) {
      setSnackbar({
        open: true,
        message: error instanceof Error ? error.message : PORTAL_CONSTANTS.MESSAGES.ACTIVATION_FAILED,
        severity: 'error',
      });
    }
  }, [activatePortal]);

  const handlePortalEdit = React.useCallback((portalId: string) => {
    navigateToEdit(portalId);
  }, [navigateToEdit]);

  const handleCreatePortal = React.useCallback(async (formData: PortalFormData) => {
    setCreatingPortal(true);
    try {
      const createdPortal = await createPortal(formData);
      const activationMessage = createdPortal.isActive 
        ? 'Developer portal created and activated successfully.' 
        : 'Developer portal created successfully, but not activated.';
      setSnackbar({
        open: true,
        message: activationMessage,
        severity: 'success',
      });
      // Navigate to theme screen for the new portal
      navigateToTheme(createdPortal.uuid);
    } catch (error) {
      setSnackbar({
        open: true,
        message: error instanceof Error ? error.message : PORTAL_CONSTANTS.MESSAGES.CREATION_FAILED,
        severity: 'error',
      });
    } finally {
      setCreatingPortal(false);
    }
  }, [createPortal, navigateToTheme]);

  const handleUpdatePortal = React.useCallback(async (formData: PortalFormData) => {
    if (selectedPortalId) {
      await updatePortal(selectedPortalId, formData);
    }
  }, [selectedPortalId, updatePortal]);

  const handleCloseSnackbar = React.useCallback(() => {
    setSnackbar(prev => ({ ...prev, open: false }));
  }, []);

  const renderContent = () => {
    switch (mode) {
      case PORTAL_CONSTANTS.MODES.LIST:
        return (
          <PortalList
            portals={devportals}
            loading={loading}
            error={error}
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
            initialData={selectedPortal ? {
              name: selectedPortal.name,
              identifier: selectedPortal.identifier,
              description: selectedPortal.description,
              apiUrl: selectedPortal.apiUrl,
              hostname: selectedPortal.hostname,
              apiKey: '', // API key needs to be re-entered for security
              headerKeyName: '', // Header key name needs to be re-entered
            } : undefined}
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
      <Box sx={{ overflowX: "auto" }}>
        {/* Header */}
        {mode === PORTAL_CONSTANTS.MODES.LIST && (
          <Box>
            <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
              <Typography variant="h3" fontWeight={700}>
                Developer Portals
              </Typography>
            </Box>

            <Typography variant="body2" sx={{ mt: 0.5, mb: 3, maxWidth: 760 }}>
              Define visibility of your portal and publish your first API. You can modify your selections later.
            </Typography>
          </Box>
        )}

        {/* Content */}
        {renderContent()}

        {/* Snackbar */}
        <Snackbar
          open={snackbar.open}
          autoHideDuration={4000}
          onClose={handleCloseSnackbar}
          anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        >
          <Alert
            onClose={handleCloseSnackbar}
            severity={snackbar.severity}
            sx={{ width: "100%" }}
          >
            {snackbar.message}
          </Alert>
        </Snackbar>
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

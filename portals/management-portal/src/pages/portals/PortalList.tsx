// src/pages/portals/PortalList.tsx
import React, { useCallback } from 'react';
import { Box, Grid, Typography, Alert, CircularProgress, Card, CardContent, Stack } from '@mui/material';
import ConfirmationDialog from '../../common/ConfirmationDialog';
import { Button } from '../../components/src/components/Button';
import PortalCard from './PortalCard';
import { PORTAL_CONSTANTS } from '../../constants/portal';
import type { PortalListProps } from '../../types/portal';
import BijiraDPLogo from "../BijiraDPLogo.png";
import NewDP from "../undraw_windows_kqsk.svg";

const PortalList: React.FC<PortalListProps> = ({
  portals,
  loading,
  error,
  onPortalClick,
  onPortalActivate,
  onPortalEdit,
  onCreateNew,
}) => {
  const handlePortalClick = useCallback((portalId: string) => {
    onPortalClick(portalId);
  }, [onPortalClick]);

  // activation will be requested via requestActivate which shows a confirmation dialog

  const [activating, setActivating] = React.useState<Set<string>>(new Set());

  const [confirmationDialog, setConfirmationDialog] = React.useState<{
    open: boolean;
    title: string;
    message: string;
    onConfirm: () => void;
    confirmText?: string;
    severity?: 'info' | 'warning' | 'error';
  }>({ open: false, title: '', message: '', onConfirm: () => {} });

  const requestActivate = React.useCallback((portalId: string) => {
    setConfirmationDialog({
      open: true,
      title: 'Activate Developer Portal',
      message: 'Are you sure you want to activate this developer portal? This will make it available to publish APIs to.',
      onConfirm: () => {
        // set activating state then call parent activation; cleanup when done
        setActivating(prev => new Set(prev).add(portalId));
        const run = async () => {
          try {
            await onPortalActivate(portalId);
          } finally {
            setActivating(prev => {
              const next = new Set(prev);
              next.delete(portalId);
              return next;
            });
          }
        };
        void run();
      },
      confirmText: 'Activate',
      severity: 'info',
    });
  }, [onPortalActivate]);

  const handlePortalEdit = useCallback((portalId: string) => {
    onPortalEdit(portalId);
  }, [onPortalEdit]);

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Alert severity="error" sx={{ mb: 2 }}>
        {error}
      </Alert>
    );
  }

  return (
    <>
      <Grid container spacing={2} ml={1} mb={1}>
        {portals.map((portal) => (
          <Grid key={portal.uuid}>
            <PortalCard
              title={portal.name}
              description={portal.description}
              selected={portal.isActive}
              onClick={() => handlePortalClick(portal.uuid)}
              logoSrc={portal.logoSrc || BijiraDPLogo}
              logoAlt={portal.logoAlt || PORTAL_CONSTANTS.DEFAULT_LOGO_ALT}
              portalUrl={portal.portalUrl || PORTAL_CONSTANTS.DEFAULT_PORTAL_URL}
              userAuthLabel={portal.userAuthLabel || PORTAL_CONSTANTS.DEFAULT_USER_AUTH_LABEL}
              authStrategyLabel={portal.authStrategyLabel || PORTAL_CONSTANTS.DEFAULT_AUTH_STRATEGY_LABEL}
              visibilityLabel={portal.visibilityLabel || PORTAL_CONSTANTS.DEFAULT_VISIBILITY_LABEL}
              onEdit={() => handlePortalEdit(portal.uuid)}
              onActivate={() => requestActivate(portal.uuid)}
              activating={activating.has(portal.uuid)}
            />
          </Grid>
        ))}

        {/* Add New Developer Portal card */}
        <Grid>
          <Card
            variant="outlined"
            sx={{
              minHeight: 365, maxHeight: 363,
              borderRadius: 2,
              borderColor: "divider",
              maxWidth:400
            }}
          >
            <CardContent sx={{ p: 3, height: '100%' , minHeight:350, display: 'flex' }} >
              <Stack
                spacing={3}
                alignItems="center"
                justifyContent="center"
                display={'flex'}
              >
                <Box
                  component="img"
                  src={NewDP}
                  alt="Add developer portal"
                  sx={{ width: 150, maxWidth: "100%", display: "block" }}
                />

                <Typography
                  align="center"
                  color="text.secondary"
                  sx={{ maxWidth: 520 }}
                >
                  Lorem Ipsum is simply dummy text of the printing and
                  typesetting industry.
                </Typography>

                <Button
                  fullWidth
                  onClick={onCreateNew}
                  aria-label={PORTAL_CONSTANTS.ARIA_LABELS.CREATE_PORTAL}
                >
                  Add Your New Developer Portal
                </Button>
              </Stack>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Confirmation dialog for activation */}
      <ConfirmationDialog
        open={confirmationDialog.open}
        onClose={() => setConfirmationDialog((prev) => ({ ...prev, open: false }))}
        onConfirm={() => {
          // caller has already wired onConfirm to start activation and manage state
          confirmationDialog.onConfirm();
        }}
        title={confirmationDialog.title}
        message={confirmationDialog.message}
        confirmText={confirmationDialog.confirmText}
        severity={confirmationDialog.severity}
      />
    </>
  );
};

export default PortalList;
// src/pages/portals/PortalList.tsx
import React, { useCallback, useState } from 'react';
import {
  Box,
  Grid,
  Typography,
  CircularProgress,
  Card,
  CardContent,
  Stack,
} from '@mui/material';
import ConfirmationDialog from '../../common/ConfirmationDialog';
import { Button } from '../../components/src/components/Button';
import PortalCard from './PortalCard';
import { PORTAL_CONSTANTS } from '../../constants/portal';
import type { Portal } from '../../hooks/devportals';
import BijiraDPLogo from '../BijiraDPLogo.png';
import NewDP from '../undraw_windows_kqsk.svg';

interface PortalListProps {
  portals: Portal[];
  loading: boolean;
  error?: string | null;
  onPortalClick: (portalId: string) => void;
  onPortalActivate: (portalId: string) => void;
  onPortalEdit: (portalId: string) => void;
  onCreateNew: () => void;
}

type ConfirmationDialogState = {
  open: boolean;
  title: string;
  message: string;
  onConfirm: () => void;
  confirmText?: string;
  severity?: 'info' | 'warning' | 'error';
};

const initialDialogState: ConfirmationDialogState = {
  open: false,
  title: '',
  message: '',
  onConfirm: () => {},
};

const PortalList: React.FC<PortalListProps> = ({
  portals,
  loading,
  onPortalClick,
  onPortalActivate,
  onPortalEdit,
  onCreateNew,
}) => {
  const [activating, setActivating] = useState<Set<string>>(new Set());
  const [confirmationDialog, setConfirmationDialog] =
    useState<ConfirmationDialogState>(initialDialogState);

  const handlePortalClick = useCallback(
    (portalId: string) => {
      onPortalClick(portalId);
    },
    [onPortalClick]
  );

  const handlePortalEdit = useCallback(
    (portalId: string) => {
      onPortalEdit(portalId);
    },
    [onPortalEdit]
  );

  const requestActivate = useCallback(
    (portalId: string) => {
      setConfirmationDialog({
        open: true,
        title: 'Activate Developer Portal',
        message:
          'Are you sure you want to activate this developer portal? This will make it available to publish APIs to.',
        onConfirm: () => {
          // Set activating state, call parent activation, cleanup when done
          setActivating((prev) => new Set(prev).add(portalId));
          const run = async () => {
            try {
              await onPortalActivate(portalId);
            } finally {
              setActivating((prev) => {
                const next = new Set(prev);
                next.delete(portalId);
                return next;
              });
              // Close dialog after activation
              setConfirmationDialog((prev) => ({ ...prev, open: false }));
            }
          };
          void run();
        },
        confirmText: 'Activate',
        severity: 'info',
      });
    },
    [onPortalActivate]
  );

  const closeConfirmationDialog = useCallback(() => {
    setConfirmationDialog((prev) => ({ ...prev, open: false }));
  }, []);

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <>
      <Grid container spacing={2} ml={1} mb={1}>
        {portals.map((portal) => (
          <Grid key={portal.uuid} sx={{ display: 'flex' }}>
            <PortalCard
              title={portal.name}
              description={portal.description}
              enabled={portal.isEnabled}
              onClick={() => handlePortalClick(portal.uuid)}
              logoSrc={portal.logoSrc || BijiraDPLogo}
              logoAlt={portal.logoAlt || PORTAL_CONSTANTS.DEFAULT_LOGO_ALT}
              portalUrl={portal.uiUrl || PORTAL_CONSTANTS.DEFAULT_PORTAL_URL}
              userAuthLabel={PORTAL_CONSTANTS.DEFAULT_USER_AUTH_LABEL}
              authStrategyLabel={PORTAL_CONSTANTS.DEFAULT_AUTH_STRATEGY_LABEL}
              visibilityLabel={
                portal.visibility === 'public' ? 'Public' : 'Private'
              }
              onEdit={() => handlePortalEdit(portal.uuid)}
              onActivate={() => requestActivate(portal.uuid)}
              activating={activating.has(portal.uuid)}
            />
          </Grid>
        ))}

        {/* Add New Developer Portal Card */}
        <Grid sx={{ display: 'flex' }}>
          <Card
            variant="outlined"
            sx={{
              height: '100%',
              borderRadius: 2,
              borderColor: 'divider',
              maxWidth: 400,
              width: '100%',
            }}
          >
            <CardContent sx={{ p: 3, height: '100%', display: 'flex' }}>
              <Stack
                spacing={3}
                alignItems="center"
                justifyContent="center"
                display="flex"
                sx={{ width: '100%' }}
              >
                <Box
                  component="img"
                  src={NewDP}
                  alt="Add developer portal"
                  sx={{ width: 150, maxWidth: '100%', display: 'block' }}
                />

                <Typography
                  align="center"
                  color="text.secondary"
                  sx={{ maxWidth: 520 }}
                >
                  Add a developer portal to host your APIs — provide developer
                  documentation, enable API discovery, and manage subscriptions
                  for your consumers.
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

      {/* Confirmation Dialog for Activation */}
      <ConfirmationDialog
        open={confirmationDialog.open}
        onClose={closeConfirmationDialog}
        onConfirm={() => {
          confirmationDialog.onConfirm();
          closeConfirmationDialog();
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

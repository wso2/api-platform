// src/pages/portals/ThemeContainer.tsx
import React, { useCallback } from 'react';
import { Grid, Box, Button } from '@mui/material';
import PromoBanner from './PromoBanner';
import ThemeSettingsPanel from './ThemeSettingsPanel';
import { PrivatePreview } from './PortalPreviews';
import { PORTAL_CONSTANTS } from '../../constants/portal';
import type { ThemeContainerProps } from '../../types/portal';
import { useNotifications } from '../../context/NotificationContext';

const ThemeContainer: React.FC<ThemeContainerProps> = ({
  portalName,
  onBack,
  onPublish,
}) => {

  const { showNotification } = useNotifications();

  const handlePublish = useCallback(async () => {
    // Theme publishing may be implemented by the parent via onPublish
    // We'll call it and surface a toast for success / failure.
    try {
      const result = onPublish ? onPublish() : undefined;
      await Promise.resolve(result);
      showNotification('Theme published successfully', 'success');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to publish theme';
      showNotification(message, 'error');
    }
  }, [onPublish, showNotification]);

  const handlePromoBannerClick = useCallback(async () => {
    try {
      const result = onPublish ? onPublish() : undefined;
      await Promise.resolve(result);
      showNotification('Promo action completed', 'success');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Promo action failed';
      showNotification(message, 'error');
    }
  }, [onPublish, showNotification]);

  return (
    <Box>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
        <Button
          variant="text"
          onClick={onBack}
          sx={{ mr: 2, minWidth: 'auto', px: 1 }}
          aria-label={PORTAL_CONSTANTS.ARIA_LABELS.BACK_TO_LIST}
        >
          ← Back
        </Button>
        <Box component="h3" sx={{ fontWeight: 700, m: 0 }}>
          Theme Settings{portalName ? ` - ${portalName}` : ''}
        </Box>
      </Box>

      {/* Description */}
      <Box sx={{ mb: 3 }}>
        Manage and customize the theme settings for your organization.
      </Box>

      {/* Theme Content */}
      <Grid
        container
        columnSpacing={3}
        alignItems="flex-start"
        sx={{ flexWrap: 'nowrap', minWidth: 960 }}
      >
        {/* Left Column: Banner + Settings */}
        <Grid>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2.5 }}>
            <PromoBanner
              imageSrc="/AITheming.svg" // TODO: Use proper asset path
              onPrimary={handlePromoBannerClick}
            />
            <ThemeSettingsPanel />
          </Box>
        </Grid>

        {/* Right Column: Preview + Publish Button */}
        <Grid>
          <Box sx={{ position: 'sticky', top: 24 }}>
            <PrivatePreview />
          </Box>

          {/* Publish Theme Button */}
          <Box sx={{ display: 'flex', justifyContent: 'flex-end', mt: 2 }}>
            <Button
              variant="contained"
              onClick={handlePublish}
              sx={{
                backgroundColor: '#FE8C3A',
                '&:hover': { backgroundColor: '#e67d33' },
              }}
            >
              Publish Theme
            </Button>
          </Box>
        </Grid>
      </Grid>
    </Box>
  );
};

export default ThemeContainer;
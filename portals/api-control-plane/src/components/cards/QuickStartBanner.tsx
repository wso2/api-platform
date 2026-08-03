import type { ReactNode } from 'react';
import { Box, Button, Card, Stack, Typography } from '@wso2/oxygen-ui';

type QuickStartBannerProps = {
  title: string;
  description: string;
  actionLabel?: string;
  onAction?: () => void;
  icon?: ReactNode;
};

/**
 * Slim hero banner shown at the top of the home/overview dashboards, modelled
 * on ai-workspace's `ProxyQuickStartBanner`.
 */
export function QuickStartBanner({
  title,
  description,
  actionLabel,
  onAction,
  icon,
}: QuickStartBannerProps) {
  return (
    <Card
      sx={{
        background: (theme) =>
          `linear-gradient(135deg, ${theme.palette.primary.main}14, ${theme.palette.primary.main}05)`,
        border: '1px solid',
        borderColor: 'divider',
        p: 3,
      }}
    >
      <Stack
        alignItems={{ sm: 'center' }}
        direction={{ xs: 'column', sm: 'row' }}
        justifyContent="space-between"
        spacing={2}
      >
        <Stack alignItems="center" direction="row" spacing={2}>
          {icon && (
            <Box
              sx={{
                alignItems: 'center',
                bgcolor: 'primary.main',
                borderRadius: 2,
                color: 'primary.contrastText',
                display: 'flex',
                height: 44,
                justifyContent: 'center',
                width: 44,
              }}
            >
              {icon}
            </Box>
          )}
          <Box>
            <Typography sx={{ fontWeight: 700 }} variant="h6">
              {title}
            </Typography>
            <Typography color="text.secondary" variant="body2">
              {description}
            </Typography>
          </Box>
        </Stack>
        {actionLabel && onAction && (
          <Button onClick={onAction} sx={{ flexShrink: 0 }} variant="contained">
            {actionLabel}
          </Button>
        )}
      </Stack>
    </Card>
  );
}

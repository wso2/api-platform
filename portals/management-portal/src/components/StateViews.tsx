import { Alert, Box, Button, Typography } from '@wso2/oxygen-ui';

import { AppLoader } from './AppLoader';

export function LoadingState({
  label = 'Loading',
  fullScreen = false,
}: {
  label?: string;
  fullScreen?: boolean;
}) {
  return <AppLoader fullScreen={fullScreen} label={label} />;
}

export function EmptyState({
  title,
  description,
  actionLabel,
  onAction,
}: {
  title: string;
  description?: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  return (
    <Box sx={{ border: '1px dashed', borderColor: 'divider', borderRadius: 3, p: 4, textAlign: 'center' }}>
      <Typography variant="h6">{title}</Typography>
      {description && (
        <Typography color="text.secondary" sx={{ mt: 1 }}>
          {description}
        </Typography>
      )}
      {actionLabel && onAction && (
        <Button variant="contained" sx={{ mt: 3 }} onClick={onAction}>
          {actionLabel}
        </Button>
      )}
    </Box>
  );
}

export function ErrorState({
  title = 'Something went wrong',
  message,
}: {
  title?: string;
  message?: string;
}) {
  return (
    <Alert severity="error">
      <Typography fontWeight={600}>{title}</Typography>
      {message && <Typography>{message}</Typography>}
    </Alert>
  );
}

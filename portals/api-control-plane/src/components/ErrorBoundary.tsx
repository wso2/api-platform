import { Box, Button, Typography } from '@wso2/oxygen-ui';
import { Component, type ErrorInfo, type ReactNode } from 'react';

type ErrorBoundaryProps = {
  children: ReactNode;
  fallback?: (error: Error, reset: () => void) => ReactNode;
};

type ErrorBoundaryState = {
  error?: Error;
};

/**
 * Top-level error boundary. Without this, a render-time throw anywhere in the
 * routed tree (for example `useConsoleScope` used outside its provider) unmounts
 * the whole app and leaves a blank screen.
 */
export class ErrorBoundary extends Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  state: ErrorBoundaryState = {};

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // eslint-disable-next-line no-console
    console.error('Unhandled error in oxygen-console', error, info);
  }

  reset = () => {
    this.setState({ error: undefined });
  };

  render() {
    const { error } = this.state;
    const { children, fallback } = this.props;

    if (!error) return children;
    if (fallback) return fallback(error, this.reset);

    return (
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          flexDirection: 'column',
          gap: 2,
          minHeight: '60vh',
          justifyContent: 'center',
          p: 4,
          textAlign: 'center',
        }}
      >
        <Typography variant="h5">Something went wrong</Typography>
        <Typography color="text.secondary">
          {error.message || 'An unexpected error occurred.'}
        </Typography>
        <Button onClick={() => window.location.reload()} variant="contained">
          Reload
        </Button>
      </Box>
    );
  }
}

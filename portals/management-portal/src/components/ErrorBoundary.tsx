import React from 'react';
import { Box, Button, Typography } from '@mui/material';

type Props = {
  children: React.ReactNode;
  fallbackTitle?: string;
  fallbackMessage?: string;
};

type State = {
  hasError: boolean;
  error?: Error | null;
  errorInfo?: React.ErrorInfo | null;
};

class ErrorBoundary extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null, errorInfo: null };
  }

  static getDerivedStateFromError(error: Error): Partial<State> {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    // You could log to an external service here
    // console.error('Uncaught error:', error, errorInfo);
    this.setState({ error, errorInfo });
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null, errorInfo: null });
  };

  renderFallback() {
    const {
      fallbackTitle = 'Something went wrong',
      fallbackMessage = 'An unexpected error occurred. You can retry or navigate away.',
    } = this.props;

    return (
      <Box sx={{ p: 4, textAlign: 'center' }} role="alert">
        <Typography variant="h5" gutterBottom>
          {fallbackTitle}
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {fallbackMessage}
        </Typography>
        <Box sx={{ display: 'flex', justifyContent: 'center', gap: 2 }}>
          <Button
            variant="contained"
            onClick={this.handleRetry}
            sx={{ textTransform: 'none' }}
          >
            Retry
          </Button>
          <Button variant="outlined" href="/" sx={{ textTransform: 'none' }}>
            Home
          </Button>
        </Box>
      </Box>
    );
  }

  render() {
    if (this.state.hasError) {
      return this.renderFallback();
    }

    return this.props.children as React.ReactElement;
  }
}

export default ErrorBoundary;

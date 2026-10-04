/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import React, { Component, type ErrorInfo, type ReactNode } from 'react';
import { Box, Button, Card, CardContent, Stack, Typography } from '@wso2/oxygen-ui';
import { Bug, RefreshCcw } from '@wso2/oxygen-ui-icons-react';

type QuickStartErrorBoundaryProps = {
  children: ReactNode;
};

type QuickStartErrorBoundaryState = {
  hasError: boolean;
};

export default class QuickStartErrorBoundary extends Component<
  QuickStartErrorBoundaryProps,
  QuickStartErrorBoundaryState
> {
  state: QuickStartErrorBoundaryState = {
    hasError: false,
  };

  static getDerivedStateFromError(): QuickStartErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Quick start wizard failed to render', error, errorInfo);
  }

  private handleRetry = () => {
    this.setState({ hasError: false });
  };

  render() {
    if (this.state.hasError) {
      return (
        <Box
          sx={{
            minHeight: '100%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Card sx={{ width: 'min(460px, 100%)' }}>
            <CardContent sx={{ p: { xs: 3, md: 4 } }}>
              <Stack spacing={2.5} alignItems="center" textAlign="center">
                <Bug size={42} />
                <Stack spacing={0.75}>
                  <Typography variant="h6" fontWeight={600}>
                    Something went wrong
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    We could not load the quick start flow. Retry the view and
                    continue from here.
                  </Typography>
                </Stack>
                <Button
                  startIcon={<RefreshCcw size={16} />}
                  variant="contained"
                  onClick={this.handleRetry}
                >
                  Retry
                </Button>
              </Stack>
            </CardContent>
          </Card>
        </Box>
      );
    }

    return this.props.children;
  }
}

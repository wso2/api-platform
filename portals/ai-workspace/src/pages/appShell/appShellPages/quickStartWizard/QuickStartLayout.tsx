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

import React, { type ReactNode } from 'react';
import {
  Box,
  Button,
  ColorSchemeToggle,
  Divider,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { BookOpen, HelpCircle, X } from '@wso2/oxygen-ui-icons-react';
import Logo from '../../../../Components/Logo';

type QuickStartLayoutProps = {
  children: ReactNode;
  onClose: () => void;
};

const HELP_URL = 'https://wso2.com/bijira/docs/ai-workspace/getting-started/';
const DOCS_URL = 'https://wso2.com/bijira/docs/ai-workspace/llm-providers/overview/';
const TERMS_URL = 'https://wso2.com/bijira/terms-of-use';
const PRIVACY_URL = 'https://wso2.com/bijira/privacy-policy';

export default function QuickStartLayout({
  children,
  onClose,
}: QuickStartLayoutProps) {
  return (
    <Box
      data-testid="ai-workspace-quick-start-layout"
      sx={{
        height: '100vh',
        display: 'grid',
        gridTemplateRows: '72px minmax(0, 1fr) 52px',
      }}
    >
      <Box
        component="header"
        sx={{
          px: { xs: 2, md: 4 },
          py: 1,
          borderBottom: '1px solid',
          borderColor: 'divider',
          backgroundColor: 'background.paper',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 2,
          flexWrap: 'wrap',
        }}
      >
        <Stack direction="row" spacing={2} alignItems="center" minWidth={0}>
          <Logo height={32} />
          <Divider flexItem orientation="vertical" />
          <Typography variant="h5" sx={{ fontWeight: 500 }}>
            Quick Start
          </Typography>
        </Stack>

        <Stack direction="row" spacing={0.5} alignItems="center">
          <Tooltip title="Toggle theme">
            <Box
              sx={{
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <ColorSchemeToggle />
            </Box>
          </Tooltip>
          <Tooltip title="Get help">
            <IconButton
              component="a"
              href={HELP_URL}
              target="_blank"
              rel="noopener noreferrer"
              size="small"
              aria-label="Get help"
              sx={{ color: 'warning.main' }}
            >
              <HelpCircle size={18} />
            </IconButton>
          </Tooltip>
          <Tooltip title="Documentation">
            <IconButton
              component="a"
              href={DOCS_URL}
              target="_blank"
              rel="noopener noreferrer"
              size="small"
              aria-label="Documentation"
              sx={{ color: 'warning.main' }}
            >
              <BookOpen size={18} />
            </IconButton>
          </Tooltip>
          <Divider flexItem orientation="vertical" sx={{ mx: 0.5 }} />
          <IconButton
            size="small"
            aria-label="Close quick start"
            onClick={onClose}
            sx={{ color: 'text.secondary' }}
          >
            <X size={18} />
          </IconButton>
        </Stack>
      </Box>

      <Box
        component="main"
        sx={{
          minHeight: 0,
          overflowY: 'auto',
          px: { xs: 2, md: 4 },
          py: { xs: 3, md: 4 },
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        {children}
      </Box>

      <Box
        component="footer"
        sx={{
          px: { xs: 2, md: 4 },
          py: 0.75,
          borderTop: '1px solid',
          borderColor: 'divider',
          backgroundColor: 'background.paper',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 2,
          flexWrap: 'wrap',
        }}
      >
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          <Button
            component="a"
            href={TERMS_URL}
            target="_blank"
            rel="noopener noreferrer"
            variant="text"
            size="small"
          >
            Terms of service
          </Button>
          <Button
            component="a"
            href={PRIVACY_URL}
            target="_blank"
            rel="noopener noreferrer"
            variant="text"
            size="small"
          >
            Privacy Policy
          </Button>
          <Button
            component="a"
            href={HELP_URL}
            target="_blank"
            rel="noopener noreferrer"
            variant="text"
            size="small"
          >
            Support
          </Button>
        </Stack>
        <Typography variant="body2" color="text.secondary">
          © {new Date().getFullYear()}, WSO2 LLC
        </Typography>
      </Box>
    </Box>
  );
}

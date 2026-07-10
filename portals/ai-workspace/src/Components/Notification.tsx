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

import { ReactNode } from 'react';
import { Box, IconButton, Typography } from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';
import { useAIWorkspaceSnackbarContext } from '../contexts/AIWorkspaceSnackbarContext';

export type NotificationProps = {
  color?: 'success' | 'error' | 'warning' | 'info';
  closeIcon?: boolean;
  testId?: string;
  message: ReactNode;
};

export default function Notification({
  color = 'success',
  closeIcon = false,
  testId,
  message,
}: NotificationProps) {
  const { closeSnackbar } = useAIWorkspaceSnackbarContext();
  const palette = {
    success: { bg: '#E9F8EF', accent: '#22C55E', text: '#1F4D2E' },
    info: { bg: '#E8F1FE', accent: '#3B82F6', text: '#1E3A8A' },
    warning: { bg: '#FFF4E5', accent: '#F59E0B', text: '#7A4B00' },
    error: { bg: '#FDECEC', accent: '#EF4444', text: '#7F1D1D' },
  } as const;
  const tone = palette[color];

  return (
    <Box
      data-testid={testId}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1.25,
        px: 2,
        py: 1.25,
        bgcolor: tone.bg,
        borderRadius: 0.5,
        border: 'none',
        borderLeft: '4px solid',
        borderLeftColor: tone.accent,
        boxShadow: 'none',
        minWidth: 360,
        maxWidth: 560,
      }}
    >
      <Box sx={{ flex: 1, minWidth: 0 }}>
        {typeof message === 'string' ? (
          <Typography
            variant="body2"
            sx={{
              color: tone.text,
              fontSize: 14,
              lineHeight: 1.4,
            }}
          >
            {message}
          </Typography>
        ) : (
          message
        )}
      </Box>
      {closeIcon ? (
        <IconButton
          size="small"
          onClick={closeSnackbar}
          aria-label="Close notification"
          sx={{
            color: '#6B7280',
            ml: 'auto',
            p: 0.25,
          }}
        >
          <X size={12} />
        </IconButton>
      ) : null}
    </Box>
  );
}

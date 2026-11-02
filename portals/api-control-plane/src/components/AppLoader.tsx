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

import { Box, CircularProgress, Stack, Typography } from '@wso2/oxygen-ui';

type AppLoaderProps = {
  /** Text shown under the loader. */
  label?: string;
  /** Center the loader in the full viewport (auth/session/suspense). */
  fullScreen?: boolean;
  /** Scale factor for the spinner (1 = 100px). Defaults to 1, or 0.5 inline. */
  size?: number;
};

/** Oxygen UI loading indicator for full-screen and inline loading states. */
export function AppLoader({ label, fullScreen = false, size }: AppLoaderProps) {
  const spinnerSize = 100 * (size ?? (fullScreen ? 1 : 0.5));

  return (
    <Box
      sx={{
        alignItems: 'center',
        display: 'flex',
        flexGrow: 1,
        justifyContent: 'center',
        minHeight: fullScreen ? '100dvh' : '70dvh',
        boxSizing: 'border-box',
        p: fullScreen ? 2 : 4,
        width: '100%',
      }}
    >
      <Stack alignItems="center" spacing={2.5}>
        <CircularProgress
          aria-label={label}
          color="primary"
          size={spinnerSize}
          sx={{
            '@media (prefers-reduced-motion: reduce)': {
              animation: 'none',
              '& .MuiCircularProgress-circle': { animation: 'none' },
            },
          }}
        />
        {label && (
          <Typography color="text.secondary" variant="body2" sx={{ textAlign: 'center' }}>
            {label}
          </Typography>
        )}
      </Stack>
    </Box>
  );
}

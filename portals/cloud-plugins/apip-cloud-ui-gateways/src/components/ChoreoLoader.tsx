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

import { Box, ColorSchemeImage, Stack, Typography } from '@wso2/oxygen-ui';

import loaderNavy from '../assets/loaders/choreo-labs-loader-navy.svg';
import loaderWhite from '../assets/loaders/choreo-labs-loader-white.svg';

/** The animated Choreo Labs mark: white on a dark scheme, navy on a light one. */
const LOADER_SRC = { dark: loaderWhite, light: loaderNavy };

type ChoreoLoaderProps = {
  /** Text shown under the loader. */
  label?: string;
  /** Centre in the full viewport instead of the page area. */
  fullScreen?: boolean;
  /** Width of the mark in px; height follows its 96x98 proportions. Defaults to 96. */
  width?: number;
};

/** Page loader: the animated mark centred horizontally and vertically in its container. */
export function ChoreoLoader({ label, fullScreen = false, width = 96 }: ChoreoLoaderProps) {
  const height = Math.round((width * 98) / 96);

  return (
    <Box
      sx={{
        alignItems: 'center',
        boxSizing: 'border-box',
        display: 'flex',
        flexGrow: 1,
        justifyContent: 'center',
        minHeight: fullScreen ? '100dvh' : '70dvh',
        p: 2,
        width: '100%',
      }}
    >
      <Stack alignItems="center" spacing={2.5}>
        <ColorSchemeImage
          alt={label ?? 'Loading'}
          height={height}
          src={LOADER_SRC}
          width={width}
        />
        {label && (
          <Typography color="text.secondary" sx={{ textAlign: 'center' }} variant="body2">
            {label}
          </Typography>
        )}
      </Stack>
    </Box>
  );
}

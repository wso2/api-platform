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

import { useEffect, useState } from 'react';
import { Box, ColorSchemeImage, Stack, Typography } from '@wso2/oxygen-ui';

import loaderNavy from '@/assets/icons/loaders/choreo-labs-loader-navy.svg';
import loaderWhite from '@/assets/icons/loaders/choreo-labs-loader-white.svg';
import staticNavy from '@/assets/icons/loaders/choreo-labs-loader-static-navy.svg';
import staticWhite from '@/assets/icons/loaders/choreo-labs-loader-static-white.svg';

/** The animated Choreo Labs mark: white on a dark scheme, navy on a light one. */
const LOADER_SRC = { dark: loaderWhite, light: loaderNavy };
/** Still mark for users who prefer reduced motion: SMIL animation ignores CSS `animation: none`. */
const STATIC_SRC = { dark: staticWhite, light: staticNavy };

/** Whether the user asked the system to reduce motion. */
function usePrefersReducedMotion(): boolean {
  const query = '(prefers-reduced-motion: reduce)';
  const [reduced, setReduced] = useState(
    () => typeof window !== 'undefined' && window.matchMedia?.(query).matches === true,
  );
  useEffect(() => {
    const media = window.matchMedia?.(query);
    if (!media) return undefined;
    const onChange = () => setReduced(media.matches);
    onChange();
    media.addEventListener('change', onChange);
    return () => media.removeEventListener('change', onChange);
  }, []);
  return reduced;
}


type AppLoaderProps = {
  /** Text shown under the loader. */
  label?: string;
  /** Center the loader in the full viewport (auth/session/suspense). */
  fullScreen?: boolean;
  /** Scale factor for the mark (1 = 96px wide). Defaults to 0.6, full screen or inline. */
  size?: number;
};

/** Choreo Labs animated loader for full-screen and inline loading states, centred in its container. */
export function AppLoader({ label, fullScreen = false, size }: AppLoaderProps) {
  const width = Math.round(96 * (size ?? 0.6));
  const height = Math.round((width * 98) / 96);
  const reducedMotion = usePrefersReducedMotion();

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
        <ColorSchemeImage alt={label ?? 'Loading'} height={height} src={reducedMotion ? STATIC_SRC : LOADER_SRC} width={width} />
        {label && (
          <Typography color="text.secondary" variant="body2" sx={{ textAlign: 'center' }}>
            {label}
          </Typography>
        )}
      </Stack>
    </Box>
  );
}

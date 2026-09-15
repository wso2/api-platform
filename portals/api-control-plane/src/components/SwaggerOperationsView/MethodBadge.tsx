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

import { Box } from '@wso2/oxygen-ui';

import { methodPalette } from './methodPalette';

/** Solid, fixed-size HTTP verb label. */

/** Fits the longest verb: `OPTIONS`. */
const WIDTH = 64;
const HEIGHT = 30;

export function MethodBadge({ method }: { method: string }) {
  return (
    <Box
      sx={{
        alignItems: 'center',
        bgcolor: methodPalette(method).badge,
        borderRadius: 0.5,
        color: 'common.white',
        display: 'inline-flex',
        flexShrink: 0,
        fontSize: 12,
        fontWeight: 700,
        height: HEIGHT,
        justifyContent: 'center',
        letterSpacing: 0.35,
        minWidth: WIDTH,
        px: 1.25,
        textTransform: 'uppercase',
      }}
    >
      {method}
    </Box>
  );
}

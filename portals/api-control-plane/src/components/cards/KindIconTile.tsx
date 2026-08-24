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
import { Boxes } from '@wso2/oxygen-ui-icons-react';

/**
 * Neutral icon tile for an API — same treatment as the gateway card's icon
 * tile so the console's cards read as one family. Shared by the API card
 * (grid view) and the API list rows.
 */
export function KindIconTile({ size = 36 }: { size?: number }) {
  return (
    <Box
      sx={{
        alignItems: 'center',
        bgcolor: 'action.hover',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1.5,
        color: 'text.secondary',
        display: 'flex',
        flexShrink: 0,
        height: size,
        justifyContent: 'center',
        width: size,
      }}
    >
      <Boxes size={size >= 44 ? 22 : 20} />
    </Box>
  );
}

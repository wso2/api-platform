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

import type { FC, ReactNode } from 'react';
import { Box, Typography } from '@wso2/oxygen-ui';

export type ActionRowProps = {
  label: string;
  icon: ReactNode;
  onClick: () => void;
};

/** A label-left/icon-right row that acts as a single clickable target — used for settings links that open a detail drawer. */
const ActionRow: FC<ActionRowProps> = ({ label, icon, onClick }) => (
  <Box
    role="button"
    tabIndex={0}
    onClick={onClick}
    onKeyDown={(event) => {
      if (event.key === 'Enter' || event.key === ' ') onClick();
    }}
    sx={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 1,
      cursor: 'pointer',
      color: 'text.primary',
      '&:hover': { color: 'primary.main' },
    }}
  >
    <Typography variant="body2" sx={{ fontWeight: 600 }}>
      {label}
    </Typography>
    <Box sx={{ display: 'flex' }}>{icon}</Box>
  </Box>
);

export default ActionRow;

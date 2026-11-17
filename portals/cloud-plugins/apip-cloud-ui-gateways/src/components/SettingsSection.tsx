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
import { Box, Grid, Typography } from '@wso2/oxygen-ui';
import { secondaryTextSx } from './styles';

export type SettingsSectionProps = {
  title?: string;
  description?: string;
  /** The section's fields, two to a row. */
  children: ReactNode;
};

/**
 * One group of settings: an overline heading, the line the platform gives it,
 * then its fields in two columns. Ungrouped fields from an older platform get
 * no heading.
 */
const SettingsSection: FC<SettingsSectionProps> = ({ title, description, children }) => (
  <Box component="section" sx={{ py: 3 }}>
    {title ? (
      <Box sx={{ mb: 2 }}>
        <Typography
          component="h3"
          variant="overline"
          sx={{
            ...secondaryTextSx,
            display: 'block',
            fontWeight: 600,
            letterSpacing: '0.12em',
            lineHeight: 1.6,
          }}
        >
          {title}
        </Typography>
        {description ? (
          <Typography variant="body2" sx={secondaryTextSx}>
            {description}
          </Typography>
        ) : null}
      </Box>
    ) : null}
    <Grid container columnSpacing={2} rowSpacing={2.5}>
      {children}
    </Grid>
  </Box>
);

/** A cell in a section: half a row, or the whole row when `wide`. */
export const SettingsCell: FC<{ wide?: boolean; children: ReactNode }> = ({ wide, children }) => (
  <Grid size={{ xs: 12, sm: wide ? 12 : 6 }}>{children}</Grid>
);

export default SettingsSection;

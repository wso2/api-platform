/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
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

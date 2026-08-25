/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import type { FC } from 'react';
import { Box, Chip, IconButton, Typography } from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';

export type PipelineStageCardProps = {
  environmentName: string;
  /** The stage's default gateway name — the only one shown, even when the environment has others. */
  gatewayName: string;
  critical?: boolean;
  onRemove?: () => void;
};

const PipelineStageCard: FC<PipelineStageCardProps> = ({
  environmentName,
  gatewayName,
  critical,
  onRemove,
}) => {
  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'flex-start',
        justifyContent: 'space-between',
        gap: 1,
        minWidth: 220,
        px: 2,
        py: 1.5,
        borderRadius: 1.5,
        border: '1px solid',
        borderColor: 'divider',
        bgcolor: 'background.paper',
      }}
    >
      <Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            {environmentName}
          </Typography>
          {critical ? (
            <Chip
              label="Critical"
              size="small"
              color="warning"
              variant="outlined"
              sx={{ height: 20, fontSize: '0.7rem' }}
            />
          ) : null}
        </Box>
        <Typography variant="caption" color="text.secondary">
          {gatewayName}
        </Typography>
      </Box>
      {onRemove ? (
        <IconButton
          size="small"
          aria-label={`Remove ${environmentName}`}
          onClick={onRemove}
          sx={{ mt: -0.5, mr: -1 }}
        >
          <X size={16} />
        </IconButton>
      ) : null}
    </Box>
  );
};

export default PipelineStageCard;

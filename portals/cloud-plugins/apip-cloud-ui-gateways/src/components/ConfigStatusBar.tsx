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

import type { FC } from 'react';
import { Box, IconButton, Tooltip, Typography } from '@wso2/oxygen-ui';
import { CircleAlert, CircleCheck, RefreshCw } from '@wso2/oxygen-ui-icons-react';
import type { ConfigPhase, ConfigStatus } from '../types';

export type ConfigStatusBarProps = {
  status: ConfigStatus;
  onRefresh: () => void;
  refreshing?: boolean;
};

/**
 * The phase of the last configuration change. `applying` is the expected state
 * immediately after ANY write and can persist for minutes — it is not a
 * failure, and the `message` that often comes with it is prose, not an error.
 */
const PHASES: Record<
  ConfigPhase,
  { label: string; color: string; Icon: typeof CircleCheck }
> = {
  applying: { color: 'info.main', Icon: RefreshCw, label: 'Applying' },
  failed: { color: 'error.main', Icon: CircleAlert, label: 'Failed' },
  healthy: { color: 'success.main', Icon: CircleCheck, label: 'Healthy' },
};

const ConfigStatusBar: FC<ConfigStatusBarProps> = ({
  status,
  onRefresh,
  refreshing = false,
}) => {
  // An unrecognised phase is a newer platform than this build; say the word it
  // sent rather than mislabelling it as healthy.
  const phase = PHASES[status.phase] ?? {
    color: 'text.secondary',
    Icon: CircleAlert,
    label: status.phase,
  };

  return (
    <Box
      sx={{
        alignItems: 'center',
        borderBottom: 1,
        borderColor: 'divider',
        display: 'flex',
        gap: 1,
        px: 3,
        py: 1.25,
      }}
    >
      <Box sx={{ color: phase.color, display: 'flex' }}>
        <phase.Icon size={18} />
      </Box>
      <Typography sx={{ color: phase.color, fontWeight: 600 }} variant="body2">
        {phase.label}
      </Typography>
      {status.message ? (
        <Typography
          color="text.secondary"
          sx={{ flex: 1, minWidth: 0 }}
          variant="body2"
        >
          {status.message}
        </Typography>
      ) : null}
      <Tooltip title="Refresh status">
        <IconButton
          aria-label="Refresh status"
          disabled={refreshing}
          onClick={onRefresh}
          size="small"
          sx={{ ml: 'auto' }}
        >
          <RefreshCw size={16} />
        </IconButton>
      </Tooltip>
    </Box>
  );
};

export default ConfigStatusBar;

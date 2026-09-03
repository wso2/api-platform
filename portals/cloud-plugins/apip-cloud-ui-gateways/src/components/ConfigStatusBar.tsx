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
import { Box, Chip, IconButton, Tooltip } from '@wso2/oxygen-ui';
import { RefreshCw } from '@wso2/oxygen-ui-icons-react';
import type { ConfigPhase, ConfigStatus } from '../types';

export type ConfigStatusBarProps = {
  status: ConfigStatus;
  onRefresh: () => void;
  refreshing?: boolean;
};

/**
 * The phase of the last configuration change, as one chip beside the gateway
 * name.
 *
 * `applying` is the expected state immediately after ANY write and can persist
 * for minutes -- it is not a failure. The `message` that often accompanies it
 * is deliberately NOT shown: it is prose of unbounded length, it pushed the
 * form down the drawer, and the phase word is the part a reader acts on. It
 * stays in the response for anyone reading the endpoint directly.
 */
type ChipColor = 'default' | 'info' | 'error' | 'success';

const PHASES: Record<ConfigPhase, { color: ChipColor; label: string }> = {
  applying: { color: 'info', label: 'Applying' },
  failed: { color: 'error', label: 'Failed' },
  healthy: { color: 'success', label: 'Healthy' },
};

const ConfigStatusBar: FC<ConfigStatusBarProps> = ({
  status,
  onRefresh,
  refreshing = false,
}) => {
  // An unrecognised phase is a newer platform than this build; say the word it
  // sent rather than mislabelling it as healthy.
  const phase = PHASES[status.phase] ?? {
    color: 'default' as ChipColor,
    label: status.phase,
  };

  return (
    <Box sx={{ alignItems: 'center', display: 'flex', gap: 0.5 }}>
      <Chip
        color={phase.color}
        label={phase.label}
        size="small"
        sx={{ flexShrink: 0 }}
        variant="outlined"
      />
      <Tooltip title="Refresh status">
        {/* Wrapped: a disabled button fires no events, so the tooltip on it
            would never open while a refresh is in flight. */}
        <Box component="span">
          <IconButton
            aria-label="Refresh status"
            disabled={refreshing}
            onClick={onRefresh}
            size="small"
          >
            <RefreshCw size={14} />
          </IconButton>
        </Box>
      </Tooltip>
    </Box>
  );
};

export default ConfigStatusBar;

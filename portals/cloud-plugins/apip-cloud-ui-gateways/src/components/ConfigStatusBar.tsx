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
import { RefreshCw } from '@wso2/oxygen-ui-icons-react';
import { describeStatus } from '../config/status';
import type { ConfigStatus } from '../types';

export type ConfigStatusBarProps = {
  status: ConfigStatus;
  onRefresh: () => void;
  refreshing?: boolean;
};

/**
 * The configuration's phase as one line of text beside the gateway name — no
 * chip: a healthy gateway shows when its configuration last landed, and only a
 * phase that is still moving or has gone wrong spends the line on a word.
 * `config/status.ts` decides what that line says and why.
 *
 * The Refresh button stays even though the drawer polls: polling is on a
 * 20-second clock and someone watching a change land wants it now. It is also
 * the only path that surfaces a read failure, a background poll being silent by
 * design.
 */
const ConfigStatusBar: FC<ConfigStatusBarProps> = ({
  status,
  onRefresh,
  refreshing = false,
}) => {
  const display = describeStatus(status);

  return (
    <Box sx={{ alignItems: 'center', display: 'flex', gap: 0.5 }}>
      {display ? (
        // An empty title renders no tooltip, so an absent detail needs no branch.
        <Tooltip title={display.detail ?? ''}>
          <Typography
            color={display.tone === 'error' ? 'error.main' : 'text.secondary'}
            sx={{ flexShrink: 0 }}
            variant="body2"
          >
            {display.text}
          </Typography>
        </Tooltip>
      ) : null}
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

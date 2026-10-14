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

import React from 'react';
import {
  Box,
  Card,
  Chip,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { FormattedMessage } from 'react-intl';
import type { AgentCardDocument } from '../../../../utils/types';

type Props = {
  agentCard: AgentCardDocument;
};

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

export default function AgentProxyCardDetails({
  agentCard,
}: Props): React.JSX.Element {
  const name = asString(agentCard.name);
  const description = asString(agentCard.description);
  const version = asString(agentCard.version);

  return (
    <Card sx={{ p: { xs: 2.5, sm: 3 } }}>
      <Stack spacing={2.25}>
        <Stack spacing={0.5} sx={{ minWidth: 0 }}>
          <Stack direction="row" spacing={1.25} alignItems="center">
            <Typography variant="h6" sx={{ fontWeight: 600 }}>
              {name || '—'}
            </Typography>
            {version ? <Chip label={version} size="small" /> : null}
          </Stack>
          {description ? (
            <Typography variant="body2" color="text.secondary">
              {description}
            </Typography>
          ) : null}
        </Stack>

        <Stack spacing={1} sx={{ borderTop: '1px solid', borderColor: 'divider', pt: 2.25 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.agent.card"
              defaultMessage="Agent Card"
            />
          </Typography>
          <Box
            component="pre"
            sx={{
              m: 0,
              p: 2,
              maxHeight: 330,
              overflow: 'auto',
              borderRadius: 1,
              border: '1px solid',
              borderColor: 'divider',
              backgroundColor: 'action.hover',
              fontFamily: 'monospace',
              fontSize: '0.78rem',
              lineHeight: 1.7,
            }}
          >
            {JSON.stringify(agentCard, null, 2)}
          </Box>
        </Stack>
      </Stack>
    </Card>
  );
}

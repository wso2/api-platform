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

import { useState, type FC } from 'react';
import { Box, Button, Card, CardContent, Collapse, Typography } from '@wso2/oxygen-ui';
import { ChevronDown, ChevronUp, Clock, Eye } from '@wso2/oxygen-ui-icons-react';
import ActionRow from './ActionRow';
import EndpointUrlDrawer from './EndpointUrlDrawer';
import StatusDot from './StatusDot';
import StatusPill from './StatusPill';
import { relativeTime } from '../utils/time';
import { gatewayStatusTone } from '../utils/status';
import type { Gateway } from '../types';

export type GatewayRowProps = {
  gateway: Gateway;
  /** Used only to label the scope of this gateway's drawers, e.g. "Development · EU Gateway". */
  environmentName: string;
  busy: boolean;
  /**
   * Whether this artifact's deployment has a backend URL of its own to show. An
   * LLM provider's upstream belongs to the provider rather than to the deployment,
   * so there is nothing per-gateway to read and the row leaves it out instead of
   * offering an empty field.
   */
  showEndpointUrl?: boolean;
  /** Stops what this gateway is serving. */
  onStop: () => void;
};

const GatewayRow: FC<GatewayRowProps> = ({
  gateway,
  environmentName,
  busy,
  showEndpointUrl = true,
  onStop,
}) => {
  const [expanded, setExpanded] = useState(false);
  const [endpointUrlOpen, setEndpointUrlOpen] = useState(false);
  const tone = gatewayStatusTone(gateway.status);
  const scopeLabel = `${environmentName} · ${gateway.name}`;

  // Stopping is the only thing a gateway row does. Getting an artifact back onto a
  // gateway — after a failure or after being stopped — is a deploy, and a deploy goes
  // through the dialog, or through a promotion from the environment before this one.
  // A row-level retry looked like a third way to ship something and was not: it put
  // back whatever that gateway last held, which is the one build the environment may
  // since have moved off.
  //
  // Nothing to stop while a deployment is still settling, or where there is none.
  const actionDisabled =
    busy ||
    gateway.status === 'NOT_DEPLOYED' ||
    gateway.status === 'UNDEPLOYED' ||
    gateway.status === 'DEPLOYING' ||
    gateway.status === 'UNDEPLOYING' ||
    !gateway.deploymentId;

  return (
    <Card>
      <CardContent sx={{ p: 1.5, '&:last-child': { pb: 1.5 } }}>
        <Box
          role="button"
          tabIndex={0}
          onClick={() => setExpanded((prev) => !prev)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') setExpanded((prev) => !prev);
          }}
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1,
            cursor: 'pointer',
            userSelect: 'none',
          }}
        >
          <StatusDot tone={tone.tone} />
          <Box sx={{ flexGrow: 1, minWidth: 0 }}>
            <Typography variant="body2" sx={{ fontWeight: 500 }} noWrap>
              {gateway.name}
            </Typography>
            {gateway.host ? (
              <Typography variant="caption" color="text.secondary" noWrap display="block">
                {gateway.host}
              </Typography>
            ) : null}
          </Box>
          <StatusPill tone={tone} variant="outlined" />
          <Box sx={{ display: 'flex', color: 'text.secondary' }}>
            {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
          </Box>
        </Box>

        <Collapse in={expanded}>
          <Box sx={{ pt: 1.5, display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            {gateway.statusReason ? (
              <Typography variant="caption" color="error">
                {gateway.statusReason}
              </Typography>
            ) : null}

            {gateway.status !== 'NOT_DEPLOYED' ? (
              <>
                {/*
                  When the deployment landed, as a plain line rather than a card: the
                  build it runs is shown once on the environment, so repeating it per
                  gateway only added a label with nothing beside it whenever the build
                  had since been reclaimed.
                */}
                {gateway.deployedAt ? (
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                    <Clock size={13} />
                    <Typography variant="caption" color="text.secondary">
                      Deployed {relativeTime(gateway.deployedAt)}
                    </Typography>
                  </Box>
                ) : null}

                {showEndpointUrl ? (
                  <ActionRow
                    label="Endpoint URL"
                    icon={<Eye size={14} />}
                    onClick={() => setEndpointUrlOpen(true)}
                  />
                ) : null}
              </>
            ) : null}

            <Button
              fullWidth
              variant="outlined"
              color="error"
              disabled={actionDisabled}
              onClick={onStop}
            >
              Stop deployment
            </Button>
          </Box>
        </Collapse>
      </CardContent>

      <EndpointUrlDrawer
        open={endpointUrlOpen}
        onClose={() => setEndpointUrlOpen(false)}
        scopeLabel={scopeLabel}
        endpointUrl={gateway.endpointUrl}
      />
    </Card>
  );
};

export default GatewayRow;

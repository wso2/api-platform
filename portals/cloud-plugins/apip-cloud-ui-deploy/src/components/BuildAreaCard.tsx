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

import { useState, type FC, type MouseEvent } from 'react';
import {
  Box,
  Button,
  ButtonGroup,
  Card,
  CardContent,
  Divider,
  Drawer,
  IconButton,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, Clock, Trash2, X } from '@wso2/oxygen-ui-icons-react';
import { relativeTime } from '../utils/time';
import { activeGatewayCount } from '../utils/status';
import type { Build, Environment } from '../types';

export type BuildAreaCardProps = {
  /** The API's builds, newest first. */
  builds: Build[];
  targetEnvironment: Environment;
  /**
   * Why each build cannot be deleted, by build id. A build a gateway is serving is
   * refused by the platform, so it is shown disabled with the reason rather than
   * offered and then rejected. Builds absent from this map are deletable.
   */
  undeletableBuilds: Record<string, string>;
  busy: boolean;
  /** Opens deployment using an existing build, or creates a build when omitted. */
  onDeployClick: (buildId?: string, createBuild?: boolean) => void;
  /**
   * Deletes a build. An API keeps a limited number, so this is how room is made
   * once deploying is refused for having no free slot.
   */
  onDeleteBuild: (buildId: string) => void;
};

const VISIBLE_BUILD_COUNT = 5;
type DeployAction = 'deploy' | 'build-and-deploy';

/**
 * The head of the pipeline. An existing build can be deployed as-is, or the API
 * can be rebuilt before it is deployed to the first environment.
 */
const BuildAreaCard: FC<BuildAreaCardProps> = ({
  builds,
  targetEnvironment,
  undeletableBuilds,
  busy,
  onDeployClick,
  onDeleteBuild,
}) => {
  const [deployMenuAnchor, setDeployMenuAnchor] = useState<HTMLElement | null>(null);
  const [buildsDrawerOpen, setBuildsDrawerOpen] = useState(false);
  const [selectedDeployAction, setSelectedDeployAction] = useState<DeployAction>('deploy');
  // Deleting a build cannot be undone, so the trash icon asks first rather than
  // acting. Confirming inline keeps it out of a modal, which the All Builds drawer
  // would otherwise have to stack one inside.
  const [pendingDelete, setPendingDelete] = useState<string | null>(null);
  const latestBuild = builds[0] ?? null;
  const visibleBuilds = builds.slice(0, VISIBLE_BUILD_COUNT);
  const canDeploy = activeGatewayCount(targetEnvironment.gateways) > 0;
  const deployDisabledReason = canDeploy
    ? ''
    : `All gateways in ${targetEnvironment.name} are inactive. Activate a gateway before deploying.`;

  const openDeployMenu = (event: MouseEvent<HTMLButtonElement>) => {
    setDeployMenuAnchor(event.currentTarget);
  };

  const chooseDeployAction = (action: DeployAction) => {
    setSelectedDeployAction(action);
    setDeployMenuAnchor(null);
  };

  const runSelectedDeployAction = () => {
    onDeployClick(
      selectedDeployAction === 'deploy' ? latestBuild?.buildId : undefined,
      selectedDeployAction === 'build-and-deploy'
    );
  };

  const renderBuilds = (items: Build[]) => (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      {items.map((build) => {
        const blockedReason = undeletableBuilds[build.buildId];
        const confirming = pendingDelete === build.buildId;
        return (
          <Card key={build.buildId} variant="outlined">
            <CardContent sx={{ p: 1.25, '&:last-child': { pb: 1.25 } }}>
              <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
                <Box sx={{ minWidth: 0, flex: 1 }}>
                  <Typography
                    variant="body2"
                    sx={{
                      color: 'text.primary',
                      fontWeight: 700,
                      fontSize: 14,
                    }}
                  >
                    {build.buildId}
                  </Typography>
                  {build.description ? (
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ display: 'block', mt: 0.25, fontSize: 12 }}
                    >
                      {build.description}
                    </Typography>
                  ) : null}
                  {build.createdAt ? (
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, mt: 0.5 }}>
                      <Clock size={13} />
                      <Typography variant="caption" color="text.secondary" sx={{ fontSize: 12 }}>
                        {relativeTime(build.createdAt)}
                      </Typography>
                    </Box>
                  ) : null}
                </Box>
                {confirming ? null : (
                  <Tooltip title={blockedReason || 'Delete this build'}>
                    <span>
                      <IconButton
                        size="small"
                        aria-label={`Delete build ${build.buildId}`}
                        disabled={busy || Boolean(blockedReason)}
                        onClick={() => setPendingDelete(build.buildId)}
                      >
                        <Trash2 size={15} />
                      </IconButton>
                    </span>
                  </Tooltip>
                )}
              </Box>
              {confirming ? (
                <Box sx={{ mt: 1 }}>
                  <Typography variant="caption" color="text.secondary" sx={{ fontSize: 12 }}>
                    Delete this build? Deployments that ran it stay on their gateways but can no
                    longer be promoted onward.
                  </Typography>
                  <Box sx={{ display: 'flex', gap: 1, mt: 1 }}>
                    <Button
                      size="small"
                      color="error"
                      variant="contained"
                      disabled={busy}
                      onClick={() => {
                        setPendingDelete(null);
                        onDeleteBuild(build.buildId);
                      }}
                    >
                      Delete
                    </Button>
                    <Button size="small" disabled={busy} onClick={() => setPendingDelete(null)}>
                      Cancel
                    </Button>
                  </Box>
                </Box>
              ) : null}
            </CardContent>
          </Card>
        );
      })}
    </Box>
  );

  return (
    <Card
      sx={{
        flex: '0 0 300px',
        width: 300,
        alignSelf: 'flex-start',
      }}
    >
      <CardContent sx={{ p: 2.5, '&:last-child': { pb: 2.5 } }}>
        <Typography sx={{ fontSize: 16, fontWeight: 600 }}>Build Area</Typography>

        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 1.5 }}>
          <Divider />

          {latestBuild ? (
            <Tooltip title={deployDisabledReason}>
              <span style={{ display: 'block' }}>
                <ButtonGroup
                  variant="contained"
                  disabled={!canDeploy || busy}
                  sx={{ display: 'flex', width: '100%' }}
                >
                  <Button
                    onClick={runSelectedDeployAction}
                    sx={{ flex: '1 1 auto', width: 'auto', minWidth: 0, fontWeight: 600 }}
                  >
                    {selectedDeployAction === 'deploy' ? 'Deploy' : 'Build and Deploy'}
                  </Button>
                  <Button
                    aria-label="Choose deploy action"
                    aria-haspopup="menu"
                    aria-expanded={Boolean(deployMenuAnchor)}
                    onClick={openDeployMenu}
                    sx={{ flex: '0 0 44px', width: 44, minWidth: 44, maxWidth: 44, px: 0 }}
                  >
                    <ChevronDown size={18} />
                  </Button>
                </ButtonGroup>
              </span>
            </Tooltip>
          ) : (
            <Tooltip title={deployDisabledReason}>
              <span style={{ display: 'block' }}>
                <Button
                  fullWidth
                  variant="contained"
                  disabled={!canDeploy || busy}
                  onClick={() => onDeployClick(undefined, true)}
                  sx={{ fontWeight: 600 }}
                >
                  Build and deploy
                </Button>
              </span>
            </Tooltip>
          )}

          <Menu
            anchorEl={deployMenuAnchor}
            open={Boolean(deployMenuAnchor)}
            onClose={() => setDeployMenuAnchor(null)}
          >
            <MenuItem onClick={() => chooseDeployAction('deploy')}>Deploy</MenuItem>
            <MenuItem onClick={() => chooseDeployAction('build-and-deploy')}>
              Build and Deploy
            </MenuItem>
          </Menu>

          {visibleBuilds.length > 0 ? (
            <Box>
              {renderBuilds(visibleBuilds)}

              {builds.length > VISIBLE_BUILD_COUNT ? (
                <Button
                  size="small"
                  onClick={() => setBuildsDrawerOpen(true)}
                  sx={{ mt: 1, px: 0 }}
                >
                  See more
                </Button>
              ) : null}
            </Box>
          ) : (
            <Card variant="outlined">
              <CardContent sx={{ p: 1.5, '&:last-child': { pb: 1.5 } }}>
                <Typography variant="body2" color="text.disabled">
                  No builds yet. Deploying prepares one.
                </Typography>
              </CardContent>
            </Card>
          )}
        </Box>
      </CardContent>

      <Drawer anchor="right" open={buildsDrawerOpen} onClose={() => setBuildsDrawerOpen(false)}>
        <Box sx={{ width: 360, p: 3 }}>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              mb: 2,
            }}
          >
            <Typography sx={{ fontSize: 16, fontWeight: 600 }}>All Builds</Typography>
            <IconButton
              size="small"
              onClick={() => setBuildsDrawerOpen(false)}
              aria-label="Close builds"
            >
              <X size={18} />
            </IconButton>
          </Box>
          {renderBuilds(builds)}
        </Box>
      </Drawer>
    </Card>
  );
};

export default BuildAreaCard;

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
import {
  Box,
  Button,
  Card,
  CardContent,
  IconButton,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { relativeTime } from '../utils/time';
import type { Build } from '../types';

export type ProviderBuildsCardProps = {
  /** The provider's builds, newest first. */
  builds: Build[];
  /** Why a build cannot be deleted, by build id; absent means it can. */
  undeletableBuilds: Record<string, string>;
  busy: boolean;
  onDeleteBuild: (buildId: string) => void;
};

const sectionLabelSx = {
  fontSize: 12,
  fontWeight: 600,
  color: 'text.secondary',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.04em',
};

/**
 * The provider's builds, and the one thing there is to do with them here: delete one.
 *
 * Deleting is what frees a slot once the provider is at its build limit and deploying
 * is refused, which is the only reason this list needs to exist — deploying a build is
 * done from an environment, not from here. A build a gateway is serving cannot go, and
 * the row says which environment is holding it rather than failing on the attempt.
 */
const ProviderBuildsCard: FC<ProviderBuildsCardProps> = ({
  builds,
  undeletableBuilds,
  busy,
  onDeleteBuild,
}) => {
  const [pendingDelete, setPendingDelete] = useState<string | null>(null);

  if (builds.length === 0) return null;

  return (
    <Box sx={{ mb: 2 }}>
      <Typography sx={{ ...sectionLabelSx, display: 'block', mb: 1 }}>Builds</Typography>
      <Box
        sx={{
          display: 'grid',
          gap: 1,
          gridTemplateColumns: { xs: '1fr', sm: 'repeat(auto-fill, minmax(280px, 1fr))' },
        }}
      >
        {builds.map((build) => {
          const blockedReason = undeletableBuilds[build.buildId];
          const confirming = pendingDelete === build.buildId;
          return (
            <Card key={build.buildId} variant="outlined">
              <CardContent sx={{ p: 1.25, '&:last-child': { pb: 1.25 } }}>
                <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
                  <Box sx={{ minWidth: 0, flex: 1 }}>
                    <Typography variant="body2" sx={{ fontWeight: 700, fontSize: 14 }}>
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
                      Delete this build?
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
    </Box>
  );
};

export default ProviderBuildsCard;

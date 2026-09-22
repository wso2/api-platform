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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
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

/**
 * The provider's builds, shown alongside its environments.
 *
 * Deleting is the only thing there is to do with them here — deploying a build is done
 * from an environment — and that is housekeeping, needed once the provider is at its
 * build limit and deploys start being refused. So the list stays out of the way of the
 * environments, which are what the page is about, rather than taking the top of it.
 *
 * A build a gateway is serving cannot go, and the row says which environment is holding
 * it rather than failing on the attempt.
 */
const ProviderBuildsCard: FC<ProviderBuildsCardProps> = ({
  builds,
  undeletableBuilds,
  busy,
  onDeleteBuild,
}) => {
  const [pendingDelete, setPendingDelete] = useState<string | null>(null);

  return (
    <Card
      // variant="outlined"
      sx={{
        flex: { md: '0 0 300px' },
        width: { xs: '100%', md: 300 },
        alignSelf: 'flex-start',
      }}
    >
      <CardContent sx={{ p: 2.5, '&:last-child': { pb: 2.5 } }}>
        <Typography sx={{ fontSize: 16, fontWeight: 600 }}>Build Area</Typography>
        <Divider sx={{ my: 2 }} />

        {builds.length === 0 ? (
          <Card variant="outlined">
            <CardContent sx={{ p: 1.5, '&:last-child': { pb: 1.5 } }}>
              <Typography variant="body2" color="text.disabled">
                No builds yet.
              </Typography>
            </CardContent>
          </Card>
        ) : (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            {builds.map((build) => {
              const blockedReason = undeletableBuilds[build.buildId];
              return (
                <Card key={build.buildId} variant="outlined">
                  <CardContent sx={{ p: 1.5, '&:last-child': { pb: 1.5 } }}>
                    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
                      <Box sx={{ minWidth: 0, flexGrow: 1 }}>
                        <Typography variant="body2" sx={{ fontWeight: 700 }} noWrap>
                          {build.buildId}
                        </Typography>
                        {build.description ? (
                          <Typography variant="caption" color="text.secondary" display="block">
                            {build.description}
                          </Typography>
                        ) : null}
                        {build.createdAt ? (
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, mt: 0.5 }}>
                            <Clock size={13} />
                            <Typography variant="caption" color="text.secondary">
                              {relativeTime(build.createdAt)}
                            </Typography>
                          </Box>
                        ) : null}
                      </Box>
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
                    </Box>
                  </CardContent>
                </Card>
              );
            })}
          </Box>
        )}
      </CardContent>

      <Dialog open={pendingDelete !== null} onClose={() => setPendingDelete(null)}>
        <DialogTitle>Delete this build?</DialogTitle>
        <DialogContent>
          <Typography color="text.secondary" variant="body2">
            This action permanently deletes the build and cannot be undone.
          </Typography>
          <Typography sx={{ mt: 1.5 }} variant="body2">
            Build ID:{' '}
            <Box component="span" sx={{ fontFamily: 'monospace', fontWeight: 600 }}>
              {pendingDelete}
            </Box>
          </Typography>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2.5 }}>
          <Button
            color="error"
            variant="contained"
            disabled={busy}
            onClick={() => {
              if (!pendingDelete) return;
              const buildId = pendingDelete;
              setPendingDelete(null);
              onDeleteBuild(buildId);
            }}
          >
            Delete
          </Button>
          <Button disabled={busy} onClick={() => setPendingDelete(null)}>
            Cancel
          </Button>
        </DialogActions>
      </Dialog>
    </Card>
  );
};

export default ProviderBuildsCard;

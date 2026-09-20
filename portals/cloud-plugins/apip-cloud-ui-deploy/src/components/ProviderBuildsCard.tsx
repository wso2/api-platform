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
  Chip,
  Collapse,
  IconButton,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, ChevronUp, Trash2 } from '@wso2/oxygen-ui-icons-react';
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
 * The provider's builds, collapsed to a single line until asked for.
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
  const [expanded, setExpanded] = useState(false);

  if (builds.length === 0) return null;

  return (
    <Box sx={{ mb: 2, border: '1px solid', borderColor: 'divider', borderRadius: 1.5 }}>
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
          gap: 0.75,
          px: 2,
          py: 1.25,
          cursor: 'pointer',
          userSelect: 'none',
        }}
      >
        <Typography sx={sectionLabelSx}>Builds</Typography>
        <Chip label={builds.length} size="small" sx={{ height: 18, fontSize: 11 }} />
        <Box sx={{ flexGrow: 1 }} />
        {!expanded ? (
          <Typography variant="caption" color="text.secondary" noWrap>
            Latest {builds[0].buildId}
          </Typography>
        ) : null}
        <Box sx={{ display: 'flex', color: 'text.secondary' }}>
          {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        </Box>
      </Box>

      <Collapse in={expanded}>
        <Box sx={{ borderTop: '1px solid', borderColor: 'divider' }}>
          {builds.map((build) => {
            const blockedReason = undeletableBuilds[build.buildId];
            const confirming = pendingDelete === build.buildId;
            return (
              <Box
                key={build.buildId}
                sx={{
                  px: 2,
                  py: 1,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 1,
                  borderTop: '1px solid',
                  borderColor: 'divider',
                  '&:first-of-type': { borderTop: 'none' },
                }}
              >
                <Box sx={{ minWidth: 0, flexGrow: 1 }}>
                  <Typography variant="body2" sx={{ fontWeight: 600, fontSize: 13 }} noWrap>
                    {build.buildId}
                  </Typography>
                  <Typography variant="caption" color="text.secondary" noWrap display="block">
                    {build.createdAt ? relativeTime(build.createdAt) : ''}
                    {build.description ? ` · ${build.description}` : ''}
                  </Typography>
                </Box>
                {confirming ? (
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Typography variant="caption" color="text.secondary">
                      Delete?
                    </Typography>
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
                ) : (
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
            );
          })}
        </Box>
      </Collapse>
    </Box>
  );
};

export default ProviderBuildsCard;

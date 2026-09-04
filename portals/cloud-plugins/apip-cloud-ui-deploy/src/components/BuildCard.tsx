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
import { Box, Button, Divider, Typography } from '@wso2/oxygen-ui';
import { Package } from '@wso2/oxygen-ui-icons-react';
import { relativeTime } from '../utils/time';
import { shortBuild } from '../utils/build';
import type { Build } from '../types';

export type BuildCardProps = {
  builds: Build[];
  busy: boolean;
  onPrepare: () => void;
};

const sectionLabelSx = {
  fontSize: 12,
  fontWeight: 600,
  color: 'text.secondary',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.04em',
};

/**
 * The head of the pipeline: preparing a build.
 *
 * This step exists so that editing the API and deploying are not the same action.
 * A build fixes the definition as it stands now; deploying sends that build. An
 * edit made afterwards changes nothing that is running, and nothing that is about
 * to be deployed, until someone prepares again.
 */
const BuildCard: FC<BuildCardProps> = ({ builds, busy, onPrepare }) => {
  const latest = builds[0];

  return (
    <Box
      sx={{
        flex: '0 0 300px',
        width: 300,
        bgcolor: 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: '14px',
        p: 2.5,
        display: 'flex',
        flexDirection: 'column',
        gap: 1.5,
        alignSelf: 'flex-start',
      }}
    >
      <Box>
        <Typography sx={{ fontSize: 16, fontWeight: 600 }}>Build</Typography>
        <Typography variant="body2" color="text.secondary">
          A snapshot of this API, taken when you prepare it.
        </Typography>
      </Box>

      <Divider />

      {latest ? (
        <Box>
          <Typography sx={sectionLabelSx}>Latest</Typography>
          <Typography variant="body2" sx={{ fontWeight: 500, mt: 0.5 }}>
            {shortBuild(latest.buildId)}
          </Typography>
          {latest.createdAt && (
            <Typography variant="caption" color="text.secondary">
              Prepared {relativeTime(latest.createdAt)}
            </Typography>
          )}
        </Box>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1, py: 2 }}>
          <Package size={32} strokeWidth={1.25} />
          <Typography variant="body2" color="text.secondary" textAlign="center">
            Nothing prepared yet. Prepare a build to deploy this API.
          </Typography>
        </Box>
      )}

      <Button fullWidth variant="contained" onClick={onPrepare} disabled={busy}>
        {latest ? 'Prepare new build' : 'Prepare build'}
      </Button>

      {builds.length > 1 && (
        <>
          <Divider sx={{ borderStyle: 'dashed' }} />
          <Box>
            <Typography sx={{ ...sectionLabelSx, mb: 0.75 }}>Earlier</Typography>
            <Box
              sx={{ display: 'flex', flexDirection: 'column', gap: 0.75, maxHeight: 220, overflowY: 'auto' }}
            >
              {builds.slice(1).map((build) => (
                <Box key={build.buildId}>
                  <Typography variant="caption" display="block">
                    {shortBuild(build.buildId)}
                  </Typography>
                  {build.createdAt && (
                    <Typography variant="caption" color="text.disabled" display="block">
                      {relativeTime(build.createdAt)}
                    </Typography>
                  )}
                </Box>
              ))}
            </Box>
          </Box>
        </>
      )}
    </Box>
  );
};

export default BuildCard;

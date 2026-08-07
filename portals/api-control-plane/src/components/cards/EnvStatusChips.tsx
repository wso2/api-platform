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

import { Box, Typography } from '@wso2/oxygen-ui';

import { useDeployments, useEnvironments } from '../../api/hooks/useMvpQueries';

/**
 * Per-environment deployment dots from the APIs page design: one "Dev"/"Prod"
 * pill per environment with a dot showing whether the API is deployed there.
 * Renders nothing until both queries resolve (no layout jump on cards).
 */
export function EnvStatusChips({ componentId }: { componentId: string }) {
  const environmentsQuery = useEnvironments();
  const deploymentsQuery = useDeployments(componentId);

  const environments = environmentsQuery.data;
  const deployments = deploymentsQuery.data;
  if (!environments?.length || !deployments) return null;

  return (
    <>
      {environments.map((environment) => {
        const deployment = deployments.find(
          (item) => item.environmentId === environment.id
        );
        const status = deployment?.status ?? 'NOT_DEPLOYED';
        const dotColor =
          status === 'READY'
            ? 'success.main'
            : status === 'IN_PROGRESS'
              ? 'warning.main'
              : status === 'FAILED'
                ? 'error.main'
                : 'action.disabled';
        return (
          <Box
            component="span"
            key={environment.id}
            sx={{
              alignItems: 'center',
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: '20px',
              display: 'inline-flex',
              flexShrink: 0,
              gap: 0.75,
              px: 1.25,
              py: 0.25,
            }}
          >
            <Box
              component="span"
              sx={{
                bgcolor: dotColor,
                borderRadius: '50%',
                height: 6,
                width: 6,
              }}
            />
            <Typography color="text.secondary" variant="caption">
              {environment.type === 'PRODUCTION' ? 'Prod' : 'Dev'}
            </Typography>
          </Box>
        );
      })}
    </>
  );
}

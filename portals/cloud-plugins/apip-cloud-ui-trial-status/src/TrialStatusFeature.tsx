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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */


import { Box, Stack, Typography } from '@wso2/oxygen-ui';
import { useEffect, useState } from 'react';

import { getBillingOrganization } from './trialApi';
import type { TrialDetails } from './types';

const TOTAL_TRIAL_DAYS = 14;

export default function TrialStatusFeature() {
  const [trial, setTrial] = useState<TrialDetails | null>(null);

  useEffect(() => {
    const controller = new AbortController();

    void getBillingOrganization(controller.signal)
      .then((organization) => {
        const subscription = organization.subscription;
        if (subscription?.status === 'trial' && subscription.trial) {
          setTrial(subscription.trial);
        } else {
          setTrial(null);
        }
      })
      .catch(() => {
        // Billing status must never prevent the rest of the header from loading.
      });

    return () => controller.abort();
  }, []);

  if (!trial) return null;

  const daysRemaining = Math.max(0, Math.ceil(trial.days_remaining));
  const trialProgress = Math.min(100, (daysRemaining / TOTAL_TRIAL_DAYS) * 100);
  const trialEnd = new Date(trial.trial_end);
  const trialEndLabel = Number.isNaN(trialEnd.getTime())
    ? undefined
    : `Trial ends ${trialEnd.toLocaleDateString()}`;

  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={1.25}
      aria-label={`${daysRemaining} days left of free trial`}
      title={trialEndLabel}
      sx={{
        border: '1.5px solid',
        borderColor: '#ff8a3d',
        borderRadius: 1,
        py: 0.4,
        px: 1,
        minWidth: 160,
        flexShrink: 0,
      }}
    >
      <Box
        sx={{
          width: 36,
          height: 36,
          borderRadius: '50%',
          p: '4px',
          background: `conic-gradient(#ff9b5c ${trialProgress}%, rgba(255, 155, 92, 0.22) 0)`,
          display: 'grid',
          placeItems: 'center',
          boxSizing: 'border-box',
          flexShrink: 0,
        }}
      >
        <Box
          sx={{
            width: '100%',
            height: '100%',
            borderRadius: '50%',
            bgcolor: 'background.paper',
            display: 'grid',
            placeItems: 'center',
          }}
        >
          <Typography variant="body2" fontWeight={700} lineHeight={1}>
            {daysRemaining}
          </Typography>
        </Box>
      </Box>
      <Box>
        <Typography variant="body2" fontWeight={700} lineHeight={1.2}>
          days left
        </Typography>
        <Typography variant="body2" color="text.secondary" lineHeight={1.2}>
          of free trial
        </Typography>
      </Box>
    </Stack>
  );
}

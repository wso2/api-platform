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

import { useEffect, useState } from 'react';

import { listSubscriptionPlans } from '../apis/subscriptionPlanApis';
import { PLATFORM_API_BASE_URL } from '../paths';
import { logger } from '../utils/logger';
import type { SubscriptionPlan } from '../utils/types';

/**
 * Every subscription plan of the organization, read once `enabled` is set. While it is read again the
 * older list stays, but `upToDate` is false: it may predate a plan activated since.
 */
export function useSubscriptionPlans(enabled: boolean): {
  plans?: SubscriptionPlan[];
  failed: boolean;
  upToDate: boolean;
} {
  const [plans, setPlans] = useState<SubscriptionPlan[]>();
  const [failed, setFailed] = useState(false);
  const [upToDate, setUpToDate] = useState(false);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    void (async () => {
      try {
        const list = await listSubscriptionPlans(PLATFORM_API_BASE_URL);
        if (cancelled) return;
        setPlans(list);
        setFailed(false);
        setUpToDate(true);
      } catch (error) {
        logger.error('Failed to load the subscription plans:', error);
        if (!cancelled) setFailed(true);
      }
    })();
    return () => {
      cancelled = true;
      setUpToDate(false);
    };
  }, [enabled]);

  return { plans, failed, upToDate };
}

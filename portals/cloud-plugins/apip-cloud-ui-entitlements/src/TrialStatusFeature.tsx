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

import { Box, Divider, TrialBadge } from '@wso2/oxygen-ui';
import { useEffect, useState } from 'react';

import { getEntitlementSummary } from './entitlementStore';
import type { EntitlementSummary } from './entitlementTypes';
import type { TrialStatusHostPort } from './hostPort';
import { subscriptionsUrl } from './subscriptionsUrl';
import { trialBadgeState } from './trialState';
import type { BillingOrganization } from './types';
import { toUsagePanel } from './usagePanel';

/**
 * Share of the budget at which the usage bars turn amber, then red.
 *
 * The component's own defaults are 0.5 and 0.75; these match the thresholds the
 * console's design calls for — a warning once most of the budget is gone, red
 * only once there is genuinely none left.
 */
const WARNING_THRESHOLD = 0.6;
const CRITICAL_THRESHOLD = 1;

export type TrialStatusFeatureProps = {
  port: TrialStatusHostPort;
  /** Cloud Console's origin, for the subscriptions link; this page's own origin when unset. */
  consoleUrl?: string;
};

export default function TrialStatusFeature({ port, consoleUrl }: TrialStatusFeatureProps) {
  const [billing, setBilling] = useState<BillingOrganization | null>(null);
  const [summary, setSummary] = useState<EntitlementSummary | null>(null);
  const organization = port.orgHandle;
  // Bumped by the host whenever counts may have moved — a create attempt, a
  // window refocus after work in another portal, the periodic backstop.
  const refreshSignal = port.resourceLimits?.refreshSignal ?? 0;

  useEffect(() => {
    let cancelled = false;

    // The host owns this read and memoises it, so asking here costs no extra
    // request even though the host's own activation hook asks for it too.
    void port.billing
      .organization()
      .then((organizationRecord) => {
        if (!cancelled) setBilling(organizationRecord ?? null);
      })
      .catch(() => {
        // Billing status must never prevent the rest of the header loading.
      });

    return () => {
      cancelled = true;
    };
  }, [port]);

  useEffect(() => {
    if (!organization) return undefined;
    let cancelled = false;

    // Shared with the entitlements gate through the module-level store, so the
    // badge and the gate cost one request between them rather than two. The
    // signal is part of the store's key, so a bump re-reads here without
    // depending on which of the two components' effects runs first.
    void getEntitlementSummary(organization, refreshSignal).then((next) => {
      if (!cancelled) setSummary(next);
    });

    return () => {
      cancelled = true;
    };
  }, [organization, refreshSignal]);

  const state = trialBadgeState(billing);
  if (!state) return null;
  const running = state.kind === 'running';

  // `null` when the organization's interfaces budget is unlimited, or nothing
  // could be read: the badge is then a plain countdown with no panel.
  const usagePanel = toUsagePanel(summary);
  const href = subscriptionsUrl(consoleUrl);
  const trialEndLabel = running
    ? `Trial ends ${new Date(state.trialEnd).toLocaleDateString()}`
    : undefined;

  // `totalDays` is passed explicitly on every render: TrialBadge's own default
  // is 14, the very value whose mismatch with this 30-day product caused the
  // original bug, so omitting it would reintroduce it inside the component.
  return (
    <>
      {/* The badge renders its usage panel with `disablePortal`, so the panel is a
          DOM child of this wrapper rather than of the body — which is why its
          surface has to be corrected from out here.

          The panel sets its own surface to `background.paper` and then explicitly
          turns `backdrop-filter` off, on the reasonable assumption that paper is
          opaque. In this theme it is not: `background.paper` is `#000000c5` /
          `#ffffffc5`, deliberately translucent, and it is the theme-wide blur that
          normally keeps what sits behind it from reading through. With that blur
          switched off the page shows straight through the panel. Repaint it on the
          one background token that is fully opaque (`background.default`), which
          the arrow then inherits, so the two still match. */}
      <Box
        sx={{
          display: 'inline-flex',
          flexShrink: 0,
          '& .MuiCard-root': { backgroundColor: 'background.default' },
        }}
      >
        <TrialBadge
          aria-label={
            running ? `${Math.ceil(state.daysLeft)} days left of free trial` : 'Free trial expired'
          }
          daysLeft={running ? state.daysLeft : 0}
          sx={{ flexShrink: 0 }}
          // The native tooltip this renders floats above every stacking context,
          // including the usage panel it would sit on top of, and the panel is
          // opened by the same hover. Keep the date only when there is no panel
          // to collide with; the panel is the richer surface of the two.
          title={usagePanel ? undefined : trialEndLabel}
          totalDays={state.totalDays}
          upgradeHref={href}
          usage={
            usagePanel
              ? {
                  sections: usagePanel.sections,
                  criticalThreshold: CRITICAL_THRESHOLD,
                  warningThreshold: WARNING_THRESHOLD,
                  upgradeHref: href,
                  // The component's default footer advertises a plan by name
                  // ("Unlimited on Pro"), which is not this product's plan. Drop it
                  // rather than ship another product's packaging.
                  footerText: null,
                }
              : undefined
          }
        />
      </Box>
      {/* Header.Actions' 4px gap only reads as space between borderless icon
          buttons; the bordered badge gets its own, as in Cloud Console. */}
      <Divider orientation="vertical" flexItem sx={{ mx: 1, my: 1.5 }} />
    </>
  );
}

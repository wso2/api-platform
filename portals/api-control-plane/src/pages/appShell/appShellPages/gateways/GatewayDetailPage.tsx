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

import { useState } from 'react';
import { Box, PageTitle, Stack, Tab, Tabs } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link, useParams } from 'react-router-dom';

import { useGateway } from '@/api/resources/gateways';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { runtimeConfig } from '@/config/runtime';
import { routes } from '@/routes/paths';
import { GatewayDetailHeader } from './components/GatewayDetailHeader';
import { GatewayGetStartedPanel } from './components/GatewayGetStartedPanel';
import { GatewayPoliciesPanel } from './components/GatewayPoliciesPanel';
import { GatewaySetupBanner } from './components/GatewaySetupBanner';
import { isSetupBannerDismissed, dismissSetupBanner } from './gatewaySetupBannerStorage';

const messages = defineMessages({
  back: {
    id: 'gateways.detail.action.back',
    defaultMessage: 'Back to list',
    description: 'Returns to the gateway listing from a gateway’s own page.',
  },
  errorMessage: {
    id: 'gateways.detail.error.message',
    defaultMessage: 'Unable to load this gateway.',
  },
  loading: {
    id: 'gateways.detail.loading',
    defaultMessage: 'Loading gateway',
  },
  notFound: {
    id: 'gateways.detail.notFound',
    defaultMessage: 'Gateway not found',
  },
  tabConfigurations: {
    id: 'gateways.detail.tab.configurations',
    defaultMessage: 'Configurations',
    description: 'Tab holding the setup walkthrough that brings a gateway online.',
  },
  tabPolicies: {
    id: 'gateways.detail.tab.policies',
    defaultMessage: 'Policies',
    description: 'Tab listing the mediation policies installed on a gateway.',
  },
});

/** Which section of the gateway the page is showing. */
type GatewayTab = 'configurations' | 'policies';

/**
 * One gateway: what it is, and how to bring it online.
 *
 * A user arriving here has just provisioned a gateway and
 * has one job: run the commands.
 *
 * Data comes from the polled detail query, so `isActive` flipping when the
 * gateway finally connects updates the banner and the header on its own.
 */
export function GatewayDetailPage() {
  const { orgHandle = '', gatewayId = '' } = useParams();
  const intl = useIntl();
  const gatewayQuery = useGateway(gatewayId, { poll: true });

  // Seeded from storage so a banner closed on a previous visit does not
  // reappear, then held in state so closing it this time takes effect at once.
  const [bannerDismissed, setBannerDismissed] = useState(() => isSetupBannerDismissed(gatewayId));
  const [tab, setTab] = useState<GatewayTab>('configurations');

  const dismissBanner = () => {
    dismissSetupBanner(gatewayId);
    setBannerDismissed(true);
  };

  // `isPending` is used instead of `isLoading` because the query is disabled
  // until the route's organization resolves; otherwise an empty state would
  // look like a missing gateway.
  if (gatewayQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (gatewayQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.errorMessage)} />;
  }
  if (!gatewayQuery.data) {
    return <ErrorState title={intl.formatMessage(messages.notFound)} />;
  }

  const gateway = gatewayQuery.data;

  return (
    <>
      <PageTitle>
        <Link to={routes.gateways(orgHandle)}>
          <PageTitle.BackButton>
            <FormattedMessage {...messages.back} />
          </PageTitle.BackButton>
        </Link>
      </PageTitle>

      <Stack spacing={3}>
        {!bannerDismissed && (
          <GatewaySetupBanner
            displayName={gateway.displayName || gatewayId}
            isConnected={Boolean(gateway.isActive)}
            onDismiss={dismissBanner}
          />
        )}

        <GatewayDetailHeader gateway={gateway} gatewayId={gatewayId} />

        <Box>
          <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
            <Tabs onChange={(_event, next: GatewayTab) => setTab(next)} value={tab}>
              <Tab label={intl.formatMessage(messages.tabConfigurations)} value="configurations" />
              <Tab label={intl.formatMessage(messages.tabPolicies)} value="policies" />
            </Tabs>
          </Box>

          <Box sx={{ pt: 3 }}>
            {tab === 'configurations' ? (
              <GatewayGetStartedPanel
                controlPlaneHost={runtimeConfig.gatewayControlPlaneHost}
                gateway={gateway}
                gatewayId={gatewayId}
              />
            ) : (
              <GatewayPoliciesPanel gatewayId={gatewayId} />
            )}
          </Box>
        </Box>
      </Stack>
    </>
  );
}

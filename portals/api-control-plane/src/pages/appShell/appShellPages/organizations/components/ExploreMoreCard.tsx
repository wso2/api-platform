/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { Box, Card, CardContent, Grid, Stack, Typography, useTheme } from '@wso2/oxygen-ui';
import { ArrowRight, BookOpen, Server, ShieldCheck } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

const DOCS_BASE = 'https://wso2.com/api-platform/docs';

const messages = defineMessages({
  exploreMore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.exploreMore',
    defaultMessage: 'Explore More',
    description: 'Heading above a card of links into the product documentation.',
  },
  designAttachPolicies: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.designAttachPolicies',
    defaultMessage: 'Attach and Manage Policies',
  },
  designCustomize: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.designCustomize',
    defaultMessage: 'Design and Customize API Proxies',
  },
  designSelfHostedGateway: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.designSelfHostedGateway',
    defaultMessage: 'Getting Started with a Self-Hosted Gateway',
  },
  designTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.designTitle',
    defaultMessage: 'Design and Deploy',
    description: 'Section heading. Refers to building an API proxy and putting it on a gateway.',
  },
  getStartedCreateProxy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.getStartedCreateProxy',
    defaultMessage: 'Create an API Proxy',
  },
  getStartedGuide: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.getStartedGuide',
    defaultMessage: 'Get Started with the WSO2 API Platform',
  },
  getStartedRestFromScratch: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.getStartedRestFromScratch',
    defaultMessage: 'Create a REST API Proxy from Scratch',
  },
  getStartedTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.getStartedTitle',
    defaultMessage: 'Get Started',
    description: 'Section heading for introductory documentation.',
  },
  governGovernance: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.governGovernance',
    defaultMessage: 'API Governance Overview',
  },
  governInsights: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.governInsights',
    defaultMessage: 'Insights and Analytics',
  },
  governTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.governTitle',
    defaultMessage: 'Govern and Monitor',
    description: 'Section heading. "Govern" means applying organization-wide rules to APIs.',
  },
});

export default function ExploreMoreCard() {
  const intl = useIntl();
  const theme = useTheme();

  const sections = [
    {
      key: 'getStarted',
      title: intl.formatMessage(messages.getStartedTitle),
      icon: BookOpen,
      links: [
        {
          label: intl.formatMessage(messages.getStartedGuide),
          href: `${DOCS_BASE}/get-started/`,
        },
        {
          label: intl.formatMessage(messages.getStartedCreateProxy),
          href: `${DOCS_BASE}/cloud/create-api-proxy/overview/`,
        },
        {
          label: intl.formatMessage(messages.getStartedRestFromScratch),
          href: `${DOCS_BASE}/cloud/create-api-proxy/my-apis/http/start-from-scratch/`,
        },
      ],
    },
    {
      key: 'design',
      title: intl.formatMessage(messages.designTitle),
      icon: Server,
      links: [
        {
          label: intl.formatMessage(messages.designCustomize),
          href: `${DOCS_BASE}/cloud/develop-api-proxy/policy/overview/`,
        },
        {
          label: intl.formatMessage(messages.designAttachPolicies),
          href: `${DOCS_BASE}/cloud/develop-api-proxy/policy/attach-and-manage-policies/`,
        },
        {
          label: intl.formatMessage(messages.designSelfHostedGateway),
          href: `${DOCS_BASE}/cloud/api-platform-gateway/getting-started/`,
        },
      ],
    },
    {
      key: 'govern',
      title: intl.formatMessage(messages.governTitle),
      icon: ShieldCheck,
      links: [
        {
          label: intl.formatMessage(messages.governGovernance),
          href: `${DOCS_BASE}/cloud/governance/overview/`,
        },
        {
          label: intl.formatMessage(messages.governInsights),
          href: `${DOCS_BASE}/cloud/monitoring-and-insights/insights/`,
        },
      ],
    },
  ];

  return (
    <Box>
      <Typography variant="h6" sx={{ mb: 1.5 }}>
        <FormattedMessage {...messages.exploreMore} />
      </Typography>
      <Card sx={{ width: '100%' }}>
        <CardContent>
          <Grid container spacing={3} sx={{ m: 0, width: '100%' }}>
            {sections.map((section) => {
              const Icon = section.icon;
              return (
                <Grid key={section.key} size={{ md: 4, xs: 12 }}>
                  <Stack spacing={1.2}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Icon size={24} color={theme.palette.primary.main} />
                      <Box>
                        <Typography variant="body1" sx={{ fontWeight: 600 }}>
                          {section.title}
                        </Typography>
                      </Box>
                    </Stack>
                    <Stack spacing={0.5}>
                      {section.links.map((link) => (
                        <Box
                          key={link.href}
                          component="a"
                          href={link.href}
                          target="_blank"
                          rel="noopener noreferrer"
                          sx={{
                            alignItems: 'center',
                            color: 'text.secondary',
                            display: 'flex',
                            fontSize: '0.82rem',
                            gap: 0.5,
                            textDecoration: 'none',
                            '&:hover': { color: 'primary.main' },
                          }}
                        >
                          <ArrowRight size={14} />
                          <span>{link.label}</span>
                        </Box>
                      ))}
                    </Stack>
                  </Stack>
                </Grid>
              );
            })}
          </Grid>
        </CardContent>
      </Card>
    </Box>
  );
}

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
import { ArrowRight, BookOpen, Network, Server } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { runtimeConfig } from '../../../../../config/runtime';

const messages = defineMessages({
  exploreMore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.exploreMore',
    defaultMessage: 'Explore More',
    description: 'Heading above a card of links into the product documentation.',
  },
  getStartedTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.getStartedTitle',
    defaultMessage: 'Get Started',
    description: 'Section heading for introductory documentation.',
  },
  getStartedGuide: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.getStartedGuide',
    defaultMessage: 'Get Started with the Choreo API Management',
  },
  getStartedCreateRest: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.getStartedCreateRest',
    defaultMessage: 'Create REST APIs',
  },
  getStartedCreateGraphql: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.getStartedCreateGraphql',
    defaultMessage: 'Create GraphQL APIs',
  },
  designTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.designTitle',
    defaultMessage: 'Design and Deploy',
    description: 'Section heading. Refers to building an API proxy and putting it on a gateway.',
  },
  designCustomize: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.designCustomize',
    defaultMessage: 'Design and Customize APIs',
  },
  designAttachPolicies: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.designAttachPolicies',
    defaultMessage: 'Attach and Manage Policies',
  },
  designDeployTest: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.designDeployTest',
    defaultMessage: 'Deploy and Test an API',
  },
  designPublish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.designPublish',
    defaultMessage: 'Publish an API',
  },
  gatewaysTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.gatewaysTitle',
    defaultMessage: 'Gateways and Portals',
    description: 'Section heading. Refers to running gateways, including self-hosted ones, and managing API portals.',
  },
  gatewaysSelfHosted: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.gatewaysSelfHosted',
    defaultMessage: 'Self-Hosted Gateways',
  },
  gatewaysManage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.gatewaysManage',
    defaultMessage: 'Manage Gateways',
  },
  gatewaysPortals: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.ExploreMoreCard.gatewaysPortals',
    defaultMessage: 'Manage API Portals',
  },
});

export default function ExploreMoreCard() {
  const intl = useIntl();
  const theme = useTheme();
  // Same docs root as the footer's Documentation link, so one setting moves both.
  const docsBase = runtimeConfig.documentationLink.replace(/\/+$/, '');

  const sections = [
    {
      key: 'getStarted',
      title: intl.formatMessage(messages.getStartedTitle),
      icon: BookOpen,
      links: [
        { label: intl.formatMessage(messages.getStartedGuide), href: `${docsBase}/get-started` },
        {
          label: intl.formatMessage(messages.getStartedCreateRest),
          href: `${docsBase}/api-control-plane/create-api-proxy/rest-apis/start-from-scratch`,
        },
        {
          label: intl.formatMessage(messages.getStartedCreateGraphql),
          href: `${docsBase}/api-control-plane/create-api-proxy/graphql-apis/start-from-scratch`,
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
          href: `${docsBase}/api-control-plane/develop-api-proxy/definition`,
        },
        {
          label: intl.formatMessage(messages.designAttachPolicies),
          href: `${docsBase}/api-control-plane/develop-api-proxy/policy/attach-and-manage-policies`,
        },
        {
          label: intl.formatMessage(messages.designDeployTest),
          href: `${docsBase}/api-control-plane/deploy-api-proxy/overview`,
        },
        {
          label: intl.formatMessage(messages.designPublish),
          href: `${docsBase}/api-control-plane/publish-api-proxy/publish-an-api`,
        },
      ],
    },
    {
      key: 'gateways',
      title: intl.formatMessage(messages.gatewaysTitle),
      icon: Network,
      links: [
        {
          label: intl.formatMessage(messages.gatewaysSelfHosted),
          href: `${docsBase}/api-gateway/self-hosted-gateways/architecture`,
        },
        {
          label: intl.formatMessage(messages.gatewaysManage),
          href: `${docsBase}/api-control-plane/administer/managing-gateways`,
        },
        {
          label: intl.formatMessage(messages.gatewaysPortals),
          href: `${docsBase}/api-control-plane/administer/managing-portals`,
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

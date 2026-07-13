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

import React from 'react';
import {
  Box,
  Card,
  CardContent,
  Grid,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  BookOpen,
  Network,
  Server,
} from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';

export default function ExploreMoreCard() {
  const sections = [
    {
      title: 'Quick Start ',
      subtitle: 'Start with AI Workspace basics and set up your first AI Gateway.',
      icon: BookOpen,
      links: [
        {
          label: 'AI Workspace Getting Started Guide',
          href: 'https://wso2.com/bijira/docs/ai-workspace/getting-started/',
        },
        {
          label: 'Set Up and Configure an AI Gateway',
          href: 'https://wso2.com/bijira/docs/ai-workspace/ai-gateways/setting-up/',
        },
      ],
    },
    {
      title: 'AI Provider Integration',
      subtitle:
        'Connect, configure, and maintain model providers for your workspace.',
      icon: Server,
      links: [
        {
          label: 'AI Providers Overview',
          href: 'https://wso2.com/bijira/docs/ai-workspace/llm-providers/overview/',
        },
        {
          label: 'Configure a New AI Provider',
          href: 'https://wso2.com/bijira/docs/ai-workspace/llm-providers/configure-provider/',
        },
        {
          label: 'Manage Existing AI Providers',
          href: 'https://wso2.com/bijira/docs/ai-workspace/llm-providers/manage-provider/',
        },
      ],
    },
    {
      title: 'App LLM Proxy Management',
      subtitle: 'Create and operate App LLM Proxies for secure and governed access.',
      icon: Network,
      links: [
        {
          label: 'App LLM Proxies Overview',
          href: 'https://wso2.com/bijira/docs/ai-workspace/llm-proxies/overview/',
        },
        {
          label: 'Create a New App LLM Proxy',
          href: 'https://wso2.com/api-platform/docs/ai-workspace/llm-proxies/configure-proxy/',
        },
        {
          label: 'Manage Existing App LLM Proxies',
          href: 'https://wso2.com/bijira/docs/ai-workspace/llm-proxies/manage-proxy/',
        },
      ],
    },
  ];

  return (
    <Box>
      <Typography variant="h6" sx={{ mb: 1.5 }}>
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.projects.ExploreMoreCard.explore.more"
          defaultMessage={'Explore More'}
        />
      </Typography>
      <Card sx={{ width: '100%' }}>
        <CardContent>
          <Grid container spacing={2}>
            {sections.map((section) => {
              const Icon = section.icon;
              return (
                <Grid key={section.title} size={{ xs: 12, md: 4 }}>
                  <Stack spacing={1.2}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Icon size={24} color="#f36822" />
                      <Box>
                        <Typography variant="body1" sx={{ fontWeight: 600 }}>
                          {section.title}
                        </Typography>
                      </Box>
                    </Stack>
                    <Stack spacing={0.5}>
                      {section.links.map((link) => (
                        <Box
                          key={link.label}
                          component="a"
                          href={link.href}
                          target="_blank"
                          rel="noopener noreferrer"
                          sx={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 0.5,
                            color: 'text.secondary',
                            textDecoration: 'none',
                            fontSize: '0.82rem',
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
